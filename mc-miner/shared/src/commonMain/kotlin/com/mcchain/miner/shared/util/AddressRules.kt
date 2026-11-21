package com.mcchain.miner.shared.util

/**
 * 账户与验证人地址校验（纯函数，供 App 与 shared 共用）。
 *
 * 返回 null 表示通过；否则返回可直接展示的中文原因。
 * 只做格式与字符集校验，不查链上状态。
 */
object AddressRules {

    /** 普通账户/收款地址。accountHrp 例如 "mc"，完整前缀为 "mc1"。 */
    fun accountError(address: String, accountHrp: String): String? {
        val prefix = accountHrp + "1"
        return when {
            address.isEmpty() -> "请输入收款地址"
            !address.startsWith(prefix) -> "收款地址必须以 $prefix 开头"
            address.length !in 39..59 -> "收款地址长度异常，请检查是否粘贴完整"
            !Bech32Address.hasValidDataPart(address, accountHrp) ->
                "收款地址包含非法字符，请检查是否复制正确"
            else -> null
        }
    }

    /** 验证人地址。accountHrp 例如 "mc"，验证人 HRP 为 "mcvaloper"。 */
    fun validatorError(address: String, accountHrp: String): String? {
        val prefix = accountHrp + "valoper1"
        return when {
            address.isEmpty() -> "请输入验证人地址"
            !address.startsWith(prefix) -> "验证人地址必须以 $prefix 开头"
            address.length !in 48..62 -> "验证人地址长度异常，请检查是否粘贴完整"
            !Bech32Address.hasValidDataPart(address, accountHrp + "valoper") ->
                "验证人地址包含非法字符"
            else -> null
        }
    }
}
