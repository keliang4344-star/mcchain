package com.mcchain.miner.shared.wallet

import com.mcchain.miner.shared.crypto.PureSecp256k1Signer
import com.mcchain.miner.shared.crypto.TxSigner
import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Ripemd160
import com.mcchain.miner.shared.util.Sha256

/**
 * 链上钱包：私钥 + 压缩公钥 + bech32 地址。
 *
 * 与 Android 端 `WalletDerivation` 完全同构（BIP-39 → BIP-44 118' → secp256k1 → hash160 → bech32），
 * 区别只是实现放在 commonMain，iOS/Android 共用，避免两端地址推导不一致导致「同一助记词两个地址」。
 */
class McWallet private constructor(
    val privateKey: ByteArray,
    val publicKey: ByteArray,
    val address: String,
    val derivationPath: String,
) {

    /** 交易签名器（纯 Kotlin secp256k1，RFC6979 + low-S）。 */
    fun signer(): TxSigner = PureSecp256k1Signer(privateKey)

    fun privateKeyHex(): String = Hex.encode(privateKey)

    override fun toString(): String = "McWallet($address)"

    companion object {
        const val DEFAULT_BECH32_PREFIX = "mc"
        const val DEFAULT_HD_PATH = "m/44'/118'/0'/0/0"
        const val BIP44_COIN_TYPE = 118

        /** 生成新钱包（熵由调用方提供，移动端用系统 CSPRNG：SecRandomCopyBytes / SecureRandom）。 */
        fun fromEntropy(entropy: ByteArray, bech32Prefix: String = DEFAULT_BECH32_PREFIX): McWallet =
            fromMnemonic(Bip39.fromEntropy(entropy), bech32Prefix = bech32Prefix)

        fun fromMnemonic(
            mnemonic: String,
            accountIndex: Int = 0,
            passphrase: String = "",
            bech32Prefix: String = DEFAULT_BECH32_PREFIX,
        ): McWallet {
            require(accountIndex >= 0) { "账户索引不能为负" }
            val seed = Bip39.toSeed(mnemonic, passphrase)
            val path = "m/44'/$BIP44_COIN_TYPE'/0'/0/$accountIndex"
            val node = Bip32.derivePath(seed, path)
            val publicKey = Bip32.publicKeyOf(node.privateKey)
            return McWallet(node.privateKey, publicKey, addressOf(publicKey, bech32Prefix), path)
        }

        fun fromPrivateKey(
            privateKey: ByteArray,
            bech32Prefix: String = DEFAULT_BECH32_PREFIX,
        ): McWallet {
            require(privateKey.size == 32) { "私钥必须是 32 字节，当前 ${privateKey.size}" }
            val publicKey = Bip32.publicKeyOf(privateKey)
            return McWallet(privateKey.copyOf(), publicKey, addressOf(publicKey, bech32Prefix), "imported")
        }

        fun fromPrivateKeyHex(hex: String, bech32Prefix: String = DEFAULT_BECH32_PREFIX): McWallet =
            fromPrivateKey(Hex.decode(hex.trim().removePrefix("0x")), bech32Prefix)

        fun isValidMnemonic(mnemonic: String): Boolean = Bip39.isValidMnemonic(mnemonic)

        fun createMnemonic(entropy: ByteArray): String = Bip39.fromEntropy(entropy)

        /** Cosmos 地址：bech32(prefix, RIPEMD160(SHA256(压缩公钥)))。 */
        fun addressOf(publicKey: ByteArray, bech32Prefix: String = DEFAULT_BECH32_PREFIX): String {
            val hash160 = Ripemd160.digest(Sha256.digest(publicKey))
            return Bech32.encode(bech32Prefix, Bech32.convertBits(hash160, 8, 5, true))
        }
    }
}
