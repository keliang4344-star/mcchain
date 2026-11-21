package com.mcchain.miner.service

import android.app.*
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.BatteryManager
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
        isRunning = true
        acquireWakeLock()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        super.onStartCommand(intent, flags, startId)
        startForegroundCompat(buildNotification())
        startMining()
        return START_STICKY
    }

    /**
     * 显式声明 specialUse 类型：Android 14+ 必须与 Manifest 声明的类型一致，
     * 且避免系统把本服务按 dataSync 计入 Android 15 的 6 小时/24 小时额度。
     */
    private fun startForegroundCompat(notification: Notification) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    override fun onBind(intent: Intent): IBinder? {
        super.onBind(intent)
        return null
    }

    override fun onDestroy() {
        miningJob?.cancel()
        serviceScope.cancel()
        wakeLock?.release()
        isRunning = false
        super.onDestroy()
    }

    private fun acquireWakeLock() {
        val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
        wakeLock = powerManager.newWakeLock(
            PowerManager.PARTIAL_WAKE_LOCK,
            "mcchain:mining"
        ).apply {
            setReferenceCounted(false)
            // 服务由前台通知维持；每轮挖矿会重新续期，避免无限期持有唤醒锁。
            acquire(WAKELOCK_TIMEOUT_MS)
        }
    }

    private fun startMining() {
        miningJob?.cancel()
        miningJob = serviceScope.launch {
            while (isActive) {
                // 每轮续期唤醒锁（60 分钟上限），息屏时仍能按时心跳。
                wakeLock?.acquire(WAKELOCK_TIMEOUT_MS)
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
        val deviceId = securePrefs.deviceId
        if (deviceId == null) {
            updateMiningNotification("未初始化设备：请先在「节点」页完成设备认证")
            return
        }
        val node = phoneNodeDao.getByDeviceId(deviceId)
        if (node == null) {
            updateMiningNotification("未注册节点：请先在「节点」页完成设备认证")
            return
        }
        if (node.status != NodeStatus.ACTIVE) {
            updateMiningNotification("节点状态为「${statusText(node.status)}」，暂停贡献")
            return
        }

        // 1. 在线心跳（防离线 slash：grace 100 块，这里每 2 分钟一次）
        maybeSendHeartbeat(deviceId, node)

        // 心跳不受功耗策略影响：已认证节点停止心跳会被链上判离线并进入冷却期。
        if (!powerPolicyAllowsMeasurement()) {
            updateMiningNotification("功耗策略限制：当前不计量贡献（心跳保持在线）")
            return
        }

        // 2. 计量带宽贡献
        measureBandwidthContribution(deviceId)

        // 3. 计量存储贡献
        measureStorageContribution(deviceId)

        // 4. 检查并执行 EdgeAI 任务
        checkAndExecuteAiTasks(deviceId)

        // 5. 更新通知
        updateMiningNotification()
    }

    private fun statusText(status: NodeStatus): String = when (status) {
        NodeStatus.UNREGISTERED -> "未注册"
        NodeStatus.PENDING -> "认证中"
        NodeStatus.ACTIVE -> "在线"
        NodeStatus.GRACE -> "宽限期"
        NodeStatus.JAILED -> "已关押"
        NodeStatus.SLASHED -> "已罚没"
    }

    /** 功耗策略：low=仅 Wi-Fi 且充电；medium=仅 Wi-Fi（可关）；high=始终。 */
    private fun powerPolicyAllowsMeasurement(): Boolean {
        val config = runCatching { securePrefs.batteryConfig }.getOrDefault("medium")
        val wifiOnly = runCatching { securePrefs.wifiOnly }.getOrDefault(true)
        return when (config) {
            "low" -> isOnWifi() && isCharging()
            "high" -> true
            else -> !wifiOnly || isOnWifi()
        }
    }

    private fun isOnWifi(): Boolean {
        val cm = getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val caps = cm.getNetworkCapabilities(cm.activeNetwork) ?: return false
        return caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) ||
            caps.hasTransport(NetworkCapabilities.TRANSPORT_ETHERNET)
    }

    private fun isCharging(): Boolean {
        val bm = getSystemService(Context.BATTERY_SERVICE) as BatteryManager
        return bm.isCharging
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
                // 广播成功只代表交易已被接受，链上是否通过验证/结算由链上状态决定，
                // 这里不能直接标成 VERIFIED（否则等于伪造验证结果）。
                contributionDao.updateStatus(contribution.id, ContribStatus.SUBMITTED)
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

    private fun updateMiningNotification(idleReason: String? = null) {
        // 「常驻通知」关闭时不刷新状态通知；前台服务仍需保留系统要求的最小通知。
        if (!securePrefs.notificationEnabled) return
        val title = if (idleReason != null) "贡献已暂停" else "贡献计量运行中"
        val text = when {
            idleReason != null -> idleReason
            submittedCount > 0 -> "本机计量 $contributionCount 次，已上链提交 $submittedCount 次"
            else -> "本次已计量 $contributionCount 次（仅本机，未上链）"
        }
        val notification = NotificationCompat.Builder(this, McMinerApp.CHANNEL_MINING)
            .setContentTitle(title)
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_menu_share)
            .setOngoing(true)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .build()
        val manager = getSystemService(NotificationManager::class.java)
        manager.notify(NOTIFICATION_ID, notification)
    }

    companion object {
        const val NOTIFICATION_ID = 1002

        /** 唤醒锁单次上限；挖矿循环每轮续期。 */
        const val WAKELOCK_TIMEOUT_MS = 60 * 60 * 1000L

        /** 心跳间隔：远小于链上 OfflineGraceBlocks（100 块 ≈ 400s）。 */
        const val HEARTBEAT_INTERVAL_MS = 120_000L

        /** 前台服务是否存活；仅供 UI 显示，重启进程后由 onCreate 重新置位。 */
        @Volatile
        var isRunning: Boolean = false

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
