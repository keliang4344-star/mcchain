package com.mcchain.miner.shared

import com.google.gson.JsonObject
import com.google.gson.JsonParser
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.DepinTx
import com.mcchain.miner.shared.protocol.EdgeAiTx
import com.mcchain.miner.shared.protocol.PhonenodeTx
import com.mcchain.miner.shared.tx.DirectTxBuilder
import com.mcchain.miner.shared.tx.DirectTxInput
import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Sha256
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test
import java.net.HttpURLConnection
import java.net.URL
import java.util.Base64

/**
 * EdgeAI 全链路（devnet，方案 Phase 4 验收）：
 *   发布任务（escrow）→ 拉取任务 → 本地确定性执行 → MsgSubmitResult →
 *   重复提交被拒（edgeai/1203）→ 争议窗口（100 块）后链上结算拨付 85%。
 *
 * 与 App 侧 TaskExecutor 使用同一 hash_chain 算法，验证「结果可被抽样复算」。
 * 默认跳过；需要 devnet 时设置 MC_DEVNET_REST / MC_DEVNET_PRIVKEY / MC_DEVNET_ADDRESS。
 */
class EdgeAiDevnetTest {

    private val rest: String = System.getenv("MC_DEVNET_REST") ?: "http://127.0.0.1:1317"
    private val privateKeyHex: String? = System.getenv("MC_DEVNET_PRIVKEY")
    private val address: String? = System.getenv("MC_DEVNET_ADDRESS")
    private val chainId: String = System.getenv("MC_DEVNET_CHAIN_ID") ?: "mcchain-dev-1"

    /** 回执可见后等待查询状态追平的时长（devnet 出块 4s）。 */
    private val settleMs = 4_500L

    private val rewardUmc = 1_000_000L
    private val enterpriseFeeUmc = rewardUmc * 150 / 10_000   // EnterpriseSettlementFeeBps=150
    private val submitterShareUmc = rewardUmc * 8500 / 10_000 // EdgeAISubmitterRatioBps=8500
    private val feeUmc = 5_000L

    @Test
    fun createTaskSubmitResultAndSettleOnDevnet() {
        assumeTrue("未设置 MC_DEVNET_PRIVKEY/MC_DEVNET_ADDRESS，跳过 devnet 集成测试", privateKeyHex != null && address != null)
        val addr = address!!
        val signer = Secp256k1Signer(Hex.decode(privateKeyHex!!))

        ensureFullOnboarding(addr, signer)

        // 心跳（MsgSubmitStateProof）：两叶树自证（leaf + padding 兄弟），链上重建校验通过。
        val leaf = "phonenode|$addr|heartbeat-test|${System.currentTimeMillis()}"
        val leafHash = Sha256.digest(leaf.toByteArray())
        val paddingHash = Sha256.digest("mcchain-phone-node-heartbeat-padding".toByteArray())
        val root = Hex.encode(Sha256.digest(leafHash + paddingHash))
        val hbReceipt = submitOnce(addr, signer, listOf(PhonenodeTx.submitStateProof(addr, root, leaf, "0", Hex.encode(paddingHash))), 200_000, "heartbeat")
        assertEquals("心跳应成功：${hbReceipt["raw_log"]}", 0, hbReceipt["code"].asInt)

        val seed = "edgeai-${System.currentTimeMillis()}"
        val description = "{\"type\":\"hash_chain\",\"seed\":\"$seed\",\"iters\":1000}"
        val balanceStart = queryBalance(addr)

        // [1] 发布任务：escrow reward + 企业结算费从 creator 扣除
        val createReceipt = submitOnce(addr, signer, listOf(EdgeAiTx.createTask(addr, description, rewardUmc)), 500_000, "edgeai-create")
        assertEquals("create 应成功：${createReceipt["raw_log"]}", 0, createReceipt["code"].asInt)

        // [2] 按 description 找回任务 ID
        val taskId = findTaskId(addr, description)
        assertTrue("未找到刚创建的任务", taskId.isNotBlank())

        // [3] 本地确定性执行（与 App TaskExecutor.hashChain 相同算法）
        val resultHash = hashChain(seed, 1000)

        // [4] 提交结果
        val nonce = "devnet-nonce-$seed"
        val submitReceipt = submitOnce(addr, signer, listOf(EdgeAiTx.submitResult(addr, taskId, resultHash, nonce)), 500_000, "edgeai-result")
        assertEquals("submitResult 应成功：${submitReceipt["raw_log"]}", 0, submitReceipt["code"].asInt)
        assertTrue(
            "回执应包含 edgeai.ResultSubmitted 事件",
            hasEvent(submitReceipt, "edgeai.ResultSubmitted"),
        )

        // [5] 重复提交（不同结果哈希 ⇒ 不同交易字节）：链上以 ErrDuplicateResult 拒绝
        val dupReceipt = submitOnce(addr, signer, listOf(EdgeAiTx.submitResult(addr, taskId, "00".repeat(32), nonce)), 500_000, "edgeai-result-dup")
        assertEquals("重复提交应被拒绝：${dupReceipt["raw_log"]}", 1203, dupReceipt["code"].asInt)
        assertEquals("codespace", "edgeai", dupReceipt["codespace"].asString)

        // [6] 争议窗口（100 块 ≈ 400s）后结算：任务转 done
        waitTaskStatus(taskId, "done", timeoutMs = 600_000)

        // [7] 余额精确核对：-escrow -企业费 -3×txfee +85% 拨付
        val expected = balanceStart - rewardUmc - enterpriseFeeUmc - 3 * feeUmc + submitterShareUmc
        assertEquals("结算后余额", expected, queryBalance(addr))
    }

    // ------------------------------------------------------------------
    // 任务查询
    // ------------------------------------------------------------------

    private fun findTaskId(creator: String, description: String): String {
        val deadline = System.currentTimeMillis() + 60_000
        while (System.currentTimeMillis() < deadline) {
            val ids = runCatching {
                httpGet("$rest/mcchain/edgeai/tasks").getAsJsonArray("task_ids")
                    .map { it.asString }
            }.getOrDefault(emptyList())
            for (id in ids.takeLast(20).reversed()) {
                val task = runCatching {
                    JsonParser.parseString(
                        httpGet("$rest/mcchain/edgeai/task/$id").get("task_json").asString
                    ).asJsonObject
                }.getOrNull() ?: continue
                if (task.get("creator")?.asString == creator &&
                    task.get("description")?.asString == description
                ) {
                    return id
                }
            }
            Thread.sleep(2_000)
        }
        return ""
    }

    private fun waitTaskStatus(taskId: String, status: String, timeoutMs: Long) {
        val deadline = System.currentTimeMillis() + timeoutMs
        var last = ""
        while (System.currentTimeMillis() < deadline) {
            val task = runCatching {
                JsonParser.parseString(
                    httpGet("$rest/mcchain/edgeai/task/$taskId").get("task_json").asString
                ).asJsonObject
            }.getOrNull()
            last = task?.get("status")?.asString ?: "?"
            if (last == status) return
            Thread.sleep(3_000)
        }
        error("任务 $taskId 未在超时内变为 $status（当前 $last）")
    }

    private fun hasEvent(txResponse: JsonObject, type: String): Boolean {
        val events = txResponse.getAsJsonArray("events") ?: return false
        return events.any { it.asJsonObject["type"].asString == type }
    }

    /** 与 App 侧 TaskExecutor.hashChain 保持一致：迭代 SHA-256，指纹 = SHA256(最终 hex)。 */
    private fun hashChain(seed: String, iters: Int): String {
        var current = Hex.encode(Sha256.digest(seed.toByteArray(Charsets.UTF_8)))
        repeat(iters - 1) {
            current = Hex.encode(Sha256.digest(current.toByteArray(Charsets.UTF_8)))
        }
        return Hex.encode(Sha256.digest(current.toByteArray(Charsets.UTF_8)))
    }

    // ------------------------------------------------------------------
    // onboarding（与 DevnetBroadcastTest 相同的四步链路）
    // ------------------------------------------------------------------

    private fun ensureFullOnboarding(addr: String, signer: Secp256k1Signer) {
        submitTolerant(addr, signer, listOf(DepinTx.registerDevice(addr, addr, "DevnetTest", "Android14")), 200_000, "register-device", "device already registered")
        submitTolerant(addr, signer, listOf(PhonenodeTx.registerNode(addr, addr, "DevnetTest", "Android14", "depin")), 200_000, "register-node", "node already registered")

        // phonenode attestation 仍有效时跳过重认证；被吊销（离线 slash 冷却）则清晰跳过本次运行。
        if (!isPhonenodeAttested(addr)) {
            val deviceIdHash = Hex.encode(Sha256.digest("devnet-test-device|$addr".toByteArray()))
            val nonce = "devnetnonce${System.currentTimeMillis()}"
            val sigHex = Hex.encode(signer.signDigest(Sha256.digest("$deviceIdHash|$nonce".toByteArray())))
            val receipt = submitOnce(addr, signer, listOf(PhonenodeTx.submitAttestation(addr, "devnet-root|test", nonce, deviceIdHash, Hex.encode(signer.publicKeyCompressed()), sigHex)), 250_000, "attest-node")
            assumeTrue(
                "phonenode attestation 已被吊销（离线 slash 冷却 43200 块）；请用 tools/devnet_e2e.ps1 新建测试账户重跑：" +
                    receipt["raw_log"].asString,
                receipt["code"].asInt == 0,
            )
        }

        val depReceipt = submitOnce(addr, signer, listOf(DepinTx.attestDevice(addr, addr, "devnet-challenge-${System.currentTimeMillis()}", "00")), 250_000, "attest-device")
        assertEquals("depin 认证应成功：${depReceipt["raw_log"]}", 0, depReceipt["code"].asInt)
    }

    /** 查询 phonenode attestation 是否有效（status=valid 且未过期）。 */
    private fun isPhonenodeAttested(addr: String): Boolean {
        val body = runCatching { httpGet("$rest/mcchain/phonenode/attestation/$addr") }.getOrNull() ?: return false
        val att = body.getAsJsonObject("attestation") ?: return false
        val status = att["status"]?.asString ?: return false
        val expiry = att["expiry"]?.asString?.toLongOrNull() ?: return false
        return status == "valid" && expiry > System.currentTimeMillis() / 1000
    }

    /** 广播一笔交易；若回执非 0，仅容忍 expectedTolerance 指示的「已存在」类幂等错误。 */
    private fun submitTolerant(addr: String, signer: Secp256k1Signer, messages: List<ByteArray>, gas: Long, memo: String, expectedTolerance: String) {
        val receipt = submitOnce(addr, signer, messages, gas, memo)
        if (receipt["code"].asInt != 0) {
            assertTrue(
                "$memo 失败只允许是「$expectedTolerance」：${receipt["raw_log"]}",
                receipt["raw_log"].asString.contains(expectedTolerance),
            )
        }
    }

    private fun submitOnce(addr: String, signer: Secp256k1Signer, messages: List<ByteArray>, gas: Long, memo: String): JsonObject {
        val info = queryAccount(addr)
        val tx = DirectTxBuilder.build(
            DirectTxInput(
                messages = messages,
                chainId = chainId,
                accountNumber = info["accountNumber"].asLong,
                sequence = info["sequence"].asLong,
                fee = listOf(Coin("umc", feeUmc.toString())),
                gasLimit = gas,
                memo = memo,
            ),
            signer,
        )
        assertEquals("$memo broadcast", 0, broadcast(tx.txRawBytes)["code"].asInt)
        return waitForReceipt(tx.txHashHex).getAsJsonObject("tx_response")
    }

    // ------------------------------------------------------------------
    // REST
    // ------------------------------------------------------------------

    private fun queryAccount(addr: String): JsonObject {
        val body = httpGet("$rest/cosmos/auth/v1beta1/accounts/$addr")
        val account = body.getAsJsonObject("account")
        val base = when {
            account.has("base_account") -> account.getAsJsonObject("base_account")
            account.has("base_vesting_account") ->
                account.getAsJsonObject("base_vesting_account").getAsJsonObject("base_account")
            else -> account
        }
        return JsonObject().apply {
            addProperty("accountNumber", base["account_number"].asString.toLong())
            addProperty("sequence", base["sequence"].asString.toLong())
        }
    }

    private fun queryBalance(addr: String): Long {
        val body = httpGet("$rest/cosmos/bank/v1beta1/balances/$addr")
        val balances = body.getAsJsonArray("balances") ?: return 0L
        return balances.firstOrNull { it.asJsonObject["denom"].asString == "umc" }
            ?.asJsonObject?.get("amount")?.asString?.toLong() ?: 0L
    }

    private fun broadcast(txRaw: ByteArray): JsonObject {
        val payload = JsonObject().apply {
            addProperty("tx_bytes", Base64.getEncoder().encodeToString(txRaw))
            addProperty("mode", "BROADCAST_MODE_SYNC")
        }
        return httpPost("$rest/cosmos/tx/v1beta1/txs", payload.toString()).getAsJsonObject("tx_response")
    }

    private fun waitForReceipt(txHash: String, timeoutMs: Long = 60_000, intervalMs: Long = 1_000): JsonObject {
        val deadline = System.currentTimeMillis() + timeoutMs
        var lastError = ""
        while (System.currentTimeMillis() < deadline) {
            try {
                val body = httpGet("$rest/cosmos/tx/v1beta1/txs/$txHash")
                val height = body.getAsJsonObject("tx_response")["height"].asString.toLong()
                if (height > 0) {
                    Thread.sleep(settleMs)
                    return body
                }
            } catch (e: Exception) {
                lastError = e.message ?: ""
            }
            Thread.sleep(intervalMs)
        }
        error("等待 txhash=$txHash 回执超时（最后错误：$lastError）")
    }

    private fun httpGet(url: String): JsonObject =
        JsonParser.parseString(request(url, "GET", null)).asJsonObject

    private fun httpPost(url: String, body: String): JsonObject =
        JsonParser.parseString(request(url, "POST", body)).asJsonObject

    private fun request(url: String, method: String, body: String?): String {
        val conn = URL(url).openConnection() as HttpURLConnection
        try {
            conn.requestMethod = method
            conn.connectTimeout = 5_000
            conn.readTimeout = 15_000
            conn.setRequestProperty("Accept", "application/json")
            if (body != null) {
                conn.doOutput = true
                conn.setRequestProperty("Content-Type", "application/json")
                conn.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }
            }
            val code = conn.responseCode
            val stream = if (code in 200..299) conn.inputStream else conn.errorStream
            val text = stream?.use { it.readBytes().toString(Charsets.UTF_8) } ?: ""
            if (code !in 200..299) error("HTTP $code $url：$text")
            return text
        } finally {
            conn.disconnect()
        }
    }
}
