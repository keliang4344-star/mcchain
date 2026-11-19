package com.mcchain.miner.ui.settings

import androidx.lifecycle.ViewModel
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.network.RpcClient
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import javax.inject.Inject

@HiltViewModel
class SettingsViewModel @Inject constructor(
    val securePrefs: SecurePrefs,
    private val rpcClient: RpcClient,
) : ViewModel() {

    private val _rpcEndpoint = MutableStateFlow("http://127.0.0.1:${MCParams.DEFAULT_RPC_PORT}")
    val rpcEndpoint: StateFlow<String> = _rpcEndpoint

    private val _oracleEndpoint = MutableStateFlow("")
    val oracleEndpoint: StateFlow<String> = _oracleEndpoint

    private val _nodeSyncEnabled = MutableStateFlow(true)
    val nodeSyncEnabled: StateFlow<Boolean> = _nodeSyncEnabled

    private val _miningEnabled = MutableStateFlow(true)
    val miningEnabled: StateFlow<Boolean> = _miningEnabled

    private val _batteryConfig = MutableStateFlow("medium")
    val batteryConfig: StateFlow<String> = _batteryConfig

    init {
        val saved = securePrefs.rpcEndpoint.ifBlank { "http://127.0.0.1:${MCParams.DEFAULT_RPC_PORT}" }
        _rpcEndpoint.value = saved
        rpcClient.setEndpoint(saved)
        _oracleEndpoint.value = securePrefs.oracleEndpoint
        _nodeSyncEnabled.value = securePrefs.nodeSyncEnabled
        _miningEnabled.value = securePrefs.miningEnabled
        _batteryConfig.value = securePrefs.batteryConfig
    }

    fun toggleNodeSync(enabled: Boolean) {
        _nodeSyncEnabled.value = enabled
        securePrefs.nodeSyncEnabled = enabled
    }

    fun toggleMining(enabled: Boolean) {
        _miningEnabled.value = enabled
        securePrefs.miningEnabled = enabled
    }

    fun setBatteryConfig(config: String) {
        _batteryConfig.value = config
        securePrefs.batteryConfig = config
    }

    fun updateRpcEndpoint(endpoint: String) {
        val normalized = endpoint.trim().trimEnd('/')
        _rpcEndpoint.value = normalized
        securePrefs.rpcEndpoint = normalized
        rpcClient.setEndpoint(normalized)
    }

    /** 认证预言机地址与 Bearer Token；留空表示未配置（设备认证会 fail-closed）。 */
    fun updateOracle(endpoint: String, token: String) {
        val normalized = endpoint.trim().trimEnd('/')
        _oracleEndpoint.value = normalized
        securePrefs.oracleEndpoint = normalized
        securePrefs.oracleToken = token.trim()
    }
}
