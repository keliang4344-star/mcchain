package com.mcchain.miner.shared.util

/**
 * bech32 地址的轻量校验（不验证 checksum）。
 *
 * 注意：分隔符 `1` 不属于 bech32 数据字符集，校验时必须跳过 `hrp1` 前缀，
 * 否则所有合法地址都会被判为「包含非法字符」。
 */
object Bech32Address {

    private const val CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

    /** 地址是否以 `hrp1` 开头，且分隔符后的数据部分字符全部合法。 */
    fun hasValidDataPart(address: String, hrp: String): Boolean {
        val prefix = "$hrp" + "1"
        if (!address.startsWith(prefix)) return false
        val data = address.substring(prefix.length)
        if (data.isEmpty()) return false
        return data.all { it in CHARSET }
    }
}
