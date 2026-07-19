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

    // 累计奖励统计
    private var totalRewardToday: Long = 0
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

        val proofHash = generateProof(deviceId, "bandwidth", totalBytes.toString())
        val contribution = Contribution(
            id = "bw_${System.currentTimeMillis()}_$deviceId",
            deviceId = deviceId,
            taskType = TaskType.BANDWIDTH,
            taskId = "",
            amount = calculateBandwidthReward(totalBytes),
            proofHash = proofHash,
            reportedAt = System.currentTimeMillis(),
            verifiedAt = null,
            status = ContribStatus.SUBMITTED
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

        val proofHash = generateProof(deviceId, "storage", storageBytes.toString())
        val contribution = Contribution(
            id = "st_${System.currentTimeMillis()}_$deviceId",
            deviceId = deviceId,
            taskType = TaskType.DATA_STORAGE,
            taskId = "",
            amount = calculateStorageReward(storageBytes),
            proofHash = proofHash,
            reportedAt = System.currentTimeMillis(),
            verifiedAt = null,
            status = ContribStatus.SUBMITTED
        )
        contributionDao.upsert(contribution)
        contributionCount++
    }

    /**
     * 检查并执行 EdgeAI 推理任务
     */
    private suspend fun checkAndExecuteAiTasks(deviceId: String) {
        val availableTasks = edgeAiTaskDao.getAvailableTasks(limit = 5)
        for (task in availableTasks) {
            if (task.status != AiTaskStatus.OPEN) continue

            // 标记为执行中
            edgeAiTaskDao.updateStatus(task.taskId, AiTaskStatus.IN_PROGRESS)

            // 模拟 AI 推理（实际应集成 ONNX Runtime / TFLite）
            val resultHash = simulateAiInference(task.taskId, task.modelId)

            // 提交贡献
            val contribution = Contribution(
                id = "ai_${task.taskId}_$deviceId",
                deviceId = deviceId,
                taskType = TaskType.AI_INFERENCE,
                taskId = task.taskId,
                amount = calculateAiReward(task),
                proofHash = resultHash,
                reportedAt = System.currentTimeMillis(),
                verifiedAt = null,
                status = ContribStatus.SUBMITTED
            )
            contributionDao.upsert(contribution)
            contributionCount++
        }
    }

    // === 奖励计算（基于 MCChain Tokenomics 参数） ===

    private fun calculateBandwidthReward(bytes: Long): Long {
        // 每 GB 带宽贡献 = 基础奖励
        val gbContributed = bytes / (1024.0 * 1024.0 * 1024.0)
        return (gbContributed * 1_000_000).toLong().coerceIn(0, 100_000_000)
    }

    private fun calculateStorageReward(bytes: Long): Long {
        // 每 GB 存储贡献 = 基础奖励
        val gbContributed = bytes / (1024.0 * 1024.0 * 1024.0)
        return (gbContributed * 500_000).toLong().coerceIn(0, 50_000_000)
    }

    private fun calculateAiReward(task: EdgeAiTask): Long {
        // 完成任务获得任务奖励的一定比例
        return task.reward.coerceAtMost(MCParams.EDGEAI_MAX_TASK_REWARD_UMC)
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

    private fun generateProof(deviceId: String, metric: String, value: String): String {
        val input = "$deviceId|$metric|$value|${System.currentTimeMillis()}|${MCParams.CHAIN_ID}"
        return java.security.MessageDigest.getInstance("SHA-256")
            .digest(input.toByteArray())
            .joinToString("") { "%02x".format(it) }
    }

    private fun simulateAiInference(taskId: String, modelHash: String): String {
        val input = "$taskId|$modelHash|${System.currentTimeMillis()}"
        return java.security.MessageDigest.getInstance("SHA-256")
            .digest(input.toByteArray())
            .joinToString("") { "%02x".format(it) }
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
            .setContentTitle("贡献挖矿运行中")
            .setContentText("今日已贡献 $contributionCount 次 | 累计奖励 ${formatUmc(totalRewardToday)} MC")
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, notification)
    }

    private fun formatUmc(umc: Long): String {
        return String.format("%.2f", umc / 1_000_000.0)
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
