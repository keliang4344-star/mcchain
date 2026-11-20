package com.mcchain.miner.shared

import com.mcchain.miner.shared.protocol.EdgeAiTx
import com.mcchain.miner.shared.protocol.ReferralTx
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 推荐返佣 / EdgeAI 争议消息编码回归。
 * 只校验类型 URL 与关键字段进入 Any 编码，防止字段号写错后被链上拒签。
 */
class EcosystemMsgsTest {

    private val inviter = "mc1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"
    private val invitee = "mc1wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww"

    @Test
    fun createReferralEncodesInviterInviteeAndCode() {
        val bytes = ReferralTx.createReferral(inviter, invitee, "channel-42")
        val text = bytes.toString(Charsets.ISO_8859_1)
        assertTrue(text.contains(ReferralTx.TYPE_MSG_CREATE_REFERRAL))
        assertTrue(text.contains(inviter))
        assertTrue(text.contains(invitee))
        assertTrue(text.contains("channel-42"))
    }

    @Test
    fun claimReferralRewardEncodesClaimer() {
        val bytes = ReferralTx.claimReward(inviter)
        val text = bytes.toString(Charsets.ISO_8859_1)
        assertTrue(text.contains(ReferralTx.TYPE_MSG_CLAIM_REFERRAL_REWARD))
        assertTrue(text.contains(inviter))
    }

    @Test
    fun openDisputeEncodesTaskAndReason() {
        val bytes = EdgeAiTx.openDispute(inviter, "task-7", "result-hash-mismatch")
        val text = bytes.toString(Charsets.ISO_8859_1)
        assertTrue(text.contains(EdgeAiTx.TYPE_MSG_OPEN_DISPUTE))
        assertTrue(text.contains("task-7"))
        assertTrue(text.contains("result-hash-mismatch"))
    }

    @Test
    fun encodingIsDeterministic() {
        val first = ReferralTx.createReferral(inviter, invitee, "c1")
        val second = ReferralTx.createReferral(inviter, invitee, "c1")
        assertEquals(first.toList(), second.toList())
    }
}
