package com.mcchain.miner.shared.protocol

/**
 * Cosmos SDK 标准模块消息编码（bank / staking / distribution）。
 * 返回值均为 Any 编码后的字节，可直接放入 TxBody.messages。
 *
 * 字段编号与 cosmos-sdk v0.47 proto 一致：
 *  - MsgSend: from_address=1, to_address=2, amount=3
 *  - MsgDelegate: delegator_address=1, validator_address=2, amount=3
 *  - MsgWithdrawDelegatorReward: delegator_address=1, validator_address=2
 */
object CosmosMsgs {
    const val TYPE_MSG_SEND = "/cosmos.bank.v1beta1.MsgSend"
    const val TYPE_MSG_DELEGATE = "/cosmos.staking.v1beta1.MsgDelegate"
    const val TYPE_MSG_UNDELEGATE = "/cosmos.staking.v1beta1.MsgUndelegate"
    const val TYPE_MSG_BEGIN_REDELEGATE = "/cosmos.staking.v1beta1.MsgBeginRedelegate"
    const val TYPE_MSG_WITHDRAW_DELEGATOR_REWARD =
        "/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward"

    fun send(fromAddress: String, toAddress: String, amount: Coin): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, fromAddress)
            writeStringField(2, toAddress)
            writeMessageField(3, CosmosTxEncoder.encodeCoin(amount))
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_SEND, value)
    }

    fun delegate(delegatorAddress: String, validatorAddress: String, amount: Coin): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, delegatorAddress)
            writeStringField(2, validatorAddress)
            writeMessageField(3, CosmosTxEncoder.encodeCoin(amount))
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_DELEGATE, value)
    }

    fun withdrawDelegatorReward(delegatorAddress: String, validatorAddress: String): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, delegatorAddress)
            writeStringField(2, validatorAddress)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_WITHDRAW_DELEGATOR_REWARD, value)
    }

    /** 解除委托（cosmos.staking.v1beta1.MsgUndelegate）。 */
    fun undelegate(delegatorAddress: String, validatorAddress: String, amount: Coin): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, delegatorAddress)
            writeStringField(2, validatorAddress)
            writeMessageField(3, CosmosTxEncoder.encodeCoin(amount))
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_UNDELEGATE, value)
    }

    /** 转委托（cosmos.staking.v1beta1.MsgBeginRedelegate）。 */
    fun beginRedelegate(
        delegatorAddress: String,
        sourceValidatorAddress: String,
        destinationValidatorAddress: String,
        amount: Coin,
    ): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, delegatorAddress)
            writeStringField(2, sourceValidatorAddress)
            writeStringField(3, destinationValidatorAddress)
            writeMessageField(4, CosmosTxEncoder.encodeCoin(amount))
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_BEGIN_REDELEGATE, value)
    }
}
