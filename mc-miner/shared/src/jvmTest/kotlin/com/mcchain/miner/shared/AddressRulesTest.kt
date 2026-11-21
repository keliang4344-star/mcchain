package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.AddressRules
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class AddressRulesTest {

    private val account = "mc14hka32hqq4ejgd8dgu70nkljwtnzsnvh2wg87y"

    // 长度 55，处于验证人地址 48..62 的合法区间。
    private val validator = "mcvaloper1" + "q".repeat(45)

    @Test
    fun validAddressesPass() {
        assertNull(AddressRules.accountError(account, "mc"))
        assertNull(AddressRules.validatorError(validator, "mc"))
    }

    @Test
    fun emptyAddressIsRejected() {
        assertTrue(AddressRules.accountError("", "mc")!!.contains("请输入收款地址"))
        assertTrue(AddressRules.validatorError("", "mc")!!.contains("请输入验证人地址"))
    }

    @Test
    fun wrongPrefixIsRejected() {
        assertTrue(AddressRules.accountError(validator, "mc")!!.contains("必须以 mc1 开头"))
        assertTrue(AddressRules.validatorError(account, "mc")!!.contains("必须以 mcvaloper1 开头"))
    }

    @Test
    fun abnormalLengthIsRejected() {
        assertTrue(AddressRules.accountError("mc1qqq", "mc")!!.contains("长度异常"))
        val longValidator = "mcvaloper1" + "q".repeat(60)
        assertTrue(AddressRules.validatorError(longValidator, "mc")!!.contains("长度异常"))
    }

    @Test
    fun illegalCharactersAreRejected() {
        // 'b' 不在 bech32 字符集内，同时长度补足 45 以绕过长度检查。
        val badAccount = "mc1" + "b".repeat(42)
        assertTrue(AddressRules.accountError(badAccount, "mc")!!.contains("非法字符"))
    }
}
