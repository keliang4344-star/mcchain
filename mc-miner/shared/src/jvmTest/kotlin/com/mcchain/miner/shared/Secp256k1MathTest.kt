package com.mcchain.miner.shared

import com.mcchain.miner.shared.crypto.PureSecp256k1Signer
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.crypto.Secp256k1Math
import com.mcchain.miner.shared.util.Sha256
import org.bouncycastle.crypto.ec.CustomNamedCurves
import org.junit.Assert
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.math.BigInteger
import java.security.SecureRandom

/**
 * 纯 Kotlin secp256k1 数学与签名器的正确性锚点：
 *  1. 域/标量运算对照 java.math.BigInteger；
 *  2. 生成元与曲线点运算对照 BouncyCastle；
 *  3. RFC6979 签名与 Secp256k1Signer（BC 实现）逐字节一致。
 */
class Secp256k1MathTest {

    private val pBig = BigInteger(1, Secp256k1Math.toBytesBE(Secp256k1Math.P.limbs))
    private val nBig = BigInteger(1, Secp256k1Math.toBytesBE(Secp256k1Math.N.limbs))
    private val random = SecureRandom()

    private fun big(v: IntArray): BigInteger = BigInteger(1, Secp256k1Math.toBytesBE(v))
    private fun limbs(v: BigInteger): IntArray {
        val raw = v.toByteArray()
        val be = ByteArray(32)
        // 取大端表示的低 32 字节（BigInteger 可能带符号零字节或更短）
        val src = if (raw.size >= 32) raw.copyOfRange(raw.size - 32, raw.size)
            else ByteArray(32 - raw.size) + raw
        src.copyInto(be)
        return Secp256k1Math.fromBytesBE(be)
    }

    @Test
    fun constantsMatchKnownValues() {
        // p = 2^256 - 2^32 - 977
        val expectedP = BigInteger.TWO.pow(256) - BigInteger.TWO.pow(32) - BigInteger.valueOf(977)
        assertEquals("p", expectedP, pBig)
        // n（secp256k1 阶标准值）
        val expectedN = BigInteger("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
        assertEquals("n", expectedN, nBig)
        // G 坐标对照 BouncyCastle 权威生成元
        val curve = CustomNamedCurves.getByName("secp256k1")
        val g = curve.g.normalize()
        assertEquals("Gx", g.xCoord.toBigInteger(), big(Secp256k1Math.GEN_X))
        assertEquals("Gy", g.yCoord.toBigInteger(), big(Secp256k1Math.GEN_Y))
        // G 在曲线上：y^2 ≡ x^3 + 7 (mod p)（secp256k1 b=7）
        val gx = g.xCoord.toBigInteger()
        val gy = g.yCoord.toBigInteger()
        val lhs = gy.modPow(BigInteger.TWO, pBig)
        val rhs = (gx.modPow(BigInteger.valueOf(3), pBig) + BigInteger.valueOf(7)).mod(pBig)
        assertEquals("G on curve", lhs, rhs)
    }

    @Test
    fun fieldArithmeticMatchesBigInteger() {
        repeat(2000) {
            val a = limbs(BigInteger(256, random).mod(pBig))
            val b = limbs(BigInteger(256, random).mod(pBig))
            val aB = big(a)
            val bB = big(b)
            assertEquals("addMod", aB.add(bB).mod(pBig), big(Secp256k1Math.addMod(a, b, Secp256k1Math.P)))
            assertEquals("subMod", aB.subtract(bB).mod(pBig), big(Secp256k1Math.subMod(a, b, Secp256k1Math.P)))
            assertEquals("mulMod", aB.multiply(bB).mod(pBig), big(Secp256k1Math.mulMod(a, b, Secp256k1Math.P)))
        }
    }

    @Test
    fun scalarArithmeticMatchesBigInteger() {
        repeat(2000) {
            val a = limbs(BigInteger(256, random).mod(nBig))
            val b = limbs(BigInteger(256, random).mod(nBig))
            val aB = big(a)
            val bB = big(b)
            assertEquals("addMod n", aB.add(bB).mod(nBig), big(Secp256k1Math.addMod(a, b, Secp256k1Math.N)))
            assertEquals("mulMod n", aB.multiply(bB).mod(nBig), big(Secp256k1Math.mulMod(a, b, Secp256k1Math.N)))
        }
    }

    @Test
    fun inverseMatchesBigInteger() {
        repeat(500) {
            val a = limbs(BigInteger(255, random).mod(pBig - BigInteger.ONE) + BigInteger.ONE)
            val inv = big(Secp256k1Math.invMod(a, Secp256k1Math.P))
            assertEquals("a * a^-1 = 1 (mod p)", BigInteger.ONE, big(a).multiply(inv).mod(pBig))
        }
    }

    @Test
    fun generatorMultiplyMatchesBouncyCastle() {
        val curve = CustomNamedCurves.getByName("secp256k1")
        repeat(20) {
            val k = BigInteger(255, random) + BigInteger.ONE
            val expected = curve.g.multiply(k).normalize().getEncoded(true)
            val (x, y) = Secp256k1Math.toAffine(Secp256k1Math.mulG(limbs(k)))
            assertArrayEquals("kG", expected, Secp256k1Math.compressedPubKey(x, y))
        }
    }

    @Test
    fun signerMatchesBouncyCastleByteForByte() {
        repeat(10) {
            val priv = ByteArray(32).also(random::nextBytes)
            val pure = PureSecp256k1Signer(priv)
            val bc = Secp256k1Signer(priv)
            assertArrayEquals("publicKey", bc.publicKeyCompressed(), pure.publicKeyCompressed())
            repeat(10) {
                val digest = Sha256.digest("msg-$it-${random.nextInt()}".toByteArray())
                val bcSig = bc.signDigest(digest)
                val pureSig = pure.signDigest(digest)
                val rMatch = bcSig.copyOfRange(0, 32).contentEquals(pureSig.copyOfRange(0, 32))
                val sMatch = bcSig.copyOfRange(32, 64).contentEquals(pureSig.copyOfRange(32, 64))
                if (!rMatch || !sMatch) {
                    val rB = java.math.BigInteger(1, pureSig.copyOfRange(0, 32))
                    val sB = java.math.BigInteger(1, pureSig.copyOfRange(32, 64))
                    // 用 BC 验签兜底：纯实现签名必须数学有效（只是 nonce 与 BC 不同）
                    val valid = com.mcchain.miner.shared.crypto.Secp256k1Verifier.verify(
                        pure.publicKeyCompressed(), digest, pureSig,
                    )
                    assertTrue(
                        "pure 签名必须可被验证 (rMatch=$rMatch sMatch=$sMatch r=$rB)",
                        valid,
                    )
                    Assert.fail("与 BC 不一致: rMatch=$rMatch sMatch=$sMatch")
                }
            }
        }
    }

    @Test
    fun privateKeyRangeValidation() {
        // d = n-1 合法；d = n 拒绝
        PureSecp256k1Signer(Secp256k1Math.toBytesBE(Secp256k1Math.subMod(Secp256k1Math.N.limbs, limbs(BigInteger.ONE), Secp256k1Math.N)))
        val rejected = runCatching { PureSecp256k1Signer(Secp256k1Math.toBytesBE(Secp256k1Math.N.limbs)) }
        assertTrue("d = n 必须被拒绝", rejected.isFailure)
    }
}
