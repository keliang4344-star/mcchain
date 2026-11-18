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
 * 贡献挖矿前台服务。
 * 负责设备贡献计量、EdgeAI 任务执行、奖励追踪。
 */
@AndroidEntryPoint
class MinerService : LifecycleService() {

    @Inject lateinit var rpcClient: RpcClient
    @Inject lateinit var securePrefs: SecurePrefs
    @Inject lateinit var contributionDao: ContributionDao
    @Inject lateinit var phoneNodeDao: PhoneNodeDao
    @Inject lateinit var edgeAiTaskDao: EdgeAiTaskDao
    @Inject lateinit var accountDao: AccountDao

    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var wakeLock: PowerManager.WakeLock? = null
    private var miningJob: Job? = null

    // 本机计量次数（不代表链上收益）
    private var contributionCount: Int = 0

    override fun onCreate() {
        super.onCreate()
        acquireWakeLock()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        super.onStartCommand(intent, flags, startId)
        startForeground(NOTIFICATION_ID, buildNotification())
        startMining()
        return START_STICKY
    }

    override fun onBind(intent: Intent): IBinder? = null

    override fun onDestroy() {
        miningJob?.cancel()
        serviceScope.cancel()
        wakeLock?.release()
        super.onDestroy()
    }

    private fun acquireWakeLock() {
        val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
        wakeLock = powerManager.newWakeLock(
            PowerManager.PARTIAL_WAKE_LOCK,
            "mcchain:mining"
        ).apply {
            setReferenceCounted(false)
            acquire(10 * 60 * 1000L)
        }
    }

    private fun startMining() {
        miningJob?.cancel()
        miningJob = serviceScope.launch {
            while (isActive) {
                try {
                    miningCycle()
                } catch (e: Exception) {
                    Timber.e(e, "Mining error")
                }
                delay(30_000L) // 每 30 秒一轮
            }
        }
    }

    private suspend fun miningCycle() {
        val deviceId = securePrefs.deviceId ?: return
        val node = phoneNodeDao.getByDeviceId(deviceId) ?: return
        if (node.status != NodeStatus.ACTIVE) return

        // 1. 计量带宽贡献
        measureBandwidthContribution(deviceId)

        // 2. 计量存储贡献
        measureStorageContribution(deviceId)

        // 3. 检查并执行 EdgeAI 任务
        checkAndExecuteAiTasks(deviceId)

        // 4. 更新通知
        updateMiningNotification()
    }

    /**
     * 计量带宽贡献：监控网络 I/O 并上报
     */
    private suspend fun measureBandwidthContribution(deviceId: String) {
        // 简化的带宽计量：基于应用实际网络流量
        val bandwidthUp = measureNetworkUpBytes()
        val bandwidthDown = measureNetworkDownBytes()

        val totalBytes = bandwidthUp + bandwidthDown
        if (totalBytes < 1024 * 1024) return // 低于 1MB 不报

        // Phase 0：只记录本机原始计量，不生成「证明」、不计算奖励、不广播交易。
        // 真实链路（设备签名 + MsgSubmitContribution + 链上结算）见 Phase 1–3。
        val contribution = Contribution(
            id = "bw_${System.currentTimeMillis()}_$deviceId",
            deviceId = deviceId,
            taskType = TaskType.BANDWIDTH,
            taskId = "",
            amount = 0,
            proofHash = "",
            reportedAt = System.currentTimeMillis(),
            verifiedAt = null,
            status = ContribStatus.LOCAL_ONLY
        )
        contributionDao.upsert(contribution)
        contributionCount++
    }

    /**
     * 计量存储贡献：检查本地数据库占用
     */
    private suspend fun measureStorageContribution(deviceId: String) {
        val storageBytes = measureLocalStorageBytes()
        if (storageBytes < 100 * 1024 * 1024) return // 低于 100MB 不报

        val contribution = Contribution(
            id = "st_${System.currentTimeMillis()}_$deviceId",
            deviceId = deviceId,
            taskType = TaskType.DATA_STORAGE,
            taskId = "",
            amount = 0,
            proofHash = "",
            reportedAt = System.currentTimeMillis(),
            verifiedAt = null,
            status = ContribStatus.LOCAL_ONLY
        )
        contributionDao.upsert(contribution)
        contributionCount++
    }

    /**
     * 检查并执行 EdgeAI 推理任务
     */
    private suspend fun checkAndExecuteAiTasks(deviceId: String) {
        // Phase 0：真实推理尚未接入（计划 Phase 4）。
        // 禁止再用 SHA-256 冒充推理结果，也不要把任务标记为执行中，避免污染任务状态。
        val available = edgeAiTaskDao.getAvailableTasks(limit = 1)
        if (available.isNotEmpty()) {
            Timber.w("EdgeAI task execution not integrated yet; skipping %d task(s)", available.size)
        }
    }

    // === 工具方法 ===

    private fun measureNetworkUpBytes(): Long {
        // 使用 TrafficStats 获取应用上行流量
        return android.net.TrafficStats.getUidTxBytes(android.os.Process.myUid()) % 1_000_000_000
    }

    private fun measureNetworkDownBytes(): Long {
        return android.net.TrafficStats.getUidRxBytes(android.os.Process.myUid()) % 1_000_000_000
    }

    private fun measureLocalStorageBytes(): Long {
        val dbPath = databaseList().firstOrNull() ?: return 0
        val dbFile = getDatabasePath(dbPath)
        return dbFile?.length() ?: 0
    }

    // === 通知 ===

    private fun buildNotification(): Notification {
        return NotificationCompat.Builder(this, McMinerApp.CHANNEL_MINING)
            .setContentTitle("MCChain 贡献挖矿")
            .setContentText("正在贡献设备资源...")
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
    }

    private fun updateMiningNotification() {
        val notification = NotificationCompat.Builder(this, McMinerApp.CHANNEL_MINING)
            .setContentTitle("贡献计量运行中")
            .setContentText("本次已计量 $contributionCount 次（仅本机，未上链）")
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, notification)
    }

    companion object {
        const val NOTIFICATION_ID = 1002

        fun start(context: Context) {
            val intent = Intent(context, MinerService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        fun stop(context: Context) {
            context.stopService(Intent(context, MinerService::class.java))
        }
    }
}
