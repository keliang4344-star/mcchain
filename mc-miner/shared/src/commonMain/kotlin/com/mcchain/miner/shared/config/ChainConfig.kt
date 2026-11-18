package com.mcchain.miner.shared.config

/**
 * 链参数。只保留 Tx 编码 / 签名 / 广播需要的最小集合，避免移动端与链仓库参数漂移。
 */
data class ChainConfig(
    val chainId: String,
    val bech32Prefix: String = "mc",
    val bondDenom: String = "umc",
    val displayDenom: String = "MC",
    val decimals: Int = 6,
    val defaultGasLimit: Long = 200_000L,
    /** 默认手续费，单位 umc，字符串形式与链上 Coin.amount 保持一致。 */
    val defaultFeeAmount: String = "5000",
    val blockTimeSeconds: Int = 4,
    val timeoutCommitMillis: Long = 4_000L,
)

object McChainConfigs {
    val mainnet = ChainConfig(chainId = "mcchain-mainnet-1")
    val testnet = ChainConfig(chainId = "mcchain-testnet-1")
    val devnet = ChainConfig(chainId = "mcchain-dev-1")
}
