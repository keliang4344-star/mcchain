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
import com.mcchain.miner.domain.edgeai.EdgeAiService
import com.mcchain.miner.domain.node.ContributionMeter
import com.mcchain.miner.domain.node.DeviceAttestation
import com.mcchain.miner.domain.wallet.TransactionService
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
    @Inject lateinit var deviceAttestation: DeviceAttestation
    @Inject lateinit var transactionService: TransactionService
    @Inject lateinit var contributionMeter: ContributionMeter
    @Inject lateinit var edgeAiService: EdgeAiService

    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var wakeLock: PowerManager.WakeLock? = null
    private var miningJob: Job? = null

    // 本机计量次数（不代表链上收益）
    private var contributionCount: Int = 0

    // 链上提交次数（成功广播并被接受）
    private var submittedCount: Int = 0

    // TrafficStats 是开机以来的累计值；delta 需要基线（进程启动时初始化）。
    private var trafficBaselineBytes: Long = -1L

    // 最近一次成功心跳时间（链上 grace 100 块 ≈ 400s，2 分钟一次留足余量）。
    private var lastHeartbeatAt: Long = 0L

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

        // 1. 在线心跳（防离线 slash：grace 100 块，这里每 2 分钟一次）
        maybeSendHeartbeat(deviceId, node)

        // 2. 计量带宽贡献
        measureBandwidthContribution(deviceId)

        // 3. 计量存储贡献
        measureStorageContribution(deviceId)

        // 4. 检查并执行 EdgeAI 任务
        checkAndExecuteAiTasks(deviceId)

        // 5. 更新通知
        updateMiningNotification()
    }

    /**
     * 心跳闸门：注册为 ACTIVE 的节点必须持续心跳，
     * 否则链上离线 slash 会吊销 attestation 并进入冷却期（无法再认证）。
     */
    private suspend fun maybeSendHeartbeat(deviceId: String, node: PhoneNodeRecord) {
        if (node.status != NodeStatus.ACTIVE) return
        val now = System.currentTimeMillis()
        if (now - lastHeartbeatAt < HEARTBEAT_INTERVAL_MS) return
        val account = accountDao.getDefaultAccount() ?: return
        // 余额只够付手续费时才心跳，避免产生排队失败的无效广播。
        if (account.balance <= 10_000L) return

        val result = transactionService.sendHeartbeat(deviceId)
        when {
            result.success -> {
                lastHeartbeatAt = now
                securePrefs.lastHeartbeatTime = now
                phoneNodeDao.updateStatus(deviceId, NodeStatus.ACTIVE, now)
            }
            result.message.contains("认证", ignoreCase = false) ||
                result.message.contains("not attested", ignoreCase = true) -> {
                // attestation 已被吊销：标记罚没，停止心跳，等待用户重新认证。
                phoneNodeDao.markSlashed(deviceId, NodeStatus.JAILED)
                Timber.w("Heartbeat found node attestation revoked; marked JAILED: %s", result.message)
            }
            else -> Timber.w("Heartbeat failed: %s", result.message)
        }
    }

    /**
     * 计量带宽贡献：按进程基线以来的增量映射 score，达到阈值且已认证时提交上链。
     */
    private suspend fun measureBandwidthContribution(deviceId: String) {
        val totalBytes = measureNetworkUpBytes() + measureNetworkDownBytes()
        if (totalBytes <= 0) return // TrafficStats 不可用（返回 -1）
        if (trafficBaselineBytes < 0) {
            trafficBaselineBytes = totalBytes
            return
        }
        val deltaBytes = (totalBytes - trafficBaselineBytes).coerceAtLeast(0)
        if (deltaBytes < 1024 * 1024) return // 增量低于 1MB 不报

        val score = contributionMeter.bandwidthScore(deltaBytes)
        val taskId = contributionMeter.taskId(deviceId, contributionMeter.currentEpoch(), "bandwidth")
        val contribution = Contribution(
            id = taskId,
            deviceId = deviceId,
            taskType = TaskType.BANDWIDTH,
            taskId = taskId,
            amount = 0,
            proofHash = "",
            reportedAt = System.currentTimeMillis(),
            verifiedAt = null,
            status = ContribStatus.LOCAL_ONLY
        )
        contributionDao.upsert(contribution)
        contributionCount++

        if (score >= ContributionMeter.MIN_SUBMIT_SCORE) {
            submitContribution(contribution, "bandwidth", score)
        }
    }

    /**
     * 计量存储贡献：检查本地数据库占用。
     * 链上 depin 暂无存储类任务类型（仅 inference/data_label/bandwidth），
     * 因此保持 LOCAL_ONLY，不做注定被 ErrUnsupportedType 拒绝的提交。
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
     * 提交一条贡献上链（depin.MsgSubmitContribution）。
     *
     * 先认证后提交：未通过（或已过期）设备认证时只保留本机计量；
     * taskId 幂等：Room 已存在非失败记录时跳过，链上 ErrTaskExists 收敛为已记录。
     */
    private suspend fun submitContribution(contribution: Contribution, chainTaskType: String, score: Int) {
        // 幂等去重：同一 taskId 已提交/已确认过就不再重复花手续费（REJECTED 允许再次尝试）。
        val existing = contributionDao.getById(contribution.id)
        if (existing != null && existing.status != ContribStatus.REJECTED &&
            existing.status != ContribStatus.LOCAL_ONLY
        ) {
            return
        }
        // 余额需覆盖手续费。
        val account = accountDao.getDefaultAccount() ?: return
        if (account.balance <= 5_000L) return

        // 先认证后计量（方案 §[5]）：认证无效时不产生注定被拒的交易。
        if (!deviceAttestation.isAttestationValid()) return

        contributionDao.upsert(contribution.copy(status = ContribStatus.SUBMITTED))
        val result = transactionService.submitContribution(contribution.id, chainTaskType, score)
        when {
            result.success || result.idempotent -> {
                contributionDao.updateVerification(
                    contribution.id, ContribStatus.VERIFIED, 0L, System.currentTimeMillis()
                )
                submittedCount++
            }
            else -> {
                contributionDao.updateVerification(
                    contribution.id, ContribStatus.REJECTED, 0L, System.currentTimeMillis()
                )
                Timber.w("Contribution rejected: %s", result.message)
            }
        }
    }

    /**
     * 检查并执行 EdgeAI 任务：同步链上 open 任务，执行一个白名单任务并提交结果。
     * 结算（争议窗口后拨付 85%）由链上完成，App 只读余额。
     */
    private suspend fun checkAndExecuteAiTasks(deviceId: String) {
        edgeAiService.syncTasks()
        val report = edgeAiService.runDueTask()
        when (report.outcome) {
            EdgeAiService.RunOutcome.SUBMITTED -> {
                Timber.i("EdgeAI task %s submitted: %s", report.taskId, report.message)
                contributionCount++
            }
            EdgeAiService.RunOutcome.FAILED,
            EdgeAiService.RunOutcome.EXEC_TIMEOUT ->
                Timber.w("EdgeAI task %s: %s", report.taskId, report.message)
            else -> Unit // NO_TASK / SKIP_* 静默
        }
    }

    // === 工具方法 ===

    private fun measureNetworkUpBytes(): Long {
        // TrafficStats 返回开机以来累计值；-1 表示不可用。
        val value = android.net.TrafficStats.getUidTxBytes(android.os.Process.myUid())
        return if (value < 0) 0 else value
    }

    private fun measureNetworkDownBytes(): Long {
        val value = android.net.TrafficStats.getUidRxBytes(android.os.Process.myUid())
        return if (value < 0) 0 else value
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
            .setContentText(
                if (submittedCount > 0) {
                    "本机计量 $contributionCount 次，已上链提交 $submittedCount 次"
                } else {
                    "本次已计量 $contributionCount 次（仅本机，未上链）"
                }
            )
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, notification)
    }

    companion object {
        const val NOTIFICATION_ID = 1002

        /** 心跳间隔：远小于链上 OfflineGraceBlocks（100 块 ≈ 400s）。 */
        const val HEARTBEAT_INTERVAL_MS = 120_000L

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
