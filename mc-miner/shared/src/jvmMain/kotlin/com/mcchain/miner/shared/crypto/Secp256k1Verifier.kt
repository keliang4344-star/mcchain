package com.mcchain.miner.shared.crypto

import org.bouncycastle.crypto.ec.CustomNamedCurves
import org.bouncycastle.crypto.params.ECDomainParameters
import org.bouncycastle.crypto.params.ECPublicKeyParameters
import org.bouncycastle.crypto.signers.ECDSASigner
import java.math.BigInteger

/**
 * secp256k1 验签（64 字节 r||s，digest 与 Cosmos SDK `PubKey.VerifySignature` 一致）。
 *
 * 用于两步：
 *  - App 校验预言机 /sign 返回的签名，避免中间人替换认证签名；
 *  - 单元测试对 [Secp256k1Signer] 的签名做自校验。
 */
object Secp256k1Verifier {
    private val domain: ECDomainParameters = CustomNamedCurves.getByName("secp256k1").let {
        ECDomainParameters(it.curve, it.g, it.n, it.h)
    }

    fun verify(publicKeyCompressed: ByteArray, digest32: ByteArray, signature64: ByteArray): Boolean {
        if (publicKeyCompressed.size != 33) return false
        if (digest32.size != 32) return false
        if (signature64.size != 64) return false

        val point = runCatching { domain.curve.decodePoint(publicKeyCompressed) }.getOrNull() ?: return false
        val params = ECPublicKeyParameters(point, domain)
        val r = BigInteger(1, signature64.copyOfRange(0, 32))
        val s = BigInteger(1, signature64.copyOfRange(32, 64))
        val signer = ECDSASigner()
        signer.init(false, params)
        return signer.verifySignature(digest32, r, s)
    }
}
