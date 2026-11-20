package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.Bech32Address
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * bech32 分隔符回归：分隔符 `1` 不属于数据字符集，校验必须只看数据部分。
 * 之前把整串地址丢进字符集检查，导致所有合法地址被拒绝。
 */
class Bech32AddressTest {

    private val account = "mc14hka32hqq4ejgd8dgu70nkljwtnzsnvh2wg87y"
    private val validator = "mcvaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"

    @Test
    fun accountAddressWithSeparatorIsValid() {
        assertTrue(Bech32Address.hasValidDataPart(account, "mc"))
    }

    @Test
    fun validatorAddressWithSeparatorIsValid() {
        assertTrue(Bech32Address.hasValidDataPart(validator, "mcvaloper"))
    }

    @Test
    fun invalidCharactersAfterSeparatorAreRejected() {
        // 'b' 不在 bech32 字符集内。
        assertFalse(Bech32Address.hasValidDataPart("mc1qqqbqqq", "mc"))
    }

    @Test
    fun missingSeparatorIsRejected() {
        assertFalse(Bech32Address.hasValidDataPart("mcqqqqqqqq", "mc"))
    }

    @Test
    fun wrongHrpIsRejected() {
        assertFalse(Bech32Address.hasValidDataPart(account, "mcvaloper"))
    }
}
