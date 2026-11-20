package com.mcchain.miner.shared.util

import java.math.BigDecimal
import java.math.BigInteger
import java.math.RoundingMode

/**
 * MC / umc 金额换算与展示（1 MC = 1,000,000 umc）。
 *
 * 统一规则：
 *  - 输入只接受普通十进制、最多 6 位小数，拒绝负数、科学计数法、空白与非数字；
 *  - 解析按最小单位向下取整，但超过 6 位小数直接拒绝，避免静默丢失精度；
 *  - 展示不使用科学计数法，去掉末尾多余的 0。
 */
object McAmount {

    const val DECIMALS = 6
    private val SCALE = BigDecimal(1_000_000)
    private val PATTERN = Regex("^\\d{1,18}(\\.\\d{1,6})?$")
    private val MAX_LONG = BigInteger.valueOf(Long.MAX_VALUE)

    /** MC 字符串转 umc；非法、负数、超过 6 位小数或溢出返回 null。 */
    fun parseToUmc(input: String): Long? {
        val text = input.trim()
        if (!PATTERN.matches(text)) return null
        val value = text.toBigDecimalOrNull() ?: return null
        val umc = value.multiply(SCALE).setScale(0, RoundingMode.DOWN).toBigInteger()
        if (umc > MAX_LONG) return null
        return umc.toLong()
    }

    /** umc 转展示字符串（MC），最多 6 位小数、无千分位、无科学计数法。 */
    fun format(umc: Long): String {
        if (umc == 0L) return "0"
        if (umc < 0L) return "-" + format(-umc)
        return BigDecimal(umc)
            .divide(SCALE, DECIMALS, RoundingMode.DOWN)
            .stripTrailingZeros()
            .toPlainString()
            .ifBlank { "0" }
    }
}
