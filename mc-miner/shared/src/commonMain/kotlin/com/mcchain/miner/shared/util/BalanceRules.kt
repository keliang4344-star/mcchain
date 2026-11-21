package com.mcchain.miner.shared.util

/** 余额预检结果，由 app 映射为中文提示。 */
enum class BalanceIssue {
    NONE,
    ZERO_BALANCE,
    INVALID_AMOUNT,
    INSUFFICIENT_AMOUNT,
    INSUFFICIENT_FEE,
}

/**
 * 余额与手续费预检（纯函数）。
 *
 * 只做本地快速失败，最终仍以链上余额与广播结果为准。
 * fee 单位为 umc，必须为非负。
 */
object BalanceRules {

    /** 支付「金额 + 手续费」前检查。 */
    fun spendIssue(balance: Long, amount: Long, fee: Long): BalanceIssue = when {
        balance <= 0 -> BalanceIssue.ZERO_BALANCE
        amount <= 0 -> BalanceIssue.INVALID_AMOUNT
        fee < 0 -> BalanceIssue.INSUFFICIENT_FEE
        amount > balance - fee -> BalanceIssue.INSUFFICIENT_AMOUNT
        else -> BalanceIssue.NONE
    }

    /** 只支付手续费的场景（解除委托、转委托、领取奖励等）。 */
    fun feeIssue(balance: Long, fee: Long): BalanceIssue =
        if (fee < 0 || balance < fee) BalanceIssue.INSUFFICIENT_FEE else BalanceIssue.NONE
}
