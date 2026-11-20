package com.mcchain.miner.shared.protocol

/**
 * mcchain.edgeai 交易消息编码（与 proto/mcchain/edgeai/tx.proto 字段号一致）。
 *
 * 移动端三条：
 *  - MsgSubmitResult：执行节点提交推理结果指纹（要求 phonenode 已认证）；
 *  - MsgCreateTask：需求方发布任务（escrow 锁定 reward）；
 *  - MsgOpenDispute：对任务结果发起争议（需缴纳链上参数规定的手续条件）。
 * ResolveDispute/SubmitRecompute 由仲裁者与验证者使用，不在此提供。
 */
object EdgeAiTx {
    const val TYPE_MSG_CREATE_TASK = "/mcchain.edgeai.MsgCreateTask"
    const val TYPE_MSG_SUBMIT_RESULT = "/mcchain.edgeai.MsgSubmitResult"
    const val TYPE_MSG_OPEN_DISPUTE = "/mcchain.edgeai.MsgOpenDispute"

    /** creator=1, description=2, reward=3（uint64 umc，proto3 零值省略）。 */
    fun createTask(creator: String, description: String, rewardUmc: Long): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, description)
            writeVarintField(3, rewardUmc)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_CREATE_TASK, value)
    }

    /**
     * creator=1, task_id=2, result_hash=3, attestation_nonce=4。
     * resultHash 为链下执行结果的确定性指纹；attestationNonce 用 depin 认证的 challenge。
     */
    fun submitResult(creator: String, taskId: String, resultHash: String, attestationNonce: String): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, taskId)
            writeStringField(3, resultHash)
            writeStringField(4, attestationNonce)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_SUBMIT_RESULT, value)
    }

    /** creator=1, task_id=2, reason=3。 */
    fun openDispute(creator: String, taskId: String, reason: String): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, taskId)
            writeStringField(3, reason)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_OPEN_DISPUTE, value)
    }
}
