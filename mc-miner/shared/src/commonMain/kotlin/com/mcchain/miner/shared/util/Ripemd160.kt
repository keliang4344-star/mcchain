package com.mcchain.miner.shared.util

/**
 * RIPEMD-160（纯 Kotlin，commonMain 可用）。
 *
 * 用途：Cosmos 地址派生 hash160 = RIPEMD160(SHA256(压缩公钥))。
 * 移动端 iOS 没有现成的 RIPEMD-160（CommonCrypto 不提供），因此这里自带实现；
 * 常量与轮函数对照 BouncyCastle `RIPEMD160Digest`，jvmTest 用标准向量（空串/abc/百万元素）校验。
 */
object Ripemd160 {

    private const val MASK = 0xFFFFFFFFL

    /** 消息字序（左线） */
    private val RL = intArrayOf(
        0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
        7, 4, 13, 1, 10, 6, 15, 3, 12, 0, 9, 5, 2, 14, 11, 8,
        3, 10, 14, 4, 9, 15, 8, 1, 2, 7, 0, 6, 13, 11, 5, 12,
        1, 9, 11, 10, 0, 8, 12, 4, 13, 3, 7, 15, 14, 5, 6, 2,
        4, 0, 5, 9, 7, 12, 2, 10, 14, 1, 3, 8, 11, 6, 15, 13,
    )

    /** 左移位数（左线） */
    private val SL = intArrayOf(
        11, 14, 15, 12, 5, 8, 7, 9, 11, 13, 14, 15, 6, 7, 9, 8,
        7, 6, 8, 13, 11, 9, 7, 15, 7, 12, 15, 9, 11, 7, 13, 12,
        11, 13, 6, 7, 14, 9, 13, 15, 14, 8, 13, 6, 5, 12, 7, 5,
        11, 12, 14, 15, 14, 15, 9, 8, 9, 14, 5, 6, 8, 6, 5, 12,
        9, 15, 5, 11, 6, 8, 13, 12, 5, 12, 13, 14, 11, 8, 5, 6,
    )

    /** 消息字序（右线） */
    private val RR = intArrayOf(
        5, 14, 7, 0, 9, 2, 11, 4, 13, 6, 15, 8, 1, 10, 3, 12,
        6, 11, 3, 7, 0, 13, 5, 10, 14, 15, 8, 12, 4, 9, 1, 2,
        15, 5, 1, 3, 7, 14, 6, 9, 11, 8, 12, 2, 10, 0, 4, 13,
        8, 6, 4, 1, 3, 11, 15, 0, 5, 12, 2, 13, 9, 7, 10, 14,
        12, 15, 10, 4, 1, 5, 8, 7, 6, 2, 13, 14, 0, 3, 9, 11,
    )

    /** 左移位数（右线） */
    private val SR = intArrayOf(
        8, 9, 9, 11, 13, 15, 15, 5, 7, 7, 8, 11, 14, 14, 12, 6,
        9, 13, 15, 7, 12, 8, 9, 11, 7, 7, 12, 7, 6, 15, 13, 11,
        9, 7, 15, 11, 8, 6, 6, 14, 12, 13, 5, 14, 13, 13, 7, 5,
        15, 5, 8, 11, 14, 14, 6, 14, 6, 9, 12, 9, 12, 5, 15, 8,
        8, 5, 12, 9, 12, 5, 14, 6, 8, 13, 6, 5, 15, 13, 11, 11,
    )

    private val KL = intArrayOf(0x00000000, 0x5A827999, 0x6ED9EBA1, 0x8F1BBCDC.toInt(), 0xA953FD4E.toInt())
    private val KR = intArrayOf(0x50A28BE6, 0x5C4DD124, 0x6D703EF3, 0x7A6D76E9, 0x00000000)

    fun digest(input: ByteArray): ByteArray {
        val h = intArrayOf(0x67452301, 0xEFCDAB89.toInt(), 0x98BADCFE.toInt(), 0x10325476, 0xC3D2E1F0.toInt())

        val msgLen = input.size
        val bitLen = msgLen.toLong() * 8
        // 0x80 + 补零 + 8 字节小端长度，对齐到 64 字节
        val padLen = (56 - (msgLen + 1) % 64 + 64) % 64
        val padded = ByteArray(msgLen + 1 + padLen + 8)
        input.copyInto(padded)
        padded[msgLen] = 0x80.toByte()
        for (i in 0 until 8) {
            padded[padded.size - 8 + i] = ((bitLen ushr (8 * i)) and 0xFF).toByte()
        }

        val x = IntArray(16)
        var offset = 0
        while (offset < padded.size) {
            for (i in 0 until 16) {
                val p = offset + i * 4
                x[i] = (padded[p].toInt() and 0xFF) or
                    ((padded[p + 1].toInt() and 0xFF) shl 8) or
                    ((padded[p + 2].toInt() and 0xFF) shl 16) or
                    ((padded[p + 3].toInt() and 0xFF) shl 24)
            }
            processBlock(h, x)
            offset += 64
        }

        val out = ByteArray(20)
        for (i in 0 until 5) {
            out[i * 4] = h[i].toByte()
            out[i * 4 + 1] = (h[i] ushr 8).toByte()
            out[i * 4 + 2] = (h[i] ushr 16).toByte()
            out[i * 4 + 3] = (h[i] ushr 24).toByte()
        }
        return out
    }

    private fun processBlock(h: IntArray, x: IntArray) {
        var al = h[0]
        var bl = h[1]
        var cl = h[2]
        var dl = h[3]
        var el = h[4]
        var ar = h[0]
        var br = h[1]
        var cr = h[2]
        var dr = h[3]
        var er = h[4]

        for (j in 0 until 80) {
            val round = j / 16

            // T = rol(A + f(B,C,D) + X[r] + K, s) + E；随后 (A,B,C,D,E) ← (E, T, B, rol(C,10), D)
            val tl = rol(al + f(round, bl, cl, dl) + x[RL[j]] + KL[round], SL[j]).toLong() + (el.toLong() and MASK)
            val tr = rol(ar + f(4 - round, br, cr, dr) + x[RR[j]] + KR[round], SR[j]).toLong() + (er.toLong() and MASK)

            val nextAl = el
            val nextEl = dl
            val nextDl = rol(cl, 10)
            val nextCl = bl
            al = nextAl
            el = nextEl
            dl = nextDl
            cl = nextCl
            bl = (tl and MASK).toInt()

            val nextAr = er
            val nextEr = dr
            val nextDr = rol(cr, 10)
            val nextCr = br
            ar = nextAr
            er = nextEr
            dr = nextDr
            cr = nextCr
            br = (tr and MASK).toInt()
        }

        val t = h[1] + cl + dr
        h[1] = h[2] + dl + er
        h[2] = h[3] + el + ar
        h[3] = h[4] + al + br
        h[4] = h[0] + bl + cr
        h[0] = t
    }

    private fun f(round: Int, x: Int, y: Int, z: Int): Int = when (round) {
        0 -> x xor y xor z
        1 -> (x and y) or (x.inv() and z)
        2 -> (x or y.inv()) xor z
        3 -> (x and z) or (y and z.inv())
        else -> x xor (y or z.inv())
    }

    private fun rol(x: Int, n: Int): Int = (x shl n) or (x ushr (32 - n))
}
