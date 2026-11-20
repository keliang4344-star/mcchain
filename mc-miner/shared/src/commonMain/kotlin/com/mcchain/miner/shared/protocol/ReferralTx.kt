package com.mcchain.miner.shared.protocol

/**
 * mcchain.referral 交易消息编码（与 proto/mcchain/referral/tx.proto 字段号一致）。
 *
 * 链上语义：
 *  - MsgCreateReferral 由邀请人发起，被邀请人必须是已注册 phonenode，且未被他人邀请；
 *  - MsgClaimReferralReward 由邀请人领取累计返佣，奖励来自生态池，不从被邀请人扣除。
 */
object ReferralTx {
    const val TYPE_MSG_CREATE_REFERRAL = "/mcchain.referral.MsgCreateReferral"
    const val TYPE_MSG_CLAIM_REFERRAL_REWARD = "/mcchain.referral.MsgClaimReferralReward"

    /** inviter=1, invitee=2, invite_code=3（可选渠道码）。 */
    fun createReferral(inviter: String, invitee: String, inviteCode: String): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, inviter)
            writeStringField(2, invitee)
            writeStringField(3, inviteCode)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_CREATE_REFERRAL, value)
    }

    /** claimer=1。 */
    fun claimReward(claimer: String): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, claimer)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_CLAIM_REFERRAL_REWARD, value)
    }
}
