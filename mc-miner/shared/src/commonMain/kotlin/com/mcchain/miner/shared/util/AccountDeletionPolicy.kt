package com.mcchain.miner.shared.util

/** 本机账户删除的前置检查结果；中文提示由调用方映射。 */
enum class AccountDeletionIssue { NONE, LAST_ACCOUNT, HAS_ASSETS, HAS_PENDING_TX }

/**
 * 账户删除策略（纯函数）。
 *
 * 删除只移除本机记录，链上地址与资产不会消失；因此只要账户仍有余额、质押、
 * 待领奖励或未确认交易，就必须阻断，避免用户误以为资产被一并清理。
 */
object AccountDeletionPolicy {

    fun issue(
        accountCount: Int,
        balance: Long,
        stake: Long,
        reward: Long,
        hasPendingTx: Boolean,
    ): AccountDeletionIssue = when {
        accountCount <= 1 -> AccountDeletionIssue.LAST_ACCOUNT
        balance > 0 || stake > 0 || reward > 0 -> AccountDeletionIssue.HAS_ASSETS
        hasPendingTx -> AccountDeletionIssue.HAS_PENDING_TX
        else -> AccountDeletionIssue.NONE
    }
}
