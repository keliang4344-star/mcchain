package com.mcchain.miner.shared.protocol

/**
 * mcchain.depin 交易消息编码。返回值均为 Any 编码后的字节，可直接放入 TxBody.messages。
 *
 * MsgSubmitAttestation 由预言机提交，移动端不构造，故不在此提供。
 */
object DepinTx {
    const val TYPE_MSG_REGISTER_DEVICE = "/mcchain.depin.MsgRegisterDevice"
    const val TYPE_MSG_ATTEST_DEVICE = "/mcchain.depin.MsgAttestDevice"
    const val TYPE_MSG_SUBMIT_CONTRIBUTION = "/mcchain.depin.MsgSubmitContribution"

    /** MsgRegisterDevice：creator 必须与签名账户一致（链上会拒绝为他人注册）。 */
    fun registerDevice(
        creator: String,
        address: String,
        model: String,
        os: String,
    ): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, address)
            writeStringField(3, model)
            writeStringField(4, os)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_REGISTER_DEVICE, value)
    }

    fun attestDevice(
        creator: String,
        address: String,
        challenge: String,
        signature: String,
    ): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, address)
            writeStringField(3, challenge)
            writeStringField(4, signature)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_ATTEST_DEVICE, value)
    }

    fun submitContribution(
        creator: String,
        taskId: String,
        taskType: String,
        score: String,
    ): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, taskId)
            writeStringField(3, taskType)
            writeStringField(4, score)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_SUBMIT_CONTRIBUTION, value)
    }
}
