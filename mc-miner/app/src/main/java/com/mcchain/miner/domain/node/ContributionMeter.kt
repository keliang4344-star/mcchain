package com.mcchain.miner.domain.node

/**
 * 贡献计量：把本机原始测量值映射为链上 score，并生成幂等键。
 *
 * 设计（对齐开发方案 §[5][6] 与链上 depin 规则）：
 *  - epoch = 24 小时桶（UTC），taskId = `deviceId|epoch|chainTaskType`；
 *    链上 Contribution:<taskID> 唯一 ⇒ 每类型每天最多一条贡献，天然构成日上限与去重；
 *  - score 钳制在 [0,100]（链上硬校验），低于 [MIN_SUBMIT_SCORE] 不提交，避免白付手续费；
 *  - 链上支持类型仅 inference / data_label / bandwidth（x/depin/keeper/reward.go），
 *    存储贡献暂无对应类型，保持 LOCAL_ONLY。
 *
 * 纯函数类，不依赖 Android，可单测。
 */
class ContributionMeter(
    private val nowMs: () -> Long = { System.currentTimeMillis() },
) {
    /** 当前 24 小时 epoch（UTC 桶）。 */
    fun currentEpoch(): Long = nowMs() / EPOCH_MS

    /** 链上幂等键：同一设备、同一 epoch、同一类型只会生效一次。 */
    fun taskId(deviceId: String, epoch: Long, chainTaskType: String): String =
        "$deviceId|$epoch|$chainTaskType"

    /** 带宽贡献分数：按当日增量字节线性映射，50MB=1 分，封顶 100。 */
    fun bandwidthScore(deltaBytes: Long): Int = scoreFor(deltaBytes, BANDWIDTH_BYTES_PER_SCORE)

    /** 推理任务分数：按完成任务数映射，10 个=1 分，封顶 100。 */
    fun inferenceScore(completedTasks: Int): Int = scoreFor(completedTasks.toLong(), INFERENCE_TASKS_PER_SCORE)

    /**
     * 本机测量值 → 链上支持类型的映射；不支持的类型返回 null（保持 LOCAL_ONLY，
     * 绝不提交注定被 ErrUnsupportedType 拒绝的交易）。
     */
    fun chainTaskType(localTaskType: String): String? = when (localTaskType) {
        "BANDWIDTH" -> "bandwidth"
        "AI_INFERENCE" -> "inference"
        "DATA_LABELING" -> "data_label"
        else -> null // DATA_STORAGE / COMPUTATION：链上暂无对应类型
    }

    private fun scoreFor(delta: Long, perScore: Long): Int {
        if (delta <= 0 || perScore <= 0) return 0
        val raw = delta / perScore
        return raw.coerceAtMost(100L).toInt()
    }

    companion object {
        const val EPOCH_MS = 24L * 60 * 60 * 1000

        const val BANDWIDTH_BYTES_PER_SCORE = 50L * 1024 * 1024
        const val INFERENCE_TASKS_PER_SCORE = 10L

        /** 低于该分数提交注定 reward=0（链上阈值 30），且大概率被质量防线拒绝，不如不花手续费。 */
        const val MIN_SUBMIT_SCORE = 30

        /** 与链上 ContributionThreshold 对齐的质量阈值：>=30 才有奖励。 */
        const val REWARD_THRESHOLD = 30
    }
}
