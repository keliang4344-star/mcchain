package com.mcchain.miner.domain.wallet

import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.shared.util.FeeRules
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 动态手续费：按节点最低 gas price × gas limit 计算，避免链上调整费率后固定 5000umc 被拒。
 *
 * - 手续费在下单前由 [refresh] 拉取；节点不可达时沿用上次值，从未取到则用默认 0.025 umc/gas。
 * - 结果不低于 [MIN_FEE_UMC]，保证极端低费率节点下仍能打包。
 */
@Singleton
class DynamicFeeProvider @Inject constructor(
    private val rpcClient: RpcClient,
) {
    @Volatile
    private var gasPrice: Double? = null

    @Volatile
    private var fetchedAt = 0L

    /** 刷新节点最低 gas price；TTL 内复用，失败静默保留旧值。 */
    suspend fun refresh() {
        val now = System.currentTimeMillis()
        if (gasPrice != null && now - fetchedAt < TTL_MS) return
        val latest = runCatching { rpcClient.getMinGasPrice() }.getOrNull() ?: return
        if (latest >= 0.0) {
            gasPrice = latest
            fetchedAt = now
        }
    }

    /** 指定 gas limit 下应付的最低手续费（umc）。 */
    fun feeFor(gasLimit: Long): Long =
        FeeRules.minFee(gasLimit, gasPrice ?: DEFAULT_GAS_PRICE_UMC, MIN_FEE_UMC)

    companion object {
        /** Cosmos SDK 默认 minimum-gas-prices = 0.025umc。 */
        const val DEFAULT_GAS_PRICE_UMC = 0.025

        /** 下限：即使节点声明 0 gas price，也支付 5000umc，避免零费交易被策略拒绝。 */
        const val MIN_FEE_UMC = 5_000L

        private const val TTL_MS = 60_000L
    }
}
