package com.mcchain.miner.shared

import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosMsgs
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 解除委托 / 转委托消息编码回归。
 * 只校验类型 URL、地址与金额字段确实进入 Any 编码，防止字段号写错后链上拒签。
 */
class CosmosStakingMsgsTest {

    private val delegator = "mc1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"
    private val source = "mcvaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"
    private val destination = "mcvaloper1wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww"

    @Test
    fun undelegateEncodesTypeUrlAndFields() {
        val bytes = CosmosMsgs.undelegate(
            delegatorAddress = delegator,
            validatorAddress = source,
            amount = Coin("umc", "1000000"),
        )
        val text = bytes.toString(Charsets.ISO_8859_1)
        assertTrue(text.contains(CosmosMsgs.TYPE_MSG_UNDELEGATE))
        assertTrue(text.contains(delegator))
        assertTrue(text.contains(source))
        assertTrue(text.contains("1000000"))
    }

    @Test
    fun redelegateEncodesSourceAndDestination() {
        val bytes = CosmosMsgs.beginRedelegate(
            delegatorAddress = delegator,
            sourceValidatorAddress = source,
            destinationValidatorAddress = destination,
            amount = Coin("umc", "2500000"),
        )
        val text = bytes.toString(Charsets.ISO_8859_1)
        assertTrue(text.contains(CosmosMsgs.TYPE_MSG_BEGIN_REDELEGATE))
        assertTrue(text.contains(source))
        assertTrue(text.contains(destination))
        assertTrue(text.contains("2500000"))
    }

    @Test
    fun encodingIsDeterministic() {
        val first = CosmosMsgs.undelegate(delegator, source, Coin("umc", "1"))
        val second = CosmosMsgs.undelegate(delegator, source, Coin("umc", "1"))
        assertEquals(first.toList(), second.toList())
    }
}
