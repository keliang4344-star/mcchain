package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.AccountDeletionIssue
import com.mcchain.miner.shared.util.AccountDeletionPolicy
import org.junit.Assert.assertEquals
import org.junit.Test

class AccountDeletionPolicyTest {

    @Test
    fun lastAccountIsAlwaysProtected() {
        assertEquals(
            AccountDeletionIssue.LAST_ACCOUNT,
            AccountDeletionPolicy.issue(
                accountCount = 1,
                balance = 1_000_000,
                stake = 2_000_000,
                reward = 3_000,
                hasPendingTx = true,
            ),
        )
    }

    @Test
    fun emptyAccountCanBeDeleted() {
        assertEquals(
            AccountDeletionIssue.NONE,
            AccountDeletionPolicy.issue(2, 0, 0, 0, hasPendingTx = false),
        )
    }

    @Test
    fun anyRemainingAssetBlocksDeletion() {
        assertEquals(
            AccountDeletionIssue.HAS_ASSETS,
            AccountDeletionPolicy.issue(2, balance = 1, stake = 0, reward = 0, hasPendingTx = false),
        )
        assertEquals(
            AccountDeletionIssue.HAS_ASSETS,
            AccountDeletionPolicy.issue(2, balance = 0, stake = 1, reward = 0, hasPendingTx = false),
        )
        assertEquals(
            AccountDeletionIssue.HAS_ASSETS,
            AccountDeletionPolicy.issue(2, balance = 0, stake = 0, reward = 1, hasPendingTx = false),
        )
    }

    @Test
    fun pendingTransactionBlocksDeletion() {
        assertEquals(
            AccountDeletionIssue.HAS_PENDING_TX,
            AccountDeletionPolicy.issue(2, 0, 0, 0, hasPendingTx = true),
        )
    }

    @Test
    fun assetsTakePrecedenceOverPendingTransaction() {
        assertEquals(
            AccountDeletionIssue.HAS_ASSETS,
            AccountDeletionPolicy.issue(2, balance = 1, stake = 0, reward = 0, hasPendingTx = true),
        )
    }
}
