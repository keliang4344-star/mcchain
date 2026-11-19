package com.mcchain.miner.shared.util

/**
 * SHA-512 / HMAC-SHA-512 / PBKDF2-HMAC-SHA-512（纯 Kotlin，commonMain 可用）。
 *
 * 用途：BIP-39 助记词 → 种子（PBKDF2-HMAC-SHA512, 2048 轮）、BIP-32 子密钥派生（HMAC-SHA512）。
 * iOS 上 CommonCrypto 能算 HMAC-SHA512，但派生逻辑放在 commonMain 可与 Android 保持同码，
 * 避免两端派生路径实现漂移。常量由 2/3 次方根小数部分生成，jvmTest 用标准向量校验。
 */
object Sha512 {

    private const val BLOCK_SIZE = 128

    private val K = longArrayOf(
        0x428A2F98D728AE22L, 0x7137449123EF65CDL, 0xB5C0FBCFEC4D3B2FuL.toLong(), 0xE9B5DBA58189DBBCuL.toLong(),
        0x3956C25BF348B538L, 0x59F111F1B605D019L, 0x923F82A4AF194F9BuL.toLong(), 0xAB1C5ED5DA6D8118uL.toLong(),
        0xD807AA98A3030242uL.toLong(), 0x12835B0145706FBEL, 0x243185BE4EE4B28CL, 0x550C7DC3D5FFB4E2L,
        0x72BE5D74F27B896FL, 0x80DEB1FE3B1696B1uL.toLong(), 0x9BDC06A725C71235uL.toLong(), 0xC19BF174CF692694uL.toLong(),
        0xE49B69C19EF14AD2uL.toLong(), 0xEFBE4786384F25E3uL.toLong(), 0x0FC19DC68B8CD5B5L, 0x240CA1CC77AC9C65L,
        0x2DE92C6F592B0275L, 0x4A7484AA6EA6E483L, 0x5CB0A9DCBD41FBD4L, 0x76F988DA831153B5L,
        0x983E5152EE66DFABuL.toLong(), 0xA831C66D2DB43210uL.toLong(), 0xB00327C898FB213FuL.toLong(), 0xBF597FC7BEEF0EE4uL.toLong(),
        0xC6E00BF33DA88FC2uL.toLong(), 0xD5A79147930AA725uL.toLong(), 0x06CA6351E003826FL, 0x142929670A0E6E70L,
        0x27B70A8546D22FFCL, 0x2E1B21385C26C926L, 0x4D2C6DFC5AC42AEDL, 0x53380D139D95B3DFL,
        0x650A73548BAF63DEL, 0x766A0ABB3C77B2A8L, 0x81C2C92E47EDAEE6uL.toLong(), 0x92722C851482353BuL.toLong(),
        0xA2BFE8A14CF10364uL.toLong(), 0xA81A664BBC423001uL.toLong(), 0xC24B8B70D0F89791uL.toLong(), 0xC76C51A30654BE30uL.toLong(),
        0xD192E819D6EF5218uL.toLong(), 0xD69906245565A910uL.toLong(), 0xF40E35855771202AuL.toLong(), 0x106AA07032BBD1B8L,
        0x19A4C116B8D2D0C8L, 0x1E376C085141AB53L, 0x2748774CDF8EEB99L, 0x34B0BCB5E19B48A8L,
        0x391C0CB3C5C95A63L, 0x4ED8AA4AE3418ACBL, 0x5B9CCA4F7763E373L, 0x682E6FF3D6B2B8A3L,
        0x748F82EE5DEFB2FCL, 0x78A5636F43172F60L, 0x84C87814A1F0AB72uL.toLong(), 0x8CC702081A6439ECuL.toLong(),
        0x90BEFFFA23631E28uL.toLong(), 0xA4506CEBDE82BDE9uL.toLong(), 0xBEF9A3F7B2C67915uL.toLong(), 0xC67178F2E372532BuL.toLong(),
        0xCA273ECEEA26619CuL.toLong(), 0xD186B8C721C0C207uL.toLong(), 0xEADA7DD6CDE0EB1EuL.toLong(), 0xF57D4F7FEE6ED178uL.toLong(),
        0x06F067AA72176FBAL, 0x0A637DC5A2C898A6L, 0x113F9804BEF90DAEL, 0x1B710B35131C471BL,
        0x28DB77F523047D84L, 0x32CAAB7B40C72493L, 0x3C9EBE0A15C9BEBCL, 0x431D67C49C100D4CL,
        0x4CC5D4BECB3E42B6L, 0x597F299CFC657E2AL, 0x5FCB6FAB3AD6FAECL, 0x6C44198C4A475817L,
    )

    fun digest(input: ByteArray): ByteArray {
        val h = longArrayOf(
            0x6A09E667F3BCC908L, 0xBB67AE8584CAA73BuL.toLong(), 0x3C6EF372FE94F82BL, 0xA54FF53A5F1D36F1uL.toLong(),
            0x510E527FADE682D1L, 0x9B05688C2B3E6C1FuL.toLong(), 0x1F83D9ABFB41BD6BL, 0x5BE0CD19137E2179L,
        )

        val msgLen = input.size
        // 0x80 + 补零，使 (len+1+pad) % 128 == 112，再补 16 字节大端长度（高 8 字节恒为 0）
        val padLen = (112 - (msgLen + 1) % 128 + 128) % 128
        val padded = ByteArray(msgLen + 1 + padLen + 16)
        input.copyInto(padded)
        padded[msgLen] = 0x80.toByte()
        val bitLen = msgLen.toLong() * 8
        for (i in 0 until 8) {
            padded[padded.size - 1 - i] = ((bitLen ushr (8 * i)) and 0xFF).toByte()
        }

        val w = LongArray(80)
        var offset = 0
        while (offset < padded.size) {
            for (i in 0 until 16) {
                var v = 0L
                for (b in 0 until 8) {
                    v = (v shl 8) or (padded[offset + i * 8 + b].toLong() and 0xFF)
                }
                w[i] = v
            }
            for (i in 16 until 80) {
                val s0 = w[i - 15].ror(1) xor w[i - 15].ror(8) xor (w[i - 15] ushr 7)
                val s1 = w[i - 2].ror(19) xor w[i - 2].ror(61) xor (w[i - 2] ushr 6)
                w[i] = w[i - 16] + s0 + w[i - 7] + s1
            }

            var a = h[0]; var b = h[1]; var c = h[2]; var d = h[3]
            var e = h[4]; var f = h[5]; var g = h[6]; var hh = h[7]

            for (i in 0 until 80) {
                val s1 = e.ror(14) xor e.ror(18) xor e.ror(41)
                val ch = (e and f) xor (e.inv() and g)
                val temp1 = hh + s1 + ch + K[i] + w[i]
                val s0 = a.ror(28) xor a.ror(34) xor a.ror(39)
                val maj = (a and b) xor (a and c) xor (b and c)
                val temp2 = s0 + maj

                hh = g; g = f; f = e
                e = d + temp1
                d = c; c = b; b = a
                a = temp1 + temp2
            }

            h[0] += a; h[1] += b; h[2] += c; h[3] += d
            h[4] += e; h[5] += f; h[6] += g; h[7] += hh
            offset += 128
        }

        val out = ByteArray(64)
        for (i in 0 until 8) {
            for (b in 0 until 8) {
                out[i * 8 + b] = (h[i] ushr (56 - 8 * b)).toByte()
            }
        }
        return out
    }

    fun hmac(key: ByteArray, message: ByteArray): ByteArray {
        val block = ByteArray(BLOCK_SIZE)
        val k = if (key.size > BLOCK_SIZE) digest(key) else key
        k.copyInto(block)
        val ipad = ByteArray(BLOCK_SIZE) { (block[it].toInt() xor 0x36).toByte() }
        val opad = ByteArray(BLOCK_SIZE) { (block[it].toInt() xor 0x5C).toByte() }
        return digest(opad + digest(ipad + message))
    }

    /** PBKDF2-HMAC-SHA512。移动端只用于 BIP-39（dkLen = 64，单块）。 */
    fun pbkdf2(password: ByteArray, salt: ByteArray, iterations: Int, dkLen: Int): ByteArray {
        require(iterations > 0) { "iterations must be > 0" }
        require(dkLen > 0) { "dkLen must be > 0" }
        val out = ByteArray(dkLen)
        var blockIndex = 1
        var copied = 0
        while (copied < dkLen) {
            val u = hmac(password, salt + byteArrayOf(
                (blockIndex ushr 24).toByte(),
                (blockIndex ushr 16).toByte(),
                (blockIndex ushr 8).toByte(),
                blockIndex.toByte(),
            ))
            var prev = u
            var t = u.copyOf()
            for (i in 1 until iterations) {
                prev = hmac(password, prev)
                for (j in t.indices) t[j] = (t[j].toInt() xor prev[j].toInt()).toByte()
            }
            val take = minOf(dkLen - copied, t.size)
            t.copyInto(out, copied, 0, take)
            copied += take
            blockIndex++
        }
        return out
    }

    private fun Long.ror(bits: Int): Long = (this ushr bits) or (this shl (64 - bits))
}
