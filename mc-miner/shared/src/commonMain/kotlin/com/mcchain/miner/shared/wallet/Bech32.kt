package com.mcchain.miner.shared.wallet

/**
 * bech32 编码（BIP-173，Cosmos 账户地址使用原始 bech32 而非 bech32m）。
 *
 * 只实现编码侧：移动端只需要把 20 字节 hash160 转成 `mc1...` 地址，
 * 收款地址的校验由链上/REST 负责，客户端不需要解码。
 */
object Bech32 {

    private const val CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
    private val GENERATOR = intArrayOf(0x3B6A57B2, 0x26508E6D, 0x1EA119FA, 0x3D4233DD, 0x2A1462B3)

    fun encode(hrp: String, data: ByteArray): String {
        require(hrp.isNotEmpty()) { "hrp 不能为空" }
        val checksum = createChecksum(hrp, data)
        val builder = StringBuilder(hrp.length + 1 + data.size + 6)
        builder.append(hrp).append('1')
        for (b in data) builder.append(CHARSET[b.toInt() and 0x1F])
        for (b in checksum) builder.append(CHARSET[b.toInt() and 0x1F])
        return builder.toString()
    }

    /** 8-bit 字节流 → 5-bit 组（pad=true 时尾部补零）。 */
    fun convertBits(data: ByteArray, fromBits: Int, toBits: Int, pad: Boolean): ByteArray {
        var acc = 0
        var bits = 0
        val maxv = (1 shl toBits) - 1
        val out = ArrayList<Byte>(data.size * fromBits / toBits + 1)
        for (value in data) {
            val b = value.toInt() and 0xFF
            require(b ushr fromBits == 0) { "invalid data range" }
            acc = (acc shl fromBits) or b
            bits += fromBits
            while (bits >= toBits) {
                bits -= toBits
                out.add(((acc ushr bits) and maxv).toByte())
            }
        }
        if (pad) {
            if (bits > 0) out.add(((acc shl (toBits - bits)) and maxv).toByte())
        } else {
            require(bits < fromBits && ((acc shl (toBits - bits)) and maxv) == 0) { "invalid padding" }
        }
        return out.toByteArray()
    }

    private fun createChecksum(hrp: String, data: ByteArray): ByteArray {
        val values = hrpExpand(hrp) + data.toList().map { it.toInt() and 0x1F } + listOf(0, 0, 0, 0, 0, 0)
        val mod = polymod(values) xor 1
        return ByteArray(6) { ((mod ushr (5 * (5 - it))) and 31).toByte() }
    }

    private fun hrpExpand(hrp: String): List<Int> {
        val result = ArrayList<Int>(hrp.length * 2 + 1)
        for (c in hrp) result.add(c.code ushr 5)
        result.add(0)
        for (c in hrp) result.add(c.code and 31)
        return result
    }

    private fun polymod(values: List<Int>): Int {
        var chk = 1
        for (value in values) {
            val top = chk ushr 25
            chk = ((chk and 0x1FFFFFF) shl 5) xor value
            for (i in 0 until 5) {
                if (((top ushr i) and 1) != 0) chk = chk xor GENERATOR[i]
            }
        }
        return chk
    }
}
