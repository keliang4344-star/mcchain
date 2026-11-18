package com.mcchain.miner.domain.wallet

import com.mcchain.miner.MCParams
import org.bitcoinj.crypto.ChildNumber
import org.bitcoinj.crypto.DeterministicKey
import org.bitcoinj.crypto.HDKeyDerivation
import org.bitcoinj.crypto.MnemonicCode
import org.bitcoinj.wallet.DeterministicSeed
import org.bouncycastle.crypto.digests.RIPEMD160Digest
import org.bouncycastle.jce.ECNamedCurveTable
import java.math.BigInteger

/**
 * 纯函数式的 BIP-39 / BIP-44 密钥派生，不依赖 Android 存储，便于单元测试。
 *
 * 路径固定为 Cosmos 标准：m/44'/118'/0'/0/{index}。
 */
object WalletDerivation {

    fun isValidMnemonic(mnemonic: List<String>): Boolean {
        if (mnemonic.size !in setOf(12, 15, 18, 21, 24)) return false
        return try {
            MnemonicCode.INSTANCE.check(mnemonic)
            true
        } catch (e: Exception) {
            false
        }
    }

    fun derive(mnemonic: List<String>, index: Int = 0): DerivedWallet {
        require(isValidMnemonic(mnemonic)) { "助记词无效" }
        require(index >= 0) { "账户索引不能为负" }

        val seed = DeterministicSeed(mnemonic, null, "", 0L)
        val seedBytes = seed.seedBytes ?: error("无法从助记词派生种子")
        val masterKey = HDKeyDerivation.createMasterPrivateKey(seedBytes)

        val path = intArrayOf(
            MCParams.BIP44_PURPOSE or ChildNumber.HARDENED_BIT,
            MCParams.BIP44_COIN_TYPE or ChildNumber.HARDENED_BIT,
            CHAIN_INDEX or ChildNumber.HARDENED_BIT,
            CHANGE_INDEX,
            index,
        )
        var key: DeterministicKey = masterKey
        for (child in path) {
            key = HDKeyDerivation.deriveChildKey(key, child)
        }

        val privateKey = key.privKeyBytes ?: error("派生私钥失败")
        require(privateKey.size == 32) { "私钥长度异常：${privateKey.size}" }

        val d = BigInteger(1, privateKey)
        val curve = ECNamedCurveTable.getParameterSpec("secp256k1")
        val publicKey = curve.g.multiply(d).normalize().getEncoded(true)

        return DerivedWallet(
            privateKey = privateKey.copyOf(),
            publicKey = publicKey,
            address = publicKeyToAddress(publicKey),
        )
    }

    /** Cosmos 地址：SHA-256(压缩公钥) -> RIPEMD-160 -> bech32("mc")。 */
    fun publicKeyToAddress(publicKey: ByteArray): String {
        val sha256 = java.security.MessageDigest.getInstance("SHA-256").digest(publicKey)
        val ripeMd160 = ByteArray(20)
        val digest = RIPEMD160Digest()
        digest.update(sha256, 0, sha256.size)
        digest.doFinal(ripeMd160, 0)
        return bech32Encode(MCParams.BECH32_PREFIX, convertBits(ripeMd160, 8, 5, true))
    }

    private const val CHAIN_INDEX = 0
    private const val CHANGE_INDEX = 0
    private const val CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

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
