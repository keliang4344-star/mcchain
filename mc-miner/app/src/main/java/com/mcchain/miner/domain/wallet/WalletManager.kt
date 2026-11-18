package com.mcchain.miner.domain.wallet

import com.mcchain.miner.data.pref.SecurePrefs
import org.bitcoinj.wallet.DeterministicSeed
import java.security.SecureRandom
import javax.inject.Inject
import javax.inject.Singleton

/** 从助记词派生出的账户材料；私钥只在内存中出现，不落盘。 */
data class DerivedWallet(
    val privateKey: ByteArray,
    val publicKey: ByteArray,
    val address: String,
)

/**
 * 钱包存储与派生入口。
 *
 * 私钥/助记词只允许存放在 EncryptedSharedPreferences；SecurePrefs 不可用时直接抛错，
 * 由 UI 提示用户，绝不回退明文。派生逻辑见 [WalletDerivation]（纯函数，可单测）。
 */
@Singleton
class WalletManager @Inject constructor(
    private val securePrefs: SecurePrefs
) {
    val isSecureStorageAvailable: Boolean
        get() = securePrefs.isSecureStorageAvailable

    fun hasStoredWallet(): Boolean =
        try {
            !securePrefs.encryptedMnemonic.isNullOrBlank()
        } catch (e: Exception) {
            false
        }

    /** 生成 24 词 BIP-39 助记词。 */
    fun generateMnemonic(): List<String> {
        val seed = DeterministicSeed(SecureRandom(), 256, "")
        return seed.mnemonicCode ?: emptyList()
    }

    fun isValidMnemonic(mnemonic: List<String>): Boolean =
        WalletDerivation.isValidMnemonic(mnemonic)

    fun storeMnemonic(mnemonic: List<String>) {
        require(isValidMnemonic(mnemonic)) { "助记词无效" }
        securePrefs.encryptedMnemonic = mnemonic.joinToString(" ")
    }

    fun getStoredMnemonic(): List<String>? {
        val raw = try {
            securePrefs.encryptedMnemonic
        } catch (e: Exception) {
            null
        } ?: return null
        return raw.trim().split(Regex("\\s+")).filter { it.isNotBlank() }.takeIf { it.isNotEmpty() }
    }

    fun clearWallet() {
        securePrefs.encryptedMnemonic = null
    }

    fun deriveWallet(
        mnemonic: List<String> = getStoredMnemonic() ?: error("钱包未初始化"),
        index: Int = 0,
    ): DerivedWallet = WalletDerivation.derive(mnemonic, index)

    fun getStoredWallet(): DerivedWallet? =
        if (!hasStoredWallet()) null else runCatching { deriveWallet() }.getOrNull()

    fun getDefaultAddress(): String? = getStoredWallet()?.address
}
