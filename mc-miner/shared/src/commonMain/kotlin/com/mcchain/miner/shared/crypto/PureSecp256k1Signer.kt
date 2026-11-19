package com.mcchain.miner.shared.crypto

import com.mcchain.miner.shared.util.Sha256

/**
 * 纯 Kotlin secp256k1 签名器（commonMain，iOS/Android/JVM 共用同一份代码）。
 *
 * 与 Cosmos SDK `secp256k1.PrivKey.Sign` 对齐：
 *  - RFC6979（HMAC-SHA256）确定性 nonce
 *  - low-S 规范化，输出 64 字节 r||s
 *  - 对入参摘要直接签名（调用方已做 SHA-256）
 *
 * 正确性锚点（jvmTest）：与 BouncyCastle 实现（Secp256k1Signer）逐字节对照 +
 * mcchaind 生成的 golden 交易向量。RFC6979 是确定性的，两者必须完全一致。
 */
class PureSecp256k1Signer(privateKey32: ByteArray) : TxSigner {

    private val d: IntArray = Secp256k1Math.fromBytesBE(privateKey32)
    private val publicKey: ByteArray

    init {
        require(privateKey32.size == 32) { "private key must be 32 bytes, got ${privateKey32.size}" }
        require(!Secp256k1Math.isZero(d) && !Secp256k1Math.gte(d, Secp256k1Math.N.limbs)) {
            "private key out of range"
        }
        val (gx, gy) = Secp256k1Math.toAffine(Secp256k1Math.mulG(d))
        publicKey = Secp256k1Math.compressedPubKey(gx, gy)
    }

    override fun publicKeyCompressed(): ByteArray = publicKey.copyOf()

    override fun signDigest(digest32: ByteArray): ByteArray {
        require(digest32.size == 32) { "digest must be 32 bytes, got ${digest32.size}" }
        val z = bits2int(digest32)

        var attempt = 0
        var rngV = ByteArray(32) { 0x01 }
        var rngK = ByteArray(32)
        // RFC6969 §3.2：K = HMAC(K, V||0x00||int2octets(x)||bits2octets(h1)) ...
        val material = dBytes() + bits2octets(digest32)
        rngK = hmac(rngK, rngV + byteArrayOf(0x00) + material)
        rngV = hmac(rngK, rngV)
        rngK = hmac(rngK, rngV + byteArrayOf(0x01) + material)
        rngV = hmac(rngK, rngV)

        while (true) {
            attempt++
            check(attempt <= 1000) { "RFC6979 nonce generation did not converge" }
            // 与 btcec / BouncyCastle 一致：先推进 V 再取候选（第一个候选是 HMAC(K,V0)，不是 V0）
            rngV = hmac(rngK, rngV)
            val t = Secp256k1Math.fromBytesBE(rngV)
            val k = modN(t)
            if (!Secp256k1Math.isZero(k)) {
                val (rx, _) = Secp256k1Math.toAffine(Secp256k1Math.mulG(k))
                val r = modN(rx)
                if (!Secp256k1Math.isZero(r)) {
                    // s = k^-1 (z + r*d) mod n
                    val rd = Secp256k1Math.mulMod(r, d, Secp256k1Math.N)
                    val sum = Secp256k1Math.addMod(rd, z, Secp256k1Math.N)
                    var s = Secp256k1Math.mulMod(Secp256k1Math.invMod(k, Secp256k1Math.N), sum, Secp256k1Math.N)
                    // low-S 规范化（与 SDK / BC 实现一致）
                    if (Secp256k1Math.gte(s, Secp256k1Math.N_HALF)) {
                        s = Secp256k1Math.subMod(Secp256k1Math.N.limbs, s, Secp256k1Math.N)
                    }
                    if (!Secp256k1Math.isZero(s)) {
                        return Secp256k1Math.toBytesBE(r) + Secp256k1Math.toBytesBE(s)
                    }
                }
            }
        }
    }

    private fun dBytes(): ByteArray = Secp256k1Math.toBytesBE(d)

    /** bits2int：大端 256 位 → 整数；≥ n 时减 n（RFC6979 对 extra bits=0 的等价形式）。 */
    private fun bits2int(bytes: ByteArray): IntArray = modN(Secp256k1Math.fromBytesBE(bytes))

    /** bits2octets：bits2int 再回到 32 字节大端。 */
    private fun bits2octets(bytes: ByteArray): ByteArray =
        Secp256k1Math.toBytesBE(bits2int(bytes))

    private fun modN(a: IntArray): IntArray {
        val r = a.copyOf()
        while (Secp256k1Math.gte(r, Secp256k1Math.N.limbs)) Secp256k1Math.subAssign(r, Secp256k1Math.N.limbs)
        return r
    }

    private companion object {
        /** HMAC-SHA256（RFC 2104），块长固定 64 字节。 */
        fun hmac(key: ByteArray, data: ByteArray): ByteArray {
            var k = key
            if (k.size > 64) k = Sha256.digest(k)
            val padded = k.copyOf(64)
            val ipad = ByteArray(64) { (padded[it].toInt() xor 0x36).toByte() }
            val opad = ByteArray(64) { (padded[it].toInt() xor 0x5c).toByte() }
            return Sha256.digest(opad + Sha256.digest(ipad + data))
        }
    }
}
