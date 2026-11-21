package com.mcchain.miner.shared.util

import kotlin.math.ceil

/**
 * 手续费计算（纯函数）。
 *
 * 链上最低手续费 = ceil(gasLimit × gasPrice)，且不低于业务下限；
 * gasPrice 非法/为负时按下限处理，避免 NaN/负数穿透到交易。
 */
object FeeRules {

    fun minFee(gasLimit: Long, gasPrice: Double, floorUmc: Long): Long {
        if (gasLimit <= 0 || !gasPrice.isFinite() || gasPrice <= 0.0) return floorUmc
        val raw = ceil(gasLimit * gasPrice)
        if (!raw.isFinite() || raw >= Long.MAX_VALUE.toDouble()) return Long.MAX_VALUE
        return maxOf(floorUmc, raw.toLong())
    }
}
