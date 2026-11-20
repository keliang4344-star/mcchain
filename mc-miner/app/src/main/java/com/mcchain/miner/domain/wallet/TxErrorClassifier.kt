package com.mcchain.miner.domain.wallet

/**
 * 链上错误到中文提示的映射。
 *
 * 同时看 ABCI code + codespace（精确）与 raw_log 文本（兜底）：
 * 同一数字 code 在不同模块会重复（depin 1100 / phonenode 1100），
 * 所以仅在 codespace 匹配时才信任 code。
 *
 * 纯函数、无 Android 依赖，便于 JVM 单测覆盖。
 */
object TxErrorClassifier {

    /** 同一笔交易字节被节点判定为「已在 mempool / 已上链」——幂等重放时属正常现象。 */
    fun isAlreadyKnown(log: String): Boolean =
        log.contains("already exists", ignoreCase = true) ||
            log.contains("tx already exists in cache", ignoreCase = true)

    fun isSequenceMismatch(code: Int, codespace: String, log: String): Boolean =
        (codespace == CODESPACE_SDK &&
            (code == SDK_ERR_INVALID_SEQUENCE || code == SDK_ERR_WRONG_SEQUENCE)) ||
            log.contains("account sequence mismatch", ignoreCase = true) ||
            log.contains("incorrect account sequence", ignoreCase = true)

    fun isInsufficientFunds(code: Int, codespace: String, log: String): Boolean =
        (codespace == CODESPACE_SDK && code == SDK_ERR_INSUFFICIENT_FUNDS) ||
            log.contains("insufficient funds", ignoreCase = true)

    fun isInsufficientFee(code: Int, codespace: String, log: String): Boolean =
        (codespace == CODESPACE_SDK && code == SDK_ERR_INSUFFICIENT_FEE) ||
            log.contains("insufficient fee", ignoreCase = true)

    /**
     * 这些拒绝原因不会随时间自行恢复（sequence 已被消耗 / 余额永久不足 / 签名不匹配），
     * 队列里应直接置为失败，避免无限重试。
     */
    fun isPermanentRejection(code: Int, codespace: String, log: String): Boolean =
        isSequenceMismatch(code, codespace, log) ||
            isInsufficientFunds(code, codespace, log) ||
            isInsufficientFee(code, codespace, log) ||
            log.contains("signature verification failed", ignoreCase = true)

    fun classify(code: Int, codespace: String, rawLog: String): String {
        val log = rawLog.trim()
        return when {
            isSequenceMismatch(code, codespace, log) ->
                "交易序号不匹配（sequence mismatch），请刷新后重试"
            isInsufficientFee(code, codespace, log) ->
                "手续费不足，请检查链上最低 gas price"
            isInsufficientFunds(code, codespace, log) ->
                "余额不足，无法支付金额与手续费"
            (codespace == CODESPACE_DEPIN && code == DEPIN_ERR_DEVICE_EXISTS) ||
                log.contains("device already registered", ignoreCase = true) ->
                "设备已上链注册，无需重复注册"
            (codespace == CODESPACE_DEPIN && code == DEPIN_ERR_TASK_EXISTS) ||
                log.contains("contribution task id already exists", ignoreCase = true) ||
                log.contains("task id already exists", ignoreCase = true) ->
                "贡献已记录（幂等收敛）"
            codespace == CODESPACE_DEPIN && code == DEPIN_ERR_DEVICE_NOT_FOUND ->
                "设备尚未上链注册，请稍后重试"
            codespace == CODESPACE_DEPIN && code == DEPIN_ERR_DEVICE_NOT_ATTESTED ->
                "设备尚未通过认证，暂时不能提交贡献"
            (codespace == CODESPACE_DEPIN &&
                (code == DEPIN_ERR_INVALID_ATTESTATION || code == DEPIN_ERR_ATTESTATION_FAILED)) ||
                log.contains("invalid attestation", ignoreCase = true) ->
                "设备认证无效或已过期，请重新认证"
            codespace == CODESPACE_DEPIN && code == DEPIN_ERR_PHONENODE_NOT_REGISTERED ->
                "当前钱包未注册为节点，请先注册节点角色"
            codespace == CODESPACE_PHONENODE && code == PHONENODE_ERR_NODE_EXISTS ->
                "节点已注册，无需重复注册"
            codespace == CODESPACE_PHONENODE && code == PHONENODE_ERR_NONCE_REUSED ->
                "认证 nonce 已被使用，请重新发起认证"
            codespace == CODESPACE_PHONENODE && code == PHONENODE_ERR_DEVICE_ALREADY_BOUND ->
                "设备已绑定其他节点地址，拒绝重复绑定"
            (codespace == CODESPACE_PHONENODE && code == PHONENODE_ERR_INVALID_ATTESTATION) ||
                log.contains("attestation signature verification failed", ignoreCase = true) ->
                "设备自签认证验签失败，请重试认证"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_TASK_NOT_FOUND ->
                "任务不存在或已被清理"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_TASK_NOT_OPEN ->
                "任务已不再开放（已被结算或过期）"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_TASK_ALREADY_DONE ->
                "任务已完成（幂等收敛）"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_DUPLICATE_RESULT ->
                "本节点已提交过该任务结果（幂等收敛）"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_NOT_ASSIGNED ->
                "任务已分配给其他节点"
            (codespace == CODESPACE_SDK && code == SDK_ERR_UNAUTHORIZED) ||
                log.contains("node not attested", ignoreCase = true) ->
                "节点未认证或认证已过期，请先完成设备认证"
            log.contains("invalid", ignoreCase = true) ->
                "交易无效：$log"
            log.isBlank() -> "广播失败，请稍后重试"
            else -> log
        }
    }

    const val CODESPACE_SDK = "sdk"
    const val CODESPACE_DEPIN = "depin"
    const val CODESPACE_PHONENODE = "phonenode"
    const val CODESPACE_EDGEAI = "edgeai"

    // Cosmos SDK types/errors/errors.go
    const val SDK_ERR_INVALID_SEQUENCE = 3
    const val SDK_ERR_UNAUTHORIZED = 4
    const val SDK_ERR_INSUFFICIENT_FUNDS = 5
    const val SDK_ERR_INSUFFICIENT_FEE = 13
    const val SDK_ERR_WRONG_SEQUENCE = 32

    // x/depin/types/errors.go
    const val DEPIN_ERR_DEVICE_EXISTS = 1100
    const val DEPIN_ERR_DEVICE_NOT_FOUND = 1101
    const val DEPIN_ERR_TASK_EXISTS = 1102
    const val DEPIN_ERR_DEVICE_NOT_ATTESTED = 1105
    const val DEPIN_ERR_INVALID_ATTESTATION = 1106
    const val DEPIN_ERR_PHONENODE_NOT_REGISTERED = 1107
    const val DEPIN_ERR_ATTESTATION_FAILED = 1108

    // x/phonenode/types/errors.go
    const val PHONENODE_ERR_NODE_EXISTS = 1100
    const val PHONENODE_ERR_NONCE_REUSED = 1104
    const val PHONENODE_ERR_DEVICE_ALREADY_BOUND = 1105
    const val PHONENODE_ERR_INVALID_ATTESTATION = 1106

    // x/edgeai/types/errors.go
    const val EDGEAI_ERR_TASK_NOT_FOUND = 1200
    const val EDGEAI_ERR_TASK_NOT_OPEN = 1201
    const val EDGEAI_ERR_TASK_ALREADY_DONE = 1202
    const val EDGEAI_ERR_DUPLICATE_RESULT = 1203
    const val EDGEAI_ERR_NOT_ASSIGNED = 1206
}
