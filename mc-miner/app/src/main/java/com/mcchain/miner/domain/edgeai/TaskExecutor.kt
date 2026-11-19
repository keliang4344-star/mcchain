package com.mcchain.miner.domain.edgeai

import com.google.gson.JsonParser
import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Sha256

/**
 * EdgeAI 任务描述解析与确定性执行（方案 Phase 4：白名单任务 + 结果可抽样复算）。
 *
 * 白名单（首期）：
 *  - hash_chain：`{"type":"hash_chain","seed":"...","iters":N}`，
 *    执行 = 对 seed 迭代 SHA-256 N 次；result_hash = SHA256(finalHex)。
 *    纯 CPU、确定性、任何节点可复算，适合作为「结果可验证」的最小任务类型。
 *  - TFLite/ONNX 模型类任务：待模型资产与执行引擎接入后再加入白名单；
 *    未识别类型返回 null，调用方跳过执行（绝不用 SHA-256 冒充推理结果）。
 */
object TaskSpec {
    const val TYPE_HASH_CHAIN = "hash_chain"

    /** 迭代上限：10 万次 SHA-256 在中端机 <2s，兼顾任务时效与功耗红线。 */
    const val MAX_ITERS = 100_000
    const val MIN_ITERS = 1
    const val MAX_SEED_LENGTH = 1024

    sealed interface Spec {
        data class HashChain(val seed: String, val iters: Int) : Spec
    }

    /** 解析 description；非白名单/越界返回 null。 */
    fun parse(description: String): Spec? {
        val json = runCatching { JsonParser.parseString(description).asJsonObject }.getOrNull() ?: return null
        return when (json.get("type")?.asString) {
            TYPE_HASH_CHAIN -> {
                val seed = json.get("seed")?.asString ?: return null
                val iters = json.get("iters")?.asInt ?: return null
                if (seed.isEmpty() || seed.length > MAX_SEED_LENGTH) return null
                if (iters < MIN_ITERS || iters > MAX_ITERS) return null
                Spec.HashChain(seed, iters)
            }
            else -> null
        }
    }
}

/**
 * 任务执行器。所有实现必须确定性：同一 spec 在任何设备上产出同一 resultHash，
 * 这是链上争议重算（MsgSubmitRecompute）与抽样复算的前提。
 */
object TaskExecutor {

    /** 执行一个白名单任务，返回结果指纹（hex）；未知类型抛 IllegalArgumentException。 */
    fun execute(spec: TaskSpec.Spec): String = when (spec) {
        is TaskSpec.Spec.HashChain -> hashChain(spec.seed, spec.iters)
    }

    /** 迭代 SHA-256：h0 = SHA256(seed)，hi = SHA256(h(i-1) 的 hex 字符串)；指纹 = SHA256(最终 hex)。 */
    fun hashChain(seed: String, iters: Int): String {
        require(iters in TaskSpec.MIN_ITERS..TaskSpec.MAX_ITERS) { "iters out of range: $iters" }
        var current = Hex.encode(Sha256.digest(seed.toByteArray(Charsets.UTF_8)))
        repeat(iters - 1) {
            current = Hex.encode(Sha256.digest(current.toByteArray(Charsets.UTF_8)))
        }
        return Hex.encode(Sha256.digest(current.toByteArray(Charsets.UTF_8)))
    }
}
