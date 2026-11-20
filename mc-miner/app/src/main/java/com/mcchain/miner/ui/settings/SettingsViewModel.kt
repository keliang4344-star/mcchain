package com.mcchain.miner.ui.settings

import android.annotation.SuppressLint
import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.db.McChainDatabase
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.network.IndexerClient
import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.service.MinerService
import com.mcchain.miner.service.NodeService
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import timber.log.Timber
import javax.inject.Inject

/**
 * 设置页状态。
 *
 * 开关全部落库到 [SecurePrefs]，并尽可能立即产生真实副作用：
 *  - 节点同步 / 贡献挖矿：直接 start/stop 对应前台服务；
 *  - Wi-Fi 计量 / 功耗模式：由 MinerService.powerPolicyAllowsMeasurement() 每轮读取；
 *  - 通知 / 开机自启：仅写入偏好，由服务与 BootReceiver 侧读取（E0 接线）。
 */
@SuppressLint("StaticFieldLeak")
@HiltViewModel
class SettingsViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
    val securePrefs: SecurePrefs,
    private val rpcClient: RpcClient,
    private val indexerClient: IndexerClient,
    private val database: McChainDatabase,
) : ViewModel() {

    private val defaultRpc = "http://127.0.0.1:${MCParams.DEFAULT_RPC_PORT}"

    /** 安全存储是否可用；不可用时所有设置都无法落盘，页面需给出提示。 */
    val storageAvailable: Boolean = securePrefs.isSecureStorageAvailable

    private val _rpcEndpoint = MutableStateFlow(defaultRpc)
    val rpcEndpoint: StateFlow<String> = _rpcEndpoint

    /** 只读索引器地址（贡献历史）；空字符串表示未配置。 */
    private val _indexerEndpoint = MutableStateFlow("")
    val indexerEndpoint: StateFlow<String> = _indexerEndpoint

    private val _indexerTestResult = MutableStateFlow<String?>(null)
    val indexerTestResult: StateFlow<String?> = _indexerTestResult

    private val _oracleEndpoint = MutableStateFlow("")
    val oracleEndpoint: StateFlow<String> = _oracleEndpoint

    private val _oracleToken = MutableStateFlow("")
    val oracleToken: StateFlow<String> = _oracleToken

    private val _nodeSyncEnabled = MutableStateFlow(true)
    val nodeSyncEnabled: StateFlow<Boolean> = _nodeSyncEnabled

    private val _miningEnabled = MutableStateFlow(true)
    val miningEnabled: StateFlow<Boolean> = _miningEnabled

    private val _wifiOnly = MutableStateFlow(true)
    val wifiOnly: StateFlow<Boolean> = _wifiOnly

    private val _batteryConfig = MutableStateFlow("medium")
    val batteryConfig: StateFlow<String> = _batteryConfig

    private val _notificationEnabled = MutableStateFlow(true)
    val notificationEnabled: StateFlow<Boolean> = _notificationEnabled

    private val _autoStartEnabled = MutableStateFlow(false)
    val autoStartEnabled: StateFlow<Boolean> = _autoStartEnabled

    /** RPC 连通性测试结果；null 表示未测试。 */
    private val _rpcTestResult = MutableStateFlow<String?>(null)
    val rpcTestResult: StateFlow<String?> = _rpcTestResult

    private val _clearingData = MutableStateFlow(false)
    val clearingData: StateFlow<Boolean> = _clearingData

    /** 清空本地数据的结果；非 null 时页面弹窗提示。 */
    private val _clearResult = MutableStateFlow<ClearResult?>(null)
    val clearResult: StateFlow<ClearResult?> = _clearResult

    init {
        runCatching {
            val savedRpc = securePrefs.rpcEndpoint.ifBlank { defaultRpc }
            _rpcEndpoint.value = savedRpc
            rpcClient.setEndpoint(savedRpc)
            _indexerEndpoint.value = securePrefs.indexerEndpoint
            _oracleEndpoint.value = securePrefs.oracleEndpoint
            _oracleToken.value = securePrefs.oracleToken
            _nodeSyncEnabled.value = securePrefs.nodeSyncEnabled
            _miningEnabled.value = securePrefs.miningEnabled
            _wifiOnly.value = securePrefs.wifiOnly
            _batteryConfig.value = securePrefs.batteryConfig
            _notificationEnabled.value = securePrefs.notificationEnabled
            _autoStartEnabled.value = securePrefs.autoStartEnabled
        }.onFailure {
            Timber.w(it, "Failed to load settings; secure storage unavailable")
        }
    }

    // === 运行开关 ===

    fun toggleNodeSync(enabled: Boolean) {
        _nodeSyncEnabled.value = enabled
        persist { securePrefs.nodeSyncEnabled = enabled }
        if (enabled) NodeService.start(context) else NodeService.stop(context)
    }

    fun toggleMining(enabled: Boolean) {
        _miningEnabled.value = enabled
        persist { securePrefs.miningEnabled = enabled }
        if (enabled) MinerService.start(context) else MinerService.stop(context)
    }

    /**
     * 仅 Wi-Fi 计量。MinerService 每 30 秒一轮读取该偏好：
     * 均衡模式下 wifiOnly=true 时移动数据不计量（心跳不受影响）；
     * 节能模式固定要求 Wi-Fi + 充电，本开关不参与判断。
     */
    fun toggleWifiOnly(enabled: Boolean) {
        _wifiOnly.value = enabled
        persist { securePrefs.wifiOnly = enabled }
    }

    fun setBatteryConfig(config: String) {
        if (config !in VALID_BATTERY_CONFIGS) return
        _batteryConfig.value = config
        persist { securePrefs.batteryConfig = config }
    }

    // === 通知 / 自启动 ===

    /** 常驻通知开关；实际通知展示由 MinerService/NodeService 读取该偏好决定（E0 接线）。 */
    fun toggleNotification(enabled: Boolean) {
        _notificationEnabled.value = enabled
        persist { securePrefs.notificationEnabled = enabled }
    }

    /** 开机自启开关；BootReceiver 读取该偏好决定是否恢复服务（E0 接线）。 */
    fun toggleAutoStart(enabled: Boolean) {
        _autoStartEnabled.value = enabled
        persist { securePrefs.autoStartEnabled = enabled }
    }

    // === 网络配置 ===

    fun updateRpcEndpoint(endpoint: String) {
        val normalized = endpoint.trim().trimEnd('/')
        _rpcEndpoint.value = normalized
        persist { securePrefs.rpcEndpoint = normalized }
        rpcClient.setEndpoint(normalized)
        _rpcTestResult.value = null
    }

    fun resetRpcTestResult() {
        _rpcTestResult.value = null
    }

    /** 索引器地址；留空表示不启用贡献历史。 */
    fun updateIndexerEndpoint(endpoint: String) {
        val normalized = endpoint.trim().trimEnd('/')
        _indexerEndpoint.value = normalized
        persist { securePrefs.indexerEndpoint = normalized }
        _indexerTestResult.value = null
    }

    fun resetIndexerTestResult() {
        _indexerTestResult.value = null
    }

    /** 用当前输入做一次 /health 探测，只验证地址可达，不读取业务数据。 */
    fun testIndexerConnection(endpoint: String? = null) {
        val target = (endpoint ?: _indexerEndpoint.value).trim().trimEnd('/')
        if (target.isBlank()) {
            _indexerTestResult.value = "请填写索引器地址"
            return
        }
        if (!target.startsWith("http://") && !target.startsWith("https://")) {
            _indexerTestResult.value = "失败：地址必须以 http:// 或 https:// 开头"
            return
        }
        _indexerTestResult.value = "连接中…"
        viewModelScope.launch {
            val startedAt = System.currentTimeMillis()
            // 只探测输入框中的地址，不写入偏好；是否保存由用户点击「保存」决定。
            val probe = runCatching { indexerClient.ping(target) }.getOrDefault(false)
            val elapsed = System.currentTimeMillis() - startedAt
            _indexerTestResult.value = if (probe) {
                "连接成功，耗时 $elapsed ms"
            } else {
                "连接失败（$elapsed ms）：请确认索引器已启动，且地址/端口可访问"
            }
        }
    }

    /** 用当前（或对话框中输入的）端点做一次真实连通性探测，并给出耗时。 */
    fun testRpcConnection(endpoint: String? = null) {
        val target = (endpoint ?: _rpcEndpoint.value).trim().trimEnd('/')
        if (target.isBlank()) {
            _rpcTestResult.value = "请填写 RPC 端点"
            return
        }
        val lower = target.lowercase()
        if (!lower.startsWith("http://") && !lower.startsWith("https://")) {
            _rpcTestResult.value = "失败：端点必须以 http:// 或 https:// 开头"
            return
        }
        _rpcTestResult.value = "连接中…"
        viewModelScope.launch {
            val startedAt = System.currentTimeMillis()
            val status = rpcClient.getNodeStatus(target)
            val elapsed = System.currentTimeMillis() - startedAt
            if (status != null && status.currentHeight > 0L) {
                val nodeChainId = status.chainId.ifBlank { "未知" }
                val warning = if (status.chainId.isNotBlank() && status.chainId != MCParams.CHAIN_ID) {
                    "。警告：节点链 ID 与 App 配置的 ${MCParams.CHAIN_ID} 不一致，交易会被节点拒绝"
                } else {
                    ""
                }
                _rpcTestResult.value =
                    "连接成功，链 ID：$nodeChainId，高度 ${status.currentHeight}，耗时 $elapsed ms$warning"
            } else {
                _rpcTestResult.value =
                    "连接失败（$elapsed ms）：未能获取区块高度，请检查地址/端口、节点是否运行、" +
                        "HTTPS 证书是否有效及网络可达性"
            }
        }
    }

    /** 认证预言机地址与 Bearer Token；留空表示未配置（设备认证会 fail-closed）。 */
    fun updateOracle(endpoint: String, token: String) {
        val normalized = endpoint.trim().trimEnd('/')
        val normalizedToken = token.trim()
        _oracleEndpoint.value = normalized
        _oracleToken.value = normalizedToken
        persist {
            securePrefs.oracleEndpoint = normalized
            securePrefs.oracleToken = normalizedToken
        }
    }

    // === 清空本地数据 ===

    /**
     * 停止两个前台服务 → 清空 Room 全部表（IO 线程）→ 清空加密偏好（含助记词）。
     * 完成后 UI 必须提示用户重启 App。
     */
    fun clearLocalData() {
        if (_clearingData.value) return
        _clearingData.value = true
        viewModelScope.launch {
            val errors = mutableListOf<String>()
            withContext(Dispatchers.IO) {
                runCatching { NodeService.stop(context) }
                    .onFailure { errors += "停止节点同步服务失败：${readableReason(it)}" }
                runCatching { MinerService.stop(context) }
                    .onFailure { errors += "停止贡献挖矿服务失败：${readableReason(it)}" }
                runCatching { database.clearAllTables() }
                    .onFailure { errors += "清空本地数据库失败：${readableReason(it)}" }
                runCatching { securePrefs.clearAll() }
                    .onFailure { errors += "清空本机偏好（含助记词）失败：${readableReason(it)}" }
            }

            if (errors.isEmpty()) {
                applyDefaultsAfterClear()
            }
            _clearingData.value = false
            _clearResult.value = if (errors.isEmpty()) {
                ClearResult(
                    success = true,
                    message = "本地钱包、助记词、数据库与全部设置已清空。请立即重启 App，以便重新初始化。"
                )
            } else {
                ClearResult(
                    success = false,
                    message = errors.joinToString("；") + "。已执行的部分不可撤销，请重试或重启 App。"
                )
            }
        }
    }

    fun dismissClearResult() {
        _clearResult.value = null
    }

    private fun applyDefaultsAfterClear() {
        _rpcEndpoint.value = defaultRpc
        rpcClient.setEndpoint(defaultRpc)
        _indexerEndpoint.value = ""
        _indexerTestResult.value = null
        _oracleEndpoint.value = ""
        _oracleToken.value = ""
        _nodeSyncEnabled.value = true
        _miningEnabled.value = true
        _wifiOnly.value = true
        _batteryConfig.value = "medium"
        _notificationEnabled.value = true
        _autoStartEnabled.value = false
        _rpcTestResult.value = null
    }

    // === 内部工具 ===

    /** 偏好写入失败（如 Keystore 不可用）时只记录日志，由页面 banner 提示。 */
    private fun persist(action: () -> Unit) {
        runCatching(action).onFailure {
            Timber.w(it, "Failed to persist setting")
        }
    }

    private fun readableReason(t: Throwable): String = when (t) {
        is IllegalStateException -> "设备安全存储不可用"
        else -> t.message ?: t::class.java.simpleName
    }

    private companion object {
        val VALID_BATTERY_CONFIGS = setOf("low", "medium", "high")
    }
}

/** 清空本地数据的最终结果。 */
data class ClearResult(
    val success: Boolean,
    val message: String,
)
