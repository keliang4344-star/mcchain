package com.mcchain.miner.domain.node

import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.data.pref.SecurePrefs
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import timber.log.Timber
import javax.inject.Inject
import javax.inject.Singleton

data class ChainState(
    val latestBlockHeight: Long = 0,
    val chainId: String = "",
    val syncedPeers: Int = 0,
    val catchingUp: Boolean = true,
    val synced: Boolean = false
)

@Singleton
class ChainStateManager @Inject constructor(
    private val rpcClient: RpcClient,
    private val securePrefs: SecurePrefs
) {
    private val _chainState = MutableStateFlow(ChainState())
    val chainState: StateFlow<ChainState> = _chainState

    private var isRefreshing = false

    suspend fun refreshChainState() {
        if (isRefreshing) return
        isRefreshing = true
        try {
            val status = rpcClient.getNodeStatus()
            if (status != null) {
                val state = ChainState(
                    latestBlockHeight = status.currentHeight,
                    chainId = status.chainId,
                    syncedPeers = status.syncedPeers,
                    catchingUp = status.catchingUp,
                    synced = !status.catchingUp
                )
                _chainState.value = state
                securePrefs.lastSyncedHeight = status.currentHeight
            } else {
                val fallbackHeight = rpcClient.getLatestBlockHeight()
                _chainState.value = ChainState(
                    latestBlockHeight = fallbackHeight,
                    synced = false
                )
                securePrefs.lastSyncedHeight = fallbackHeight
            }
        } catch (e: Exception) {
            Timber.e(e, "Failed to refresh chain state")
        } finally {
            isRefreshing = false
        }
    }
}
