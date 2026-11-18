package com.mcchain.miner.shared.crypto

import org.bouncycastle.crypto.digests.SHA256Digest
import org.bouncycastle.crypto.ec.CustomNamedCurves
import org.bouncycastle.crypto.params.ECDomainParameters
import org.bouncycastle.crypto.params.ECPrivateKeyParameters
import org.bouncycastle.crypto.signers.ECDSASigner
import org.bouncycastle.crypto.signers.HMacDSAKCalculator
import java.math.BigInteger

/**
 * JVM/Android 侧 secp256k1 签名实现。
 *
 * 与 Cosmos SDK 的 `secp256k1.PrivKey.Sign` 对齐：
 *  - RFC6979（HMAC-SHA256）确定性 nonce
 *  - low-S 规范化
 *  - 输出 64 字节 r||s
 */
class Secp256k1Signer(privateKey32: ByteArray) : TxSigner {
    private val domain: ECDomainParameters
    private val privateValue: BigInteger
    private val publicKey: ByteArray

    init {
        require(privateKey32.size == 32) { "private key must be 32 bytes, got ${privateKey32.size}" }
        val curve = CustomNamedCurves.getByName("secp256k1")
        domain = ECDomainParameters(curve.curve, curve.g, curve.n, curve.h)
        privateValue = BigInteger(1, privateKey32)
        require(privateValue.signum() > 0 && privateValue < curve.n) {
            "private key out of secp256k1 range"
        }
        publicKey = curve.g.multiply(privateValue).normalize().getEncoded(true)
    }

    override fun publicKeyCompressed(): ByteArray = publicKey.copyOf()

    override fun signDigest(digest32: ByteArray): ByteArray {
        require(digest32.size == 32) { "digest must be 32 bytes, got ${digest32.size}" }
        val signer = ECDSASigner(HMacDSAKCalculator(SHA256Digest()))
        signer.init(true, ECPrivateKeyParameters(privateValue, domain))
        val components = signer.generateSignature(digest32)
        val r = components[0]
        var s = components[1]
        val halfN = domain.n.shiftRight(1)
        if (s > halfN) {
            s = domain.n.subtract(s)
        }
        val out = ByteArray(64)
        writeFixed32(r, out, 0)
        writeFixed32(s, out, 32)
        return out
    }

    private fun writeFixed32(value: BigInteger, out: ByteArray, offset: Int) {
        val raw = value.toByteArray()
        // BigInteger 为正数且最高位为 1 时会多出一个 0x00 符号位，需要剥掉。
        val start = if (raw.size == 33 && raw[0] == 0.toByte()) 1 else 0
        val length = raw.size - start
        require(length <= 32) { "value too large for 32 bytes" }
        raw.copyInto(out, offset + (32 - length), start, raw.size)
    }
}
