package com.mcchain.miner.shared.util

/**
 * 设备认证有效期判定（纯函数）。
 *
 * 链上认证状态以 depin/phonenode 记录为准；这里只处理本机保存的到期时间，
 * 用于决定是否允许提交贡献/接单，以及 UI 的过期状态。
 */
object AttestationPolicy {

    /** 有 token 且当前时间早于到期时间才算有效。 */
    fun isValid(token: String?, expiry: Long, now: Long): Boolean =
        !token.isNullOrBlank() && expiry > now

    /** 仅按到期时间判断（没有 token 的场景，例如节点认证步骤）。 */
    fun isUnexpired(expiry: Long, now: Long): Boolean = expiry > now

    /** 有到期记录且已过期；expiry <= 0 视为无记录，不判定为过期。 */
    fun isExpired(expiry: Long, now: Long): Boolean = expiry > 0 && now >= expiry
}
