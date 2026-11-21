package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.FeeRules
import org.junit.Assert.assertEquals
import org.junit.Test

class FeeRulesTest {

    @Test
    fun feeIsCeilOfGasTimesPrice() {
        // 1000 × 0.0255 = 25.5 -> 26（向上取整，避免差 1 umc 被节点拒绝）。
        assertEquals(26L, FeeRules.minFee(1_000L, 0.0255, 1L))
    }

    @Test
    fun floorWinsForSmallGas() {
        // 150000 × 0.025 = 3750，低于 5000 下限。
        assertEquals(5_000L, FeeRules.minFee(150_000L, 0.025, 5_000L))
    }

    @Test
    fun computedFeeWinsAboveFloor() {
        assertEquals(10_000L, FeeRules.minFee(400_000L, 0.025, 5_000L))
    }

    @Test
    fun invalidInputsFallBackToFloor() {
        assertEquals(5_000L, FeeRules.minFee(0L, 0.025, 5_000L))
        assertEquals(5_000L, FeeRules.minFee(-1L, 0.025, 5_000L))
        assertEquals(5_000L, FeeRules.minFee(150_000L, 0.0, 5_000L))
        assertEquals(5_000L, FeeRules.minFee(150_000L, -1.0, 5_000L))
        assertEquals(5_000L, FeeRules.minFee(150_000L, Double.NaN, 5_000L))
    }

    @Test
    fun overflowDoesNotProduceNegativeFee() {
        // Long.MAX_VALUE × 1.0 超出 Long 上限，必须饱和到 MAX 而不是溢出为负数。
        assertEquals(Long.MAX_VALUE, FeeRules.minFee(Long.MAX_VALUE, 1.0, 5_000L))
    }
}
