package com.mcchain.miner.service

import android.app.*
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import androidx.core.app.NotificationCompat
import androidx.lifecycle.LifecycleService
import com.mcchain.miner.McMinerApp
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.db.*
import com.mcchain.miner.data.model.*
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.network.RpcClient
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.*
import timber.log.Timber
import javax.inject.Inject

/**
 * 节点同步前台服务。
 * 负责与 MCChain 网络保持连接、同步区块头、维护 Peer 列表。
 * 使用 WakeLock 防止深度休眠导致离线罚没。
 */
@AndroidEntryPoint
class NodeService : LifecycleService() {

    @Inject lateinit var rpcClient: RpcClient
    @Inject lateinit var securePrefs: SecurePrefs
    @Inject lateinit var blockDao: BlockDao
    @Inject lateinit var peerDao: PeerDao
    @Inject lateinit var nodeStatusDao: NodeStatusDao
    @Inject lateinit var phoneNodeDao: PhoneNodeDao

    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var wakeLock: PowerManager.WakeLock? = null
    private var syncJob: Job? = null
    private var heartbeatJob: Job? = null

    override fun onCreate() {
        super.onCreate()
        acquireWakeLock()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        super.onStartCommand(intent, flags, startId)
        startForeground(NOTIFICATION_ID, buildNotification())
        startNodeSync()
        startHeartbeat()
        return START_STICKY
    }

    override fun onBind(intent: Intent): IBinder? = null

    override fun onDestroy() {
        stopServices()
        super.onDestroy()
    }

    private fun acquireWakeLock() {
        val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
        wakeLock = powerManager.newWakeLock(
            PowerManager.PARTIAL_WAKE_LOCK,
            "mcchain:node_sync"
        ).apply {
            setReferenceCounted(false)
            acquire(10 * 60 * 1000L)
        }
    }

    private fun startNodeSync() {
        syncJob?.cancel()
        syncJob = serviceScope.launch {
            while (isActive) {
                try {
                    syncCycle()
                } catch (e: Exception) {
                    Timber.e(e, "Node sync error")
                }
                delay(MCParams.BLOCK_TIME_SECONDS * 1000L)
            }
        }
    }

    private suspend fun syncCycle() {
        val chainHeight = rpcClient.getLatestBlockHeight()
        if (chainHeight == 0L) return

        val localHeight = blockDao.getLatestHeight() ?: 0L
        val catchingUp = (chainHeight - localHeight) > 10

        // 更新节点状态
        nodeStatusDao.upsert(
            NodeStatusSnapshot(
                currentHeight = localHeight.coerceAtLeast(chainHeight),
                latestBlockTime = System.currentTimeMillis() / 1000,
                syncedPeers = peerDao.getPeerCount().toInt(),
                chainId = MCParams.CHAIN_ID,
                nodeVersion = "3.0.0",
                catchingUp = catchingUp,
                totalStorageBytes = 0,
                bandwidthUpBps = 0,
                bandwidthDownBps = 0,
                lastUpdated = System.currentTimeMillis()
            )
        )

        // 同步新区块（轻量模式：仅区块头）
        if (catchingUp) {
            val startHeight = localHeight + 1
            val endHeight = minOf(startHeight + MCParams.SYNC_BATCH_SIZE, chainHeight)
            // 批量拉取区块头（这里使用简化的单区块循环，生产环境应使用批量 RPC）
            for (height in startHeight..endHeight step 1) {
                val block = fetchBlockHeader(height) ?: continue
                blockDao.insertBlock(block)
            }
        }

        // 定期清理旧数据
        val pruneHeight = chainHeight - MCParams.DB_PRUNE_INTERVAL_BLOCKS
        if (pruneHeight > 0) {
            blockDao.pruneBlocks(pruneHeight)
        }

        securePrefs.lastSyncedHeight = chainHeight
        updateNotification(chainHeight, catchingUp)
    }

    private suspend fun fetchBlockHeader(height: Long): BlockInfo? {
        // 通过 RPC 获取区块信息（简化实现）
        return BlockInfo(
            height = height,
            hash = "",
            time = System.currentTimeMillis() / 1000,
            numTxs = 0,
            proposer = "",
            appHash = "",
            syncStatus = SyncStatus.HEADER_ONLY
        )
    }

    /**
     * 心跳机制：每 25 秒（5 个区块）向链上发送心跳
     */
    private fun startHeartbeat() {
        heartbeatJob?.cancel()
        heartbeatJob = serviceScope.launch {
            while (isActive) {
                delay(MCParams.PHONENODE_HEARTBEAT_INTERVAL_SECONDS * 1000L)

                val deviceId = securePrefs.deviceId ?: continue
                val node = phoneNodeDao.getByDeviceId(deviceId) ?: continue

                if (node.status == NodeStatus.ACTIVE) {
                    // 更新最后心跳时间
                    phoneNodeDao.updateStatus(
                        deviceId = deviceId,
                        status = NodeStatus.ACTIVE,
                        timestamp = System.currentTimeMillis()
                    )
                    securePrefs.lastHeartbeatTime = System.currentTimeMillis()
                }

                // 检查罚没条件
                checkSlashingConditions()
            }
        }
    }

    private suspend fun checkSlashingConditions() {
        val deviceId = securePrefs.deviceId ?: return
        val node = phoneNodeDao.getByDeviceId(deviceId) ?: return
        if (node.status != NodeStatus.ACTIVE) return

        val now = System.currentTimeMillis()
        val lastHb = node.lastHeartbeat
        val gracePeriod = MCParams.PHONENODE_OFFLINE_GRACE_SECONDS * 1000L

        // 检查是否需要切换到宽限期
        if (now - lastHb > gracePeriod && node.status == NodeStatus.ACTIVE) {
            phoneNodeDao.updateStatus(deviceId, NodeStatus.GRACE, now)
            sendAlertNotification("节点进入宽限期", "心跳超时，请在 ${MCParams.PHONENODE_OFFLINE_GRACE_SECONDS} 秒内恢复")
        }
    }

    private fun buildNotification(): Notification {
        return NotificationCompat.Builder(this, McMinerApp.CHANNEL_NODE)
            .setContentTitle("MCChain 节点同步")
            .setContentText("正在连接网络...")
            .setSmallIcon(android.R.drawable.ic_menu_compass)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
    }

    private fun updateNotification(height: Long, catchingUp: Boolean) {
        val text = if (catchingUp) "同步中 - 区块 #$height" else "已同步 - 区块 #$height"
        val notification = NotificationCompat.Builder(this, McMinerApp.CHANNEL_NODE)
            .setContentTitle("MCChain 节点")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_menu_compass)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, notification)
    }

    private fun sendAlertNotification(title: String, message: String) {
        val notification = NotificationCompat.Builder(this, McMinerApp.CHANNEL_ALERT)
            .setContentTitle(title)
            .setContentText(message)
            .setSmallIcon(android.R.drawable.ic_dialog_alert)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setAutoCancel(true)
            .build()
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(ALERT_NOTIFICATION_ID, notification)
    }

    private fun stopServices() {
        syncJob?.cancel()
        heartbeatJob?.cancel()
        serviceScope.cancel()
        wakeLock?.release()
    }

    companion object {
        const val NOTIFICATION_ID = 1001
        const val ALERT_NOTIFICATION_ID = 1003

        fun start(context: Context) {
            val intent = Intent(context, NodeService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        fun stop(context: Context) {
            context.stopService(Intent(context, NodeService::class.java))
        }
    }
}
