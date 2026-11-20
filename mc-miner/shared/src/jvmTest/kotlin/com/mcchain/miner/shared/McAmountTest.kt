package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.McAmount
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * MC/umc 金额换算回归：精度、边界与非法输入。
 */
class McAmountTest {

    @Test
    fun parsesIntegerMc() {
        assertEquals(1_000_000L, McAmount.parseToUmc("1"))
        assertEquals(12_500_000L, McAmount.parseToUmc("12.5"))
    }

    @Test
    fun parsesMinimumUnit() {
        assertEquals(1L, McAmount.parseToUmc("0.000001"))
    }

    @Test
    fun zeroIsAllowedButCallerChecksPositive() {
        assertEquals(0L, McAmount.parseToUmc("0"))
    }

    @Test
    fun rejectsMoreThanSixDecimals() {
        assertNull(McAmount.parseToUmc("1.2345678"))
    }

    @Test
    fun rejectsNegativeBlankScientificAndNonNumeric() {
        assertNull(McAmount.parseToUmc("-1"))
        assertNull(McAmount.parseToUmc(""))
        assertNull(McAmount.parseToUmc("   "))
        assertNull(McAmount.parseToUmc("1e6"))
        assertNull(McAmount.parseToUmc("abc"))
        assertNull(McAmount.parseToUmc("1,5"))
    }

    @Test
    fun rejectsOverflow() {
        // 18 位整数 ×1e6 超过 Long 上限。
        assertNull(McAmount.parseToUmc("999999999999999999"))
    }

    @Test
    fun formatsWithoutScientificNotation() {
        assertEquals("0", McAmount.format(0))
        assertEquals("1", McAmount.format(1_000_000))
        assertEquals("0.000001", McAmount.format(1))
        assertEquals("1.234567", McAmount.format(1_234_567))
    }
}
