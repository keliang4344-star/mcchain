package com.mcchain.miner.ui.mining

import android.annotation.SuppressLint
import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.data.db.AccountDao
import com.mcchain.miner.data.db.ContributionDao
import com.mcchain.miner.data.db.EdgeAiTaskDao
import com.mcchain.miner.data.db.PhoneNodeDao
import com.mcchain.miner.data.model.ContribStatus
import com.mcchain.miner.data.model.Contribution
import com.mcchain.miner.data.model.EdgeAiTask
import com.mcchain.miner.data.model.NodeStatus
import com.mcchain.miner.data.model.PhoneNodeRecord
import com.mcchain.miner.data.model.TaskType
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.domain.edgeai.EdgeAiService
import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.service.MinerService
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Date
import java.util.Locale
import javax.inject.Inject

/** 单个贡献类型的今日汇总（次数与上报量）。 */
data class ContributionTypeStat(
    val type: TaskType,
    val count: Int,
    val amount: Long
)

/**
 * 贡献总览。所有数字均来自 Room 中本机计量记录与链上查询结果，
 * 不做任何收益估算或「已验证」推断。
 */
data class ContributionSummary(
    val todayCount: Int = 0,
    val todayAmount: Long = 0,
    val totalCount: Int = 0,
    val onChainCount: Int = 0,
    val pendingCount: Int = 0,
    val typeStats: List<ContributionTypeStat> = emptyList(),
    /** 本机记录数超过单次加载上限，统计只覆盖最近 N 条。 */
    val truncated: Boolean = false
)

/** 服务启动前置条件项。 */
data class PrerequisiteItem(
    val label: String,
    val ok: Boolean,
    val detail: String
)

@SuppressLint("StaticFieldLeak")
@HiltViewModel
class MiningViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
    private val accountDao: AccountDao,
    private val contributionDao: ContributionDao,
    private val edgeAiTaskDao: EdgeAiTaskDao,
    private val phoneNodeDao: PhoneNodeDao,
    private val rpcClient: RpcClient,
    private val securePrefs: SecurePrefs,
    private val edgeAiService: EdgeAiService,
) : ViewModel() {

    private val _contributions = MutableStateFlow<List<Contribution>>(emptyList())
    val contributions: StateFlow<List<Contribution>> = _contributions

    private val _summary = MutableStateFlow(ContributionSummary())
    val summary: StateFlow<ContributionSummary> = _summary

    /** EdgeAI 任务（优先本机 Room 缓存，最近 30 条，含全部状态）。 */
    private val _edgeTasks = MutableStateFlow<List<EdgeAiTask>>(emptyList())
    val edgeTasks: StateFlow<List<EdgeAiTask>> = _edgeTasks

    /** 钱包余额（umc），来自链上查询后持久化的账户记录，不做本地估算。 */
    private val _walletBalance = MutableStateFlow(0L)
    val walletBalance: StateFlow<Long> = _walletBalance

    /** 贡献前台服务是否在运行。 */
    private val _serviceRunning = MutableStateFlow(MinerService.isRunning)
    val serviceRunning: StateFlow<Boolean> = _serviceRunning

    /** 最近一次链上心跳时间（毫秒），0 表示尚未心跳。 */
    private val _lastHeartbeat = MutableStateFlow(0L)
    val lastHeartbeat: StateFlow<Long> = _lastHeartbeat

    /** 设备节点状态（null = 未注册）。 */
    private val _nodeStatus = MutableStateFlow<NodeStatus?>(null)
    val nodeStatus: StateFlow<NodeStatus?> = _nodeStatus

    /** 无法开始贡献的原因；null 表示前置条件满足。 */
    private val _startBlockReason = MutableStateFlow<String?>(null)
    val startBlockReason: StateFlow<String?> = _startBlockReason

    /** 启动前置条件明细，供页面直接展示。 */
    private val _prerequisites = MutableStateFlow<List<PrerequisiteItem>>(emptyList())
    val prerequisites: StateFlow<List<PrerequisiteItem>> = _prerequisites

    /** RPC 检测中标记。 */
    private val _checkingPrerequisites = MutableStateFlow(false)
    val checkingPrerequisites: StateFlow<Boolean> = _checkingPrerequisites

    /** 当前功耗策略说明（来自 SecurePrefs.batteryConfig）。 */
    private val _powerPolicyText = MutableStateFlow("")
    val powerPolicyText: StateFlow<String> = _powerPolicyText

    /** EdgeAI 任务检查中标记。 */
    private val _checkingTasks = MutableStateFlow(false)
    val checkingTasks: StateFlow<Boolean> = _checkingTasks

    /** 最近一次任务检查结果/失败原因。 */
    private val _edgeTaskMessage = MutableStateFlow<String?>(null)
    val edgeTaskMessage: StateFlow<String?> = _edgeTaskMessage

    private val _edgeTaskMessageIsError = MutableStateFlow(false)
    val edgeTaskMessageIsError: StateFlow<Boolean> = _edgeTaskMessageIsError

    /** 最近一次任务检查时间（毫秒），0 表示本次进程内尚未检查。 */
    private val _lastTaskCheckAt = MutableStateFlow(0L)
    val lastTaskCheckAt: StateFlow<Long> = _lastTaskCheckAt

    /** RPC 最近一次探测结果；null 表示尚未检测。 */
    private var rpcReachable: Boolean? = null

    init {
        refresh()
        viewModelScope.launch(Dispatchers.IO) {
            var tick = 0
            while (isActive) {
                _serviceRunning.value = MinerService.isRunning
                _lastHeartbeat.value = pref({ securePrefs.lastHeartbeatTime }, 0L)
                // 每 6 秒刷新一次本地数据，服务在后台写入的计量记录能及时反映。
                if (tick % 3 == 0) refreshLocal()
                tick++
                delay(2_000L)
            }
        }
    }

    fun refresh() {
        viewModelScope.launch(Dispatchers.IO) { refreshLocal() }
    }

    /** 主动做一次真实 RPC 探测，并刷新前置条件。 */
    fun checkPrerequisites() {
        if (_checkingPrerequisites.value) return
        viewModelScope.launch(Dispatchers.IO) {
            _checkingPrerequisites.value = true
            rpcReachable = rpcClient.ping()
            refreshLocal()
            _checkingPrerequisites.value = false
        }
    }

    /**
     * 开始贡献前做前置校验：钱包、设备认证、节点在线、RPC 可达。
     * 缺任何一项都直接给出可操作提示，避免「显示运行中但什么都不做」。
     */
    fun startMining() {
        viewModelScope.launch(Dispatchers.IO) {
            val account = runCatching { accountDao.getDefaultAccount() }.getOrNull()
            if (account == null) {
                _startBlockReason.value = "请先在「钱包」页创建或导入钱包"
                return@launch
            }
            val deviceId = pref({ securePrefs.deviceId }, null)
            if (deviceId.isNullOrBlank()) {
                _startBlockReason.value = "请先在「节点」页完成设备认证"
                return@launch
            }
            val attestToken = pref({ securePrefs.attestationToken }, null)
            val attestExpiry = pref({ securePrefs.attestationExpiry }, 0L)
            if (attestToken.isNullOrBlank() || System.currentTimeMillis() >= attestExpiry) {
                _startBlockReason.value = "设备认证未完成或已过期，请先在「节点」页重新认证"
                return@launch
            }
            val node = runCatching { phoneNodeDao.getByDeviceId(deviceId) }.getOrNull()
            if (node == null || node.status != NodeStatus.ACTIVE) {
                _startBlockReason.value =
                    "设备节点未在线（${statusText(node?.status)}），请先在「节点」页完成认证"
                return@launch
            }
            if (!rpcClient.ping()) {
                rpcReachable = false
                _startBlockReason.value = "无法连接 RPC 节点，请到「设置 → RPC 端点」配置并测试连接"
                refreshLocal()
                return@launch
            }
            rpcReachable = true
            _startBlockReason.value = null
            runCatching {
                securePrefs.miningEnabled = true
                MinerService.start(context)
            }.onSuccess {
                _serviceRunning.value = true
            }.onFailure { e ->
                _startBlockReason.value = "启动贡献服务失败：${readableError(e)}"
            }
            refreshLocal()
        }
    }

    fun stopMining() {
        runCatching {
            securePrefs.miningEnabled = false
            MinerService.stop(context)
        }.onSuccess {
            _serviceRunning.value = false
        }.onFailure { e ->
            _startBlockReason.value = "停止贡献服务失败：${readableError(e)}"
            _serviceRunning.value = MinerService.isRunning
        }
    }

    /**
     * 立即检查 EdgeAI 任务：先确认 RPC 可达，再通过 EdgeAiService.syncTasks()
     * 真实拉取链上 open 任务并写入 Room。
     */
    fun checkEdgeAiTasks() {
        if (_checkingTasks.value) return
        viewModelScope.launch(Dispatchers.IO) {
            _checkingTasks.value = true
            _edgeTaskMessageIsError.value = false
            _edgeTaskMessage.value = "正在连接 RPC 获取链上任务…"
            val reachable = rpcClient.ping()
            rpcReachable = reachable
            if (!reachable) {
                _edgeTaskMessageIsError.value = true
                _edgeTaskMessage.value =
                    "无法连接 RPC 节点：未能获取链上任务，当前展示的是本机缓存的 ${_edgeTasks.value.size} 条记录"
            } else {
                runCatching { edgeAiService.syncTasks() }
                    .onSuccess { count ->
                        _lastTaskCheckAt.value = System.currentTimeMillis()
                        _edgeTaskMessage.value = if (count > 0) {
                            "已从链上同步 $count 个开放任务"
                        } else {
                            "同步完成：链上返回 0 个开放任务（若刚配置节点，请确认 REST API 端口可达）"
                        }
                    }
                    .onFailure { e ->
                        _edgeTaskMessageIsError.value = true
                        _edgeTaskMessage.value = "任务同步失败：${readableError(e)}"
                    }
            }
            _edgeTasks.value = runCatching { edgeAiTaskDao.getRecentTasks(EDGE_TASK_LIMIT) }
                .getOrDefault(emptyList())
            _checkingTasks.value = false
            refreshLocal()
        }
    }

    // === 内部加载 ===

    private suspend fun refreshLocal() {
        val deviceId = pref({ securePrefs.deviceId }, null)
        val account = runCatching { accountDao.getDefaultAccount() }.getOrNull()
        val node = if (deviceId.isNullOrBlank()) {
            null
        } else {
            runCatching { phoneNodeDao.getByDeviceId(deviceId) }.getOrNull()
        }

        val records = if (deviceId.isNullOrBlank()) {
            emptyList()
        } else {
            runCatching { contributionDao.getByDevice(deviceId, CONTRIB_FETCH_LIMIT) }
                .getOrDefault(emptyList())
        }
        val totalCount = if (deviceId.isNullOrBlank()) {
            0
        } else {
            runCatching { contributionDao.getCountByDevice(deviceId) }.getOrDefault(0)
        }

        _contributions.value = records
        _summary.value = buildSummary(records, totalCount)
        _edgeTasks.value = runCatching { edgeAiTaskDao.getRecentTasks(EDGE_TASK_LIMIT) }
            .getOrDefault(emptyList())
        _walletBalance.value = account?.balance ?: 0L
        _nodeStatus.value = node?.status
        _powerPolicyText.value = buildPowerPolicyText()
        _prerequisites.value = buildPrerequisites(account, node)
    }

    private fun buildSummary(
        records: List<Contribution>,
        totalCount: Int
    ): ContributionSummary {
        val dayStart = startOfTodayMillis()
        val today = records.filter { it.reportedAt >= dayStart }
        val typeStats = TaskType.values().map { type ->
            ContributionTypeStat(
                type = type,
                count = today.count { it.taskType == type },
                amount = today.filter { it.taskType == type }.sumOf { it.amount }
            )
        }.filter { it.count > 0 }

        return ContributionSummary(
            todayCount = today.size,
            todayAmount = today.sumOf { it.amount },
            totalCount = totalCount,
            onChainCount = records.count { it.status != ContribStatus.LOCAL_ONLY },
            pendingCount = records.count { it.status == ContribStatus.LOCAL_ONLY },
            typeStats = typeStats,
            truncated = records.size >= CONTRIB_FETCH_LIMIT && totalCount > records.size
        )
    }

    private fun buildPrerequisites(
        account: WalletAccount?,
        node: PhoneNodeRecord?
    ): List<PrerequisiteItem> {
        val walletOk = account != null
        val attestToken = pref({ securePrefs.attestationToken }, null)
        val attestExpiry = pref({ securePrefs.attestationExpiry }, 0L)
        val attestOk = !attestToken.isNullOrBlank() && System.currentTimeMillis() < attestExpiry

        val rpcItem = when (rpcReachable) {
            true -> PrerequisiteItem("RPC 连接", true, "已连通（上次检测）")
            false -> PrerequisiteItem("RPC 连接", false, "无法连接，请到「设置 → RPC 端点」检查")
            null -> PrerequisiteItem("RPC 连接", false, "尚未检测，点击「检测前置条件」")
        }

        return listOf(
            PrerequisiteItem(
                label = "钱包账户",
                ok = walletOk,
                detail = if (walletOk) {
                    "地址 ${account?.address.orEmpty().let { shortAddress(it) }}"
                } else {
                    "未创建钱包，请前往「钱包」页创建或导入"
                }
            ),
            PrerequisiteItem(
                label = "设备认证",
                ok = attestOk,
                detail = if (attestOk) {
                    "认证有效期至 ${formatDateTime(attestExpiry)}"
                } else {
                    "未认证或已过期，请前往「节点」页完成硬件认证"
                }
            ),
            PrerequisiteItem(
                label = "节点状态",
                ok = node?.status == NodeStatus.ACTIVE,
                detail = statusText(node?.status)
            ),
            rpcItem
        )
    }

    private fun buildPowerPolicyText(): String {
        val config = pref({ securePrefs.batteryConfig }, "medium")
        val wifiOnly = pref({ securePrefs.wifiOnly }, true)
        return when (config) {
            "low" -> "省电：仅在 Wi-Fi 且充电时计量贡献（心跳不受影响）"
            "high" -> "高性能：始终计量贡献（耗电与流量较高）"
            else -> if (wifiOnly) {
                "均衡：仅在 Wi-Fi 下计量贡献（心跳不受影响）"
            } else {
                "均衡：允许使用移动网络计量贡献"
            }
        }
    }

    private fun statusText(status: NodeStatus?): String = when (status) {
        null, NodeStatus.UNREGISTERED -> "未注册"
        NodeStatus.PENDING -> "认证中"
        NodeStatus.ACTIVE -> "在线"
        NodeStatus.GRACE -> "宽限期"
        NodeStatus.JAILED -> "已关押"
        NodeStatus.SLASHED -> "已罚没"
    }

    private fun shortAddress(address: String): String =
        if (address.length <= 14) address else "${address.take(8)}…${address.takeLast(4)}"

    private fun formatDateTime(millis: Long): String =
        SimpleDateFormat("MM-dd HH:mm", Locale.getDefault()).format(Date(millis))

    private fun startOfTodayMillis(): Long {
        val calendar = Calendar.getInstance().apply {
            set(Calendar.HOUR_OF_DAY, 0)
            set(Calendar.MINUTE, 0)
            set(Calendar.SECOND, 0)
            set(Calendar.MILLISECOND, 0)
        }
        return calendar.timeInMillis
    }

    private fun readableError(e: Throwable): String =
        e.message?.takeIf { it.isNotBlank() } ?: e.javaClass.simpleName

    /** SecurePrefs 在安全存储不可用时会抛异常；读取失败按缺省值处理，由页面提示用户。 */
    private fun <T> pref(read: () -> T, fallback: T): T =
        runCatching(read).getOrDefault(fallback)

    private companion object {
        /** 单次加载的本机贡献记录上限（用于今日统计与列表展示）。 */
        const val CONTRIB_FETCH_LIMIT = 1_000

        /** EdgeAI 任务列表上限。 */
        const val EDGE_TASK_LIMIT = 30
    }
}
