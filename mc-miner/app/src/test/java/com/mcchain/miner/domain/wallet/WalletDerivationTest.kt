package com.mcchain.miner.domain.wallet

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 对照 mcchaind 的派生结果：
 *   mcchaind keys add probe --recover（BIP-39 标准测试助记词，BIP-44 m/44'/118'/0'/0/0）
 *   -> mc19rl4cm2hmr8afy4kldpxz3fka4jguq0au9f863
 */
class WalletDerivationTest {

    private val mnemonic =
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
            .split(" ")

    @Test
    fun standardMnemonicDerivesCliCompatibleAddress() {
        val wallet = WalletDerivation.derive(mnemonic)
        assertEquals("mc19rl4cm2hmr8afy4kldpxz3fka4jguq0au9f863", wallet.address)
        assertEquals(32, wallet.privateKey.size)
        assertEquals(33, wallet.publicKey.size)
    }

    @Test
    fun invalidMnemonicsAreRejected() {
        assertFalse(WalletDerivation.isValidMnemonic(listOf("abandon")))
        assertFalse(WalletDerivation.isValidMnemonic("abandon ".repeat(12).trim().split(" ")))
    }

    @Test
    fun validMnemonicIsAccepted() {
        assertTrue(WalletDerivation.isValidMnemonic(mnemonic))
    }

    @Test
    fun accountIndexDerivesDistinctDeterministicAddresses() {
        val first = WalletDerivation.derive(mnemonic, 0)
        val firstAgain = WalletDerivation.derive(mnemonic, 0)
        val second = WalletDerivation.derive(mnemonic, 1)
        assertEquals("同一索引必须稳定", first.address, firstAgain.address)
        assertNotEquals("不同索引必须得到不同地址", first.address, second.address)
        assertTrue(second.address.startsWith("mc1"))
        assertEquals(32, second.privateKey.size)
    }
}
