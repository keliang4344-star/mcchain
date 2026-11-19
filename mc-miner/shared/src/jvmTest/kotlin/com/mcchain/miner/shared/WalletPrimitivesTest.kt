package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Ripemd160
import com.mcchain.miner.shared.util.Sha512
import com.mcchain.miner.shared.wallet.Bip32
import com.mcchain.miner.shared.wallet.Bip39
import com.mcchain.miner.shared.wallet.McWallet
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 共享钱包原语测试（SHA-512 / RIPEMD-160 / BIP-39 / BIP-32 / 地址派生）。
 *
 * 向量来源：
 *  - SHA-512 / HMAC-SHA-512：NIST FIPS 180-4、RFC 4231；
 *  - PBKDF2-HMAC-SHA512：RFC 7914 附录 B 派生用例；
 *  - BIP-39：官方测试向量（TREZOR passphrase）；
 *  - BIP-32：BIP-32 官方 Test Vector 1；
 *  - 地址：链上 golden 交易（mcchaind 离线签名）里的私钥/公钥/地址三元组。
 */
class WalletPrimitivesTest {

    private fun hex(s: String) = Hex.decode(s)

    // ---------------------------------------------------------------- SHA-512

    @Test
    fun sha512MatchesNistVectors() {
        assertEquals(
            "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce" +
                "47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
            Hex.encode(Sha512.digest(ByteArray(0))),
        )
        assertEquals(
            "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
                "2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
            Hex.encode(Sha512.digest("abc".encodeToByteArray())),
        )
        // 112 字节：跨两个 block，验证长度补齐
        assertEquals(
            "8e959b75dae313da8cf4f72814fc143f8f7779c6eb9f7fa17299aeadb6889018" +
                "501d289e4900f7e4331b99dec4b5433ac7d329eeb6dd26545e96e55b874be909",
            Hex.encode(
                Sha512.digest(
                    (
                        "abcdefghbcdefghicdefghijdefghijkefghijklfghijklmghijklmnhijklmno" +
                            "ijklmnopjklmnopqklmnopqrlmnopqrsmnopqrstnopqrstu"
                        ).encodeToByteArray(),
                ),
            ),
        )
    }

    @Test
    fun hmacSha512MatchesRfc4231() {
        val key = ByteArray(20) { 0x0b }
        assertEquals(
            "87aa7cdea5ef619d4ff0b4241a1d6cb02379f4e2ce4ec2787ad0b30545e17cde" +
                "daa833b7d6b8a702038b274eaea3f4e4be9d914eeb61f1702e696c203a126854",
            Hex.encode(Sha512.hmac(key, "Hi There".encodeToByteArray())),
        )
    }

    @Test
    fun pbkdf2MatchesRfc7914() {
        val out = Sha512.pbkdf2("password".encodeToByteArray(), "salt".encodeToByteArray(), 1, 64)
        assertEquals(
            "867f70cf1ade02cff3752599a3a53dc4af34c7a669815ae5d513554e1c8cf252" +
                "c02d470a285a0501bad999bfe943c08f050235d7d68b1da55e63f73b60a57fce",
            Hex.encode(out),
        )
    }

    // ------------------------------------------------------------ RIPEMD-160

    @Test
    fun ripemd160MatchesReferenceVectors() {
        assertEquals("9c1185a5c5e9fc54612808977ee8f548b2258d31", Hex.encode(Ripemd160.digest(ByteArray(0))))
        assertEquals("8eb208f7e05d987a9b044a8e98c6b087f15a0bfc", Hex.encode(Ripemd160.digest("abc".encodeToByteArray())))
        assertEquals(
            "5d0689ef49d2fae572b881b123a85ffa21595f36",
            Hex.encode(Ripemd160.digest("message digest".encodeToByteArray())),
        )
        assertEquals(
            "12a053384a9c0c88e405a06c27dcf49ada62eb2b",
            Hex.encode(
                Ripemd160.digest(
                    "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq".encodeToByteArray(),
                ),
            ),
        )
    }

    // ---------------------------------------------------------------- BIP-39

    private val abandonAbout =
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

    @Test
    fun bip39AcceptsKnownMnemonicAndRejectsBadChecksum() {
        assertTrue(Bip39.isValidMnemonic(abandonAbout))
        assertTrue(Bip39.isValidMnemonic("  $abandonAbout  "))
        // 把最后一个词换掉：校验和不再匹配
        assertFalse(Bip39.isValidMnemonic(abandonAbout.replace(Regex("about$"), "abandon")))
        // 词表外单词
        assertFalse(Bip39.isValidMnemonic(abandonAbout.replace(Regex("about$"), "abandonx")))
        // 词数不对
        assertFalse(Bip39.isValidMnemonic("abandon abandon abandon"))
    }

    @Test
    fun bip39SeedMatchesOfficialVector() {
        val seed = Bip39.toSeed(abandonAbout, "TREZOR")
        assertEquals(
            "c55257c360c07c72029aebc1b53c05ed0362ada38ead3e3e9efa3708e5349553" +
                "1f09a6987599d18264c1e1c92f2cf141630c7a3c4ab7c81b2f001698e7463b04",
            Hex.encode(seed),
        )
    }

    @Test
    fun bip39EntropyRoundTrip() {
        for (size in intArrayOf(16, 20, 24, 28, 32)) {
            val entropy = ByteArray(size) { (it * 7 + 3).toByte() }
            val mnemonic = Bip39.fromEntropy(entropy)
            assertTrue("size=$size 生成的助记词应校验通过", Bip39.isValidMnemonic(mnemonic))
            assertArrayEquals("size=$size 熵应可无损还原", entropy, Bip39.entropyOf(mnemonic))
            // 官方零熵向量：12 词全 abandon + about
            if (size == 16) {
                assertEquals(abandonAbout, Bip39.fromEntropy(ByteArray(16)))
            }
        }
    }

    // ---------------------------------------------------------------- BIP-32

    @Test
    fun bip32MatchesOfficialTestVector1() {
        val seed = hex("000102030405060708090a0b0c0d0e0f")
        val master = Bip32.fromSeed(seed)
        assertEquals("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35", Hex.encode(master.privateKey))
        assertEquals("873dff81c02f525623fd1fe5167eac3a55a049de3d314bb42ee227ffed37d508", Hex.encode(master.chainCode))

        val m0h = Bip32.derive(master, 0x80000000L)
        assertEquals("edb2e14f9ee77d26dd93b4ecede8d16ed408ce149b6cd80b0715a2d911a0afea", Hex.encode(m0h.privateKey))
        assertEquals("47fdacbd0f1097043b78c63c20c34ef4ed9a111d980047ad16282c7ae6236141", Hex.encode(m0h.chainCode))

        val m0h1 = Bip32.derive(m0h, 1)
        assertEquals("3c6cb8d0f6a264c91ea8b5030fadaa8e538b020f0a387421a12de9319dc93368", Hex.encode(m0h1.privateKey))
        assertEquals("2a7857631386ba23dacac34180dd1983734e444fdbf774041578e9b6adb37c19", Hex.encode(m0h1.chainCode))

        val m0h1_2h = Bip32.derive(m0h1, 0x80000002L)
        assertEquals("cbce0d719ecf7431d88e6a89fa1483e02e35092af60c042b1df2ff59fa424dca", Hex.encode(m0h1_2h.privateKey))
        assertEquals("04466b9cc8e161e966409ca52986c584f07e9dc81f735db683c3ff6ec7b1503f", Hex.encode(m0h1_2h.chainCode))
        assertEquals("m/0'/1/2'", m0h1_2h.path)
    }

    @Test
    fun bip32ParsesCosmosPath() {
        assertEquals(
            listOf(44L + Bip32.HARDENED_BIT, 118L + Bip32.HARDENED_BIT, Bip32.HARDENED_BIT, 0L, 7L),
            Bip32.parsePath("m/44'/118'/0'/0/7"),
        )
        assertEquals(
            listOf(44L + Bip32.HARDENED_BIT, 118L + Bip32.HARDENED_BIT, Bip32.HARDENED_BIT, 0L, 0L),
            Bip32.parsePath("m/44h/118h/0h/0/0"),
        )
    }

    // ------------------------------------------------------------- 地址与钱包

    @Test
    fun addressDerivationMatchesGoldenWallet() {
        val privateKey = hex("e7a3158f08d4f8474b59db0769151e5d64eee6216c482afc30895dfdf6e3e6e3")
        val wallet = McWallet.fromPrivateKey(privateKey)
        assertEquals("03d13fb1d3f67c4d39aefa52f013535aaa93c3f69927ed8038bc1fe2ef65a92ebb", Hex.encode(wallet.publicKey))
        assertEquals("mc1zdtn74r5ggkmzll07uvs8ehn0ep37lehwd55qx", wallet.address)
    }

    @Test
    fun walletFromMnemonicIsDeterministicAndSignable() {
        val a = McWallet.fromMnemonic(abandonAbout)
        val b = McWallet.fromMnemonic(abandonAbout, accountIndex = 0)
        assertEquals(a.address, b.address)
        assertEquals(McWallet.DEFAULT_HD_PATH, a.derivationPath)
        assertTrue("Cosmos 地址应以 mc1 开头", a.address.startsWith("mc1"))

        // 不同账户索引必须得到不同地址
        val other = McWallet.fromMnemonic(abandonAbout, accountIndex = 1)
        assertFalse(a.address == other.address)

        // 签名器对同一摘要必须确定性输出 64 字节
        val digest = hex("6deaa6333986cddd5b5841c971f6be479250960d3b626e76901aec3aa8a26d96")
        val sig1 = a.signer().signDigest(digest)
        val sig2 = a.signer().signDigest(digest)
        assertEquals(64, sig1.size)
        assertArrayEquals(sig1, sig2)
    }

    @Test
    fun walletRejectsInvalidInputs() {
        try {
            McWallet.fromMnemonic("abandon abandon abandon")
            throw AssertionError("非法助记词应抛异常")
        } catch (expected: IllegalArgumentException) {
            // ok
        }
        try {
            McWallet.fromMnemonic(abandonAbout, accountIndex = -1)
            throw AssertionError("负账户索引应抛异常")
        } catch (expected: IllegalArgumentException) {
            // ok
        }
        try {
            McWallet.fromPrivateKey(ByteArray(31))
            throw AssertionError("31 字节私钥应抛异常")
        } catch (expected: IllegalArgumentException) {
            // ok
        }
    }
}
