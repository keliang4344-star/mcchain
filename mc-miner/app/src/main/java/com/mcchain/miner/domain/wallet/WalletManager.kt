package com.mcchain.miner.domain.wallet

import android.util.Base64
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.pref.SecurePrefs
import org.bitcoinj.crypto.*
import org.bitcoinj.wallet.DeterministicSeed
import org.bouncycastle.crypto.digests.RIPEMD160Digest
import org.bouncycastle.crypto.digests.SHA256Digest
import org.bouncycastle.jce.ECNamedCurveTable
import org.bouncycastle.jce.provider.BouncyCastleProvider
import org.bouncycastle.jce.spec.ECParameterSpec
import java.math.BigInteger
import java.security.*
import java.security.spec.ECGenParameterSpec
import javax.inject.Inject
import javax.inject.Singleton

/**
 * BIP-39 助记词生成与 BIP-32/44 密钥派生。
 * 基于 bitcoinj + BouncyCastle，与 Cosmos SDK secp256k1 兼容。
 */
@Singleton
class WalletManager @Inject constructor(
    private val securePrefs: SecurePrefs
) {
    init {
        Security.insertProviderAt(BouncyCastleProvider(), 1)
    }

    /**
     * 生成 24 词 BIP-39 助记词
     */
    fun generateMnemonic(): List<String> {
        val seed = DeterministicSeed(SecureRandom(), 256, "")
        return seed.mnemonicCode
    }

    /**
     * 通过助记词恢复钱包，派生第一个账户的密钥对
     */
    fun recoverFromMnemonic(mnemonic: List<String>): KeyPair {
        val seed = DeterministicSeed(mnemonic, null, "", System.currentTimeMillis())
        val masterKey = HDUtils.createMasterKey(seed.seedBytes!!)
        return deriveKeyPair(masterKey, 0)
    }

    /**
     * 按 Cosmos 标准 BIP-44 路径派生: m/44'/118'/0'/0/index
     */
    private fun deriveKeyPair(masterKey: DeterministicKey, index: Int): KeyPair {
        val purpose = HDUtils.deriveChildKey(masterKey, 44 or ChildNumber.HARDENED_BIT)
        val coinType = HDUtils.deriveChildKey(purpose, MCParams.BIP44_COIN_TYPE or ChildNumber.HARDENED_BIT)
        val account = HDUtils.deriveChildKey(coinType, 0 or ChildNumber.HARDENED_BIT)
        val change = HDUtils.deriveChildKey(account, 0)
        val child = HDUtils.deriveChildKey(change, index)

        val privKeyBytes = child.privKeyBytes
        val privKey = BigInteger(1, privKeyBytes)

        // 从私钥恢复公钥 (secp256k1)
        val spec = ECNamedCurveTable.getParameterSpec("secp256k1")
        val keyFactory = KeyFactory.getInstance("ECDSA", "BC")
        val privKeySpec = java.security.spec.ECPrivateKeySpec(privKey, spec)
        val privateKey = keyFactory.generatePrivate(privKeySpec) as java.security.interfaces.ECPrivateKey

        // 计算公钥
        val q = spec.g.multiply(privKey).normalize()
        val publicKey = keyFactory.generatePublic(
            java.security.spec.ECPublicKeySpec(
                java.security.spec.ECPoint(q.affineXCoord.toBigInteger(), q.affineYCoord.toBigInteger()),
                spec
            )
        ) as java.security.interfaces.ECPublicKey

        return KeyPair(publicKey, privateKey)
    }

    /**
     * 从公钥生成 bech32 "mc" 地址
     */
    fun publicKeyToAddress(pubKeyBytes: ByteArray): String {
        // SHA256 -> RIPEMD160
        val sha256 = Sha256Hash.hash(pubKeyBytes)
        val ripeMd160 = ByteArray(20)
        val digest = RIPEMD160Digest()
        digest.update(sha256, 0, sha256.size)
        digest.doFinal(ripeMd160, 0)

        // Bech32 编码 (mc 前缀)
        return bech32Encode(MCParams.BECH32_PREFIX, convertBits(ripeMd160, 8, 5, true))
    }

    /**
     * 使用私钥签名数据 (Cosmos StdTx 兼容)
     */
    fun signWithPrivateKey(privateKey: PrivateKey, data: ByteArray): ByteArray {
        val signature = Signature.getInstance("SHA256withECDSA", "BC")
        signature.initSign(privateKey)
        signature.update(data)
        return signature.sign()
    }

    /**
     * 使用公钥验证签名
     */
    fun verifySignature(publicKey: PublicKey, data: ByteArray, signature: ByteArray): Boolean {
        val sig = Signature.getInstance("SHA256withECDSA", "BC")
        sig.initVerify(publicKey)
        sig.update(data)
        return sig.verify(signature)
    }

    /**
     * 加密并存储助记词
     */
    fun storeEncryptedMnemonic(mnemonic: List<String>) {
        securePrefs.encryptedMnemonic = mnemonic.joinToString(" ")
    }

    /**
     * 读取并解密助记词
     */
    fun getStoredMnemonic(): List<String>? {
        return securePrefs.encryptedMnemonic?.split(" ")
    }

    /**
     * 获取当前默认账户的地址
     */
    fun getDefaultAddress(): String? {
        val mnemonic = getStoredMnemonic() ?: return null
        val keyPair = recoverFromMnemonic(mnemonic)
        val pubKeyEncoded = (keyPair.public as java.security.interfaces.ECPublicKey)
            .w.affineX.toByteArray() + (keyPair.public as java.security.interfaces.ECPublicKey)
            .w.affineY.toByteArray()
        return publicKeyToAddress(pubKeyEncoded)
    }

    // === Bech32 编码 ===
    private val CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

    private fun bech32Encode(hrp: String, data: ByteArray): String {
        val combined = data + createChecksum(hrp, data)
        return hrp + "1" + combined.map { CHARSET[it.toInt()] }.joinToString("")
    }

    private fun createChecksum(hrp: String, data: ByteArray): ByteArray {
        val values = hrpExpand(hrp) + data.toList().map { it.toInt() } + listOf(0, 0, 0, 0, 0, 0)
        val mod = polymod(values) xor 1
        val checksum = ByteArray(6)
        for (i in 0 until 6) {
            checksum[i] = ((mod shr (5 * (5 - i))) and 31).toByte()
        }
        return checksum
    }

    private fun hrpExpand(hrp: String): List<Int> {
        val result = mutableListOf<Int>()
        for (c in hrp) {
            result.add(c.code shr 5)
        }
        result.add(0)
        for (c in hrp) {
            result.add(c.code and 31)
        }
        return result
    }

    private fun polymod(values: List<Int>): Int {
        val generator = intArrayOf(0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3)
        var chk = 1
        for (value in values) {
            val top = chk shr 25
            chk = ((chk and 0x1ffffff) shl 5) xor value
            for (i in 0 until 5) {
                if (((top shr i) and 1) != 0) {
                    chk = chk xor generator[i]
                }
            }
        }
        return chk
    }

    private fun convertBits(data: ByteArray, fromBits: Int, toBits: Int, pad: Boolean): ByteArray {
        var acc = 0
        var bits = 0
        val result = mutableListOf<Byte>()
        val maxv = (1 shl toBits) - 1
        for (value in data) {
            val b = value.toInt() and 0xFF
            if (b shr fromBits != 0) throw IllegalArgumentException("Invalid data")
            acc = (acc shl fromBits) or b
            bits += fromBits
            while (bits >= toBits) {
                bits -= toBits
                result.add(((acc shr bits) and maxv).toByte())
            }
        }
        if (pad) {
            if (bits > 0) result.add(((acc shl (toBits - bits)) and maxv).toByte())
        } else if (bits >= fromBits || (acc shl (toBits - bits)) and maxv != 0) {
            throw IllegalArgumentException("Invalid padding")
        }
        return result.toByteArray()
    }
}
