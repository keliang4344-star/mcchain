package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.BalanceIssue
import com.mcchain.miner.shared.util.BalanceRules
import org.junit.Assert.assertEquals
import org.junit.Test

class BalanceRulesTest {

    @Test
    fun sufficientBalancePasses() {
        assertEquals(BalanceIssue.NONE, BalanceRules.spendIssue(10_000L, 1_000L, 5_000L))
    }

    @Test
    fun exactBoundaryPasses() {
        // amount + fee 恰好等于余额时允许继续（链上最终裁定）。
        assertEquals(BalanceIssue.NONE, BalanceRules.spendIssue(6_000L, 1_000L, 5_000L))
    }

    @Test
    fun zeroBalanceIsReportedFirst() {
        assertEquals(BalanceIssue.ZERO_BALANCE, BalanceRules.spendIssue(0L, 1_000L, 5_000L))
        assertEquals(BalanceIssue.ZERO_BALANCE, BalanceRules.spendIssue(-1L, 0L, 0L))
    }

    @Test
    fun nonPositiveAmountIsRejected() {
        assertEquals(BalanceIssue.INVALID_AMOUNT, BalanceRules.spendIssue(10_000L, 0L, 5_000L))
        assertEquals(BalanceIssue.INVALID_AMOUNT, BalanceRules.spendIssue(10_000L, -1L, 5_000L))
    }

    @Test
    fun amountPlusFeeOverBalanceIsRejected() {
        assertEquals(BalanceIssue.INSUFFICIENT_AMOUNT, BalanceRules.spendIssue(6_000L, 2_000L, 5_000L))
    }

    @Test
    fun negativeFeeIsRejected() {
        assertEquals(BalanceIssue.INSUFFICIENT_FEE, BalanceRules.spendIssue(10_000L, 1_000L, -1L))
    }

    @Test
    fun feeOnlyRules() {
        assertEquals(BalanceIssue.NONE, BalanceRules.feeIssue(5_000L, 5_000L))
        assertEquals(BalanceIssue.INSUFFICIENT_FEE, BalanceRules.feeIssue(4_999L, 5_000L))
        assertEquals(BalanceIssue.INSUFFICIENT_FEE, BalanceRules.feeIssue(10_000L, -1L))
    }
}
