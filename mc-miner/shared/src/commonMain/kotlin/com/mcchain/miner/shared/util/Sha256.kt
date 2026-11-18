package com.mcchain.miner.shared.util

/**
 * 纯 Kotlin SHA-256（FIPS 180-4）。
 *
 * 放在 commonMain 而不是依赖平台 API：Cosmos 的 SignDoc 摘要必须两端一致，
 * 且 Phase 1 的 iOS 侧不需要再引入额外哈希库。
 */
object Sha256 {
    private const val MASK = 0xFFFFFFFFL

    private val K = longArrayOf(
        0x428a2f98L, 0x71374491L, 0xb5c0fbcfL, 0xe9b5dba5L, 0x3956c25bL, 0x59f111f1L, 0x923f82a4L, 0xab1c5ed5L,
        0xd807aa98L, 0x12835b01L, 0x243185beL, 0x550c7dc3L, 0x72be5d74L, 0x80deb1feL, 0x9bdc06a7L, 0xc19bf174L,
        0xe49b69c1L, 0xefbe4786L, 0x0fc19dc6L, 0x240ca1ccL, 0x2de92c6fL, 0x4a7484aaL, 0x5cb0a9dcL, 0x76f988daL,
        0x983e5152L, 0xa831c66dL, 0xb00327c8L, 0xbf597fc7L, 0xc6e00bf3L, 0xd5a79147L, 0x06ca6351L, 0x14292967L,
        0x27b70a85L, 0x2e1b2138L, 0x4d2c6dfcL, 0x53380d13L, 0x650a7354L, 0x766a0abbL, 0x81c2c92eL, 0x92722c85L,
        0xa2bfe8a1L, 0xa81a664bL, 0xc24b8b70L, 0xc76c51a3L, 0xd192e819L, 0xd6990624L, 0xf40e3585L, 0x106aa070L,
        0x19a4c116L, 0x1e376c08L, 0x2748774cL, 0x34b0bcb5L, 0x391c0cb3L, 0x4ed8aa4aL, 0x5b9cca4fL, 0x682e6ff3L,
        0x748f82eeL, 0x78a5636fL, 0x84c87814L, 0x8cc70208L, 0x90befffaL, 0xa4506cebL, 0xbef9a3f7L, 0xc67178f2L,
    )

    fun digest(input: ByteArray): ByteArray {
        val bitLength = input.size.toLong() * 8L
        val padZeros = (56 - (input.size + 1) % 64 + 64) % 64
        val data = ByteArray(input.size + 1 + padZeros + 8)
        input.copyInto(data)
        data[input.size] = 0x80.toByte()
        for (i in 0 until 8) {
            data[data.size - 1 - i] = ((bitLength ushr (8 * i)) and 0xFFL).toByte()
        }

        var h0 = 0x6a09e667L
        var h1 = 0xbb67ae85L
        var h2 = 0x3c6ef372L
        var h3 = 0xa54ff53aL
        var h4 = 0x510e527fL
        var h5 = 0x9b05688cL
        var h6 = 0x1f83d9abL
        var h7 = 0x5be0cd19L

        val w = LongArray(64)
        var offset = 0
        while (offset < data.size) {
            for (i in 0 until 16) {
                var v = 0L
                for (j in 0 until 4) {
                    v = (v shl 8) or (data[offset + i * 4 + j].toLong() and 0xFFL)
                }
                w[i] = v
            }
            for (i in 16 until 64) {
                val x = w[i - 15]
                val y = w[i - 2]
                val s0 = rotr(x, 7) xor rotr(x, 18) xor (x ushr 3)
                val s1 = rotr(y, 17) xor rotr(y, 19) xor (y ushr 10)
                w[i] = (w[i - 16] + s0 + w[i - 7] + s1) and MASK
            }

            var a = h0
            var b = h1
            var c = h2
            var d = h3
            var e = h4
            var f = h5
            var g = h6
            var h = h7

            for (i in 0 until 64) {
                val s1 = rotr(e, 6) xor rotr(e, 11) xor rotr(e, 25)
                val ch = (e and f) xor (e.inv() and g)
                val temp1 = (h + s1 + ch + K[i] + w[i]) and MASK
                val s0 = rotr(a, 2) xor rotr(a, 13) xor rotr(a, 22)
                val maj = (a and b) xor (a and c) xor (b and c)
                val temp2 = (s0 + maj) and MASK

                h = g
                g = f
                f = e
                e = (d + temp1) and MASK
                d = c
                c = b
                b = a
                a = (temp1 + temp2) and MASK
            }

            h0 = (h0 + a) and MASK
            h1 = (h1 + b) and MASK
            h2 = (h2 + c) and MASK
            h3 = (h3 + d) and MASK
            h4 = (h4 + e) and MASK
            h5 = (h5 + f) and MASK
            h6 = (h6 + g) and MASK
            h7 = (h7 + h) and MASK

            offset += 64
        }

        val out = ByteArray(32)
        val state = longArrayOf(h0, h1, h2, h3, h4, h5, h6, h7)
        for (i in 0 until 8) {
            for (j in 0 until 4) {
                out[i * 4 + j] = ((state[i] ushr (24 - 8 * j)) and 0xFFL).toByte()
            }
        }
        return out
    }

    private fun rotr(x: Long, n: Int): Long = ((x ushr n) or (x shl (32 - n))) and MASK
}
