package com.mcchain.miner.shared.util

object Hex {
    private const val LOWER = "0123456789abcdef"
    private const val UPPER = "0123456789ABCDEF"

    fun encode(bytes: ByteArray, upper: Boolean = false): String {
        val table = if (upper) UPPER else LOWER
        val out = StringBuilder(bytes.size * 2)
        for (b in bytes) {
            val v = b.toInt() and 0xFF
            out.append(table[v ushr 4]).append(table[v and 0x0F])
        }
        return out.toString()
    }

    fun decode(hex: String): ByteArray {
        require(hex.length % 2 == 0) { "hex length must be even: ${hex.length}" }
        val out = ByteArray(hex.length / 2)
        for (i in out.indices) {
            val hi = nibble(hex[i * 2])
            val lo = nibble(hex[i * 2 + 1])
            out[i] = ((hi shl 4) or lo).toByte()
        }
        return out
    }

    private fun nibble(c: Char): Int = when (c) {
        in '0'..'9' -> c - '0'
        in 'a'..'f' -> c - 'a' + 10
        in 'A'..'F' -> c - 'A' + 10
        else -> throw IllegalArgumentException("invalid hex char: $c")
    }
}
