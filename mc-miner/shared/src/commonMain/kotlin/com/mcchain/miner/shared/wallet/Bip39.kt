package com.mcchain.miner.shared.wallet

import com.mcchain.miner.shared.util.Sha512
import com.mcchain.miner.shared.util.Sha256

/**
 * BIP-39 助记词（英文词表）。
 *
 * 限制说明：Kotlin common 没有 Unicode NFKD 归一化，因此只支持英文助记词与 ASCII passphrase；
 * 这与移动端产品形态一致（钱包只生成/导入英文助记词），非 ASCII 输入直接拒绝而不是静默算错种子。
 */
object Bip39 {

    private val VALID_WORD_COUNTS = setOf(12, 15, 18, 21, 24)
    private const val PBKDF2_ROUNDS = 2048
    private const val SEED_LENGTH = 64

    /** 生成助记词：熵长度必须是 16/20/24/28/32 字节（对应 12/15/18/21/24 词）。 */
    fun fromEntropy(entropy: ByteArray): String {
        require(entropy.size in 16..32 && entropy.size % 4 == 0) {
            "熵长度无效：${entropy.size}（需为 16/20/24/28/32 字节）"
        }
        val checksumBits = entropy.size / 4
        val checksumByte = Sha256.digest(entropy)[0].toInt() and 0xFF
        val bitCount = entropy.size * 8
        val total = bitCount + checksumBits
        val words = ArrayList<String>(total / 11)
        var acc = 0
        var bits = 0
        for (i in 0 until total) {
            val bit = if (i < bitCount) {
                (entropy[i / 8].toInt() ushr (7 - i % 8)) and 1
            } else {
                (checksumByte ushr (7 - (i - bitCount))) and 1
            }
            acc = (acc shl 1) or bit
            bits++
            if (bits == 11) {
                words.add(Bip39Wordlist.words[acc])
                acc = 0
                bits = 0
            }
        }
        return words.joinToString(" ")
    }

    fun isValidMnemonic(mnemonic: String): Boolean = entropyOf(normalize(mnemonic)) != null

    /** 助记词 → 64 字节种子（BIP-39 标准 PBKDF2-HMAC-SHA512，salt = "mnemonic" + passphrase）。 */
    fun toSeed(mnemonic: String, passphrase: String = ""): ByteArray {
        val normalized = normalize(mnemonic)
        require(entropyOf(normalized) != null) { "助记词无效（词表或校验和错误）" }
        require(passphrase.all { it.code < 128 }) { "暂不支持非 ASCII passphrase" }
        return Sha512.pbkdf2(
            password = (normalized).encodeToByteArray(),
            salt = ("mnemonic" + passphrase).encodeToByteArray(),
            iterations = PBKDF2_ROUNDS,
            dkLen = SEED_LENGTH,
        )
    }

    fun normalize(mnemonic: String): String =
        mnemonic.trim().split(Regex("\\s+")).filter { it.isNotEmpty() }.joinToString(" ")

    /** 校验并还原熵；不合法返回 null。 */
    fun entropyOf(normalizedMnemonic: String): ByteArray? {
        val words = normalizedMnemonic.split(' ')
        if (words.size !in VALID_WORD_COUNTS) return null
        val bitCount = words.size * 11
        val entropyBits = bitCount * 32 / 33
        val checksumBits = bitCount - entropyBits

        val indices = IntArray(words.size) {
            val idx = Bip39Wordlist.indexOf(words[it])
            if (idx < 0) return null
            idx
        }
        val entropy = ByteArray(entropyBits / 8)
        for (i in 0 until entropyBits) {
            val wordIndex = i / 11
            val shift = 10 - (i % 11)
            val bit = (indices[wordIndex] ushr shift) and 1
            if (bit == 1) {
                val pos = i / 8
                entropy[pos] = (entropy[pos].toInt() or (1 shl (7 - i % 8))).toByte()
            }
        }
        val expected = Sha256.digest(entropy)[0].toInt() and 0xFF
        var actual = 0
        for (i in 0 until checksumBits) {
            val wordIndex = (entropyBits + i) / 11
            val shift = 10 - ((entropyBits + i) % 11)
            actual = (actual shl 1) or ((indices[wordIndex] ushr shift) and 1)
        }
        val expectedChecksum = (expected ushr (8 - checksumBits)) and ((1 shl checksumBits) - 1)
        return if (actual == expectedChecksum) entropy else null
    }
}
