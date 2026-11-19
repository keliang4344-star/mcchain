package com.mcchain.miner.shared

import com.google.gson.JsonObject
import com.google.gson.JsonParser
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosMsgs
import com.mcchain.miner.shared.protocol.DepinTx
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
 * devnet 端到端验证：真实账户查询 → SIGN_MODE_DIRECT 签名 → REST 广播 → 回执确认。
 *
 * 默认跳过（CI 与普通单测不需要起链）。需要本地 devnet 时：
 *   MC_DEVNET_REST=http://127.0.0.1:1317
 *   MC_DEVNET_PRIVKEY=<验证人私钥 hex>
 *   MC_DEVNET_ADDRESS=<对应 mc1... 地址>
 *
 * 断言的余额变化同时覆盖了三条 Phase 1 验收要点：
 *  - 账户号/序列号取自链上，签名可被 ante 校验通过；
 *  - 手续费按 fee 全额扣除（本用例自转 1umc + 5000umc fee，净变化 -5000）；
 *  - 交易进入区块后 sequence 自增 1，回执可按 txhash 轮询到。
 */
class DevnetBroadcastTest {

    /** 回执可见后等待查询状态追平的时长（devnet 出块 4s）。 */
    private val SETTLE_MS = 4_500L

    private val rest: String = System.getenv("MC_DEVNET_REST") ?: "http://127.0.0.1:1317"
    private val privateKeyHex: String? = System.getenv("MC_DEVNET_PRIVKEY")
    private val address: String? = System.getenv("MC_DEVNET_ADDRESS")

    @Test
    fun selfTransferBroadcastsAndConfirmsOnDevnet() {
        assumeTrue("未设置 MC_DEVNET_PRIVKEY/MC_DEVNET_ADDRESS，跳过 devnet 集成测试", privateKeyHex != null && address != null)
        val addr = address!!
        val chainId = System.getenv("MC_DEVNET_CHAIN_ID") ?: "mcchain-dev-1"

        val before = queryAccount(addr)
        val beforeSequence = before["sequence"].asLong
        val beforeAccountNumber = before["accountNumber"].asLong
        val beforeBalance = queryBalance(addr)

        val signed = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(CosmosMsgs.send(addr, addr, Coin("umc", "1"))),
                chainId = chainId,
                accountNumber = beforeAccountNumber,
                sequence = beforeSequence,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 200_000,
            ),
            Secp256k1Signer(Hex.decode(privateKeyHex!!)),
        )

        val broadcast = broadcast(signed.txRawBytes)
        assertEquals("broadcast raw_log: ${broadcast["raw_log"]}", 0, broadcast["code"].asInt)
        assertEquals("txhash", signed.txHashHex.lowercase(), broadcast["txhash"].asString.lowercase())

        val receiptHeight = waitForHeight(signed.txHashHex)
        assertTrue("交易未进入区块", receiptHeight > 0)

        val after = queryAccount(addr)
        assertEquals("sequence 应自增 1", beforeSequence + 1, after["sequence"].asLong)
        assertEquals("account_number 不应变化", beforeAccountNumber, after["accountNumber"].asLong)
        assertEquals("余额应只扣除手续费", beforeBalance - 5000L, queryBalance(addr))

        // 断网重放场景：把同一笔签名交易字节再广播一次。
        // 因为 txhash 与 sequence 都已消耗，节点只会拒绝（already exists / sequence mismatch），
        // 余额不会再变化——这正是离线队列重放的幂等保证。
        val balanceAfterConfirm = queryBalance(addr)
        val replay = broadcast(signed.txRawBytes)
        assertTrue("重放不应被再次执行：${replay["raw_log"]}", replay["code"].asInt != 0)
        assertEquals("重放不得改变余额", balanceAfterConfirm, queryBalance(addr))
    }

    /**
     * 注册设备消息必须真正在链上执行：回执里能看到 depin.MsgRegisterDevice，
     * 且首次执行 code=0；重复执行时链上以 ErrDeviceExists 拒绝（幂等收敛）。
     */
    @Test
    fun registerDeviceLandsOnDevnet() {
        assumeTrue("未设置 MC_DEVNET_PRIVKEY/MC_DEVNET_ADDRESS，跳过 devnet 集成测试", privateKeyHex != null && address != null)
        val addr = address!!
        val chainId = System.getenv("MC_DEVNET_CHAIN_ID") ?: "mcchain-dev-1"
        val info = queryAccount(addr)

        val signed = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(DepinTx.registerDevice(addr, addr, "DevnetTest", "Android14")),
                chainId = chainId,
                accountNumber = info["accountNumber"].asLong,
                sequence = info["sequence"].asLong,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 200_000,
                memo = "register-device",
            ),
            Secp256k1Signer(Hex.decode(privateKeyHex!!)),
        )

        val broadcast = broadcast(signed.txRawBytes)
        assertEquals("broadcast raw_log: ${broadcast["raw_log"]}", 0, broadcast["code"].asInt)

        val receipt = waitForReceipt(signed.txHashHex)
        val response = receipt.getAsJsonObject("tx_response")
        // REST 回执结构为 { tx: { body: { messages: [...] } }, tx_response: {...} }
        val messages = receipt.getAsJsonObject("tx")
            ?.getAsJsonObject("body")
            ?.getAsJsonArray("messages")
        assertEquals(
            "链上交易消息类型",
            "/mcchain.depin.MsgRegisterDevice",
            messages?.firstOrNull()?.asJsonObject?.get("@type")?.asString,
        )

        if (response["code"].asInt != 0) {
            assertTrue(
                "重复注册应仅因 ErrDeviceExists 失败：${response["raw_log"]}",
                response["raw_log"].asString.contains("device already registered"),
            )
        }
    }

    /**
     * 用过期 sequence 签名时，链上必须以 sdk/32 拒绝。
     * App 的自动重签逻辑依赖这组 (codespace, code) 常量，这里直接对真实链校验。
     */
    @Test
    fun staleSequenceReturnsSdkCode32() {
        assumeTrue("未设置 MC_DEVNET_PRIVKEY/MC_DEVNET_ADDRESS，跳过 devnet 集成测试", privateKeyHex != null && address != null)
        val addr = address!!
        val chainId = System.getenv("MC_DEVNET_CHAIN_ID") ?: "mcchain-dev-1"
        val info = queryAccount(addr)
        val currentSequence = info["sequence"].asLong
        assumeTrue("账户 sequence 为 0，无法构造过期交易", currentSequence > 0)

        val signed = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(CosmosMsgs.send(addr, addr, Coin("umc", "1"))),
                chainId = chainId,
                accountNumber = info["accountNumber"].asLong,
                sequence = currentSequence - 1,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 200_000,
                // memo 让这笔交易与同 sequence 的正常用例字节不同，避免命中 "already exists in cache"。
                memo = "stale-sequence-test",
            ),
            Secp256k1Signer(Hex.decode(privateKeyHex!!)),
        )

        val broadcast = broadcast(signed.txRawBytes)
        assertEquals("过期 sequence 应被拒绝：${broadcast["raw_log"]}", 32, broadcast["code"].asInt)
        assertEquals("codespace", "sdk", broadcast["codespace"].asString)
    }

    /**
     * 设备认证上链 + 重放拒绝：
     *  - depin.MsgAttestDevice 在 devnet（SoftOracle）执行成功；
     *  - 同一个 challenge 第二次提交（新 sequence）必须因 challenge 已消费被拒绝（depin/1106）。
     * 这条链上行为是 Phase 2「重放被拒」验收的基础。
     */
    @Test
    fun attestDeviceLandsAndReplayIsRejected() {
        assumeTrue("未设置 MC_DEVNET_PRIVKEY/MC_DEVNET_ADDRESS，跳过 devnet 集成测试", privateKeyHex != null && address != null)
        val addr = address!!
        val chainId = System.getenv("MC_DEVNET_CHAIN_ID") ?: "mcchain-dev-1"
        val signer = Secp256k1Signer(Hex.decode(privateKeyHex!!))

        // 1) 确保设备已注册（重复注册时链上返回 ErrDeviceExists，视为幂等成功）
        val regInfo = queryAccount(addr)
        val register = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(DepinTx.registerDevice(addr, addr, "DevnetTest", "Android14")),
                chainId = chainId,
                accountNumber = regInfo["accountNumber"].asLong,
                sequence = regInfo["sequence"].asLong,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 200_000,
                memo = "register-device",
            ),
            signer,
        )
        assertEquals("register broadcast", 0, broadcast(register.txRawBytes)["code"].asInt)
        val regResponse = waitForReceipt(register.txHashHex).getAsJsonObject("tx_response")
        if (regResponse["code"].asInt != 0) {
            assertTrue(
                "注册失败只允许是 ErrDeviceExists：${regResponse["raw_log"]}",
                regResponse["raw_log"].asString.contains("device already registered"),
            )
        }

        // 2) 首次认证
        val challenge = "devnet-challenge-${System.currentTimeMillis()}"
        val attestInfo = queryAccount(addr)
        val attest = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(DepinTx.attestDevice(addr, addr, challenge, "00")),
                chainId = chainId,
                accountNumber = attestInfo["accountNumber"].asLong,
                sequence = attestInfo["sequence"].asLong,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 250_000,
                memo = "attest-device",
            ),
            signer,
        )
        assertEquals("attest broadcast", 0, broadcast(attest.txRawBytes)["code"].asInt)
        val attestResponse = waitForReceipt(attest.txHashHex).getAsJsonObject("tx_response")
        assertEquals("首次认证应成功：${attestResponse["raw_log"]}", 0, attestResponse["code"].asInt)

        // 3) 同一 challenge 重放（新 sequence，字节不同，确实是第二笔交易）
        val replayInfo = queryAccount(addr)
        val replay = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(DepinTx.attestDevice(addr, addr, challenge, "00")),
                chainId = chainId,
                accountNumber = replayInfo["accountNumber"].asLong,
                sequence = replayInfo["sequence"].asLong,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 250_000,
                memo = "attest-device",
            ),
            signer,
        )
        assertEquals("replay broadcast", 0, broadcast(replay.txRawBytes)["code"].asInt)
        val replayResponse = waitForReceipt(replay.txHashHex).getAsJsonObject("tx_response")
        assertEquals("重放应被拒绝", 1106, replayResponse["code"].asInt)
        assertEquals("codespace", "depin", replayResponse["codespace"].asString)
        assertTrue(
            "失败原因应为 challenge 已消费：${replayResponse["raw_log"]}",
            replayResponse["raw_log"].asString.contains("already consumed"),
        )
    }

    /**
     * 贡献提交全链路（devnet）：完整 onboarding（设备注册→节点注册→phonenode 自签认证→
     * depin 预言机认证）后提交 MsgSubmitContribution，验证：
     *  - 上链成功且发币闸口生效（depin.RewardPaid 事件，reward ∈ base×[0.5,1.5] 共振区间）；
     *  - 同一 taskId 以另一类型再提交（绕开 Layer3 频率防线）必须返回 depin/1102 ErrTaskExists。
     *
     * Layer3 对同设备同类型有 300 块（20 分钟）间隔限制，因此类型按时间桶轮换并带回退，
     * 保证测试在 20 分钟内重复运行仍然可靠。
     */
    @Test
    fun submitContributionPaysOutAndDuplicateTaskIdRejected() {
        assumeTrue("未设置 MC_DEVNET_PRIVKEY/MC_DEVNET_ADDRESS，跳过 devnet 集成测试", privateKeyHex != null && address != null)
        val addr = address!!
        val chainId = System.getenv("MC_DEVNET_CHAIN_ID") ?: "mcchain-dev-1"
        val signer = Secp256k1Signer(Hex.decode(privateKeyHex!!))
        val taskId = "devnet-contrib-${System.currentTimeMillis()}"
        val types = listOf("bandwidth", "data_label", "inference")
        val bucket = (System.currentTimeMillis() / 600_000).toInt()

        ensureFullOnboarding(addr, chainId, signer)

        // 首次提交：score=50（>=阈值30 才有奖励）；类型轮换 + Layer3 回退
        var usedType = ""
        var firstResponse: JsonObject? = null
        for (offset in 0..2) {
            val candidate = types[(bucket + offset) % 3]
            val response = submitContributionOnce(addr, chainId, signer, taskId, candidate, "50")
            if (response["code"].asInt == 0) {
                usedType = candidate
                firstResponse = response
                break
            }
            assertTrue(
                "提交失败只允许是 Layer3 频率防线（换下一类型）：${response["raw_log"]}",
                response["raw_log"].asString.contains("defense layer 3"),
            )
        }
        val first = firstResponse ?: error("三种类型都被 Layer3 拦截，请等 20 分钟后重跑")

        // 发币闸口：从事件读取 RewardPaid.reward。
        // 类型费率不同（bandwidth 1x / data_label 3x / inference 5x），共振倍数 ∈ [0.5,1.5]，
        // 因此按实际 usedType 费率断言区间：base=50×rate，reward ∈ [base/2, base×1.5]。
        val rate = when (usedType) { "inference" -> 5; "data_label" -> 3; else -> 1 }
        val base = 50L * rate
        val reward = rewardPaidAmount(first)
        assertTrue(
            "RewardPaid 事件 reward 应在 [$base/2, $base×1.5]，实际 $reward",
            reward in (base / 2)..(base * 3 / 2),
        )
        // 事件里的 task_type 必须与提交类型一致（幂等键与消息一致）
        assertEquals(usedType, taskTypeOf(first))

        // 重复 taskId（用未占用的其它类型绕开 Layer3）：链上必须以 ErrTaskExists 拒绝
        var dupCode = -1
        var dupLog = ""
        for (offset in 1..2) {
            val candidate = types[(bucket + offset) % 3]
            if (candidate == usedType) continue
            val dup = submitContributionOnce(addr, chainId, signer, taskId, candidate, "50")
            if (dup["code"].asInt == 1102) {
                dupCode = 1102
                assertEquals("codespace", "depin", dup["codespace"].asString)
                break
            }
            assertTrue(
                "重复提交失败只允许是 Layer3 或 ErrTaskExists：${dup["raw_log"]}",
                dup["raw_log"].asString.contains("defense layer 3") || dup["code"].asInt == 1102,
            )
            dupCode = dup["code"].asInt
            dupLog = dup["raw_log"].asString
        }
        assertEquals("应至少有一次以 ErrTaskExists 拒绝，最后结果：$dupLog", 1102, dupCode)
    }

    /** 广播一笔 MsgSubmitContribution 并等待回执。 */
    private fun submitContributionOnce(
        addr: String,
        chainId: String,
        signer: Secp256k1Signer,
        taskId: String,
        taskType: String,
        score: String,
    ): JsonObject {
        val info = queryAccount(addr)
        val tx = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(DepinTx.submitContribution(addr, taskId, taskType, score)),
                chainId = chainId,
                accountNumber = info["accountNumber"].asLong,
                sequence = info["sequence"].asLong,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 350_000,
            ),
            signer,
        )
        assertEquals("broadcast($taskType)", 0, broadcast(tx.txRawBytes)["code"].asInt)
        return waitForReceipt(tx.txHashHex).getAsJsonObject("tx_response")
    }

    /** 从回执事件中提取 depin.RewardPaid.reward（umc）；无事件返回 -1。
     *  REST 回执中 attribute 为明文字符串；RPC 路径是 base64，两者都兼容。 */
    private fun rewardPaidAmount(txResponse: JsonObject): Long {
        val events = txResponse.getAsJsonArray("events") ?: return -1
        for (element in events) {
            val event = element.asJsonObject
            if (event["type"].asString == "depin.RewardPaid") {
                val attr = event.getAsJsonArray("attributes").firstOrNull {
                    val key = it.asJsonObject["key"].asString
                    key == "reward" || decodeEventField(key) == "reward"
                } ?: return -1
                val value = attr.asJsonObject["value"].asString
                return value.toLongOrNull() ?: decodeEventField(value).toLongOrNull() ?: -1
            }
        }
        return -1
    }

    private fun decodeEventField(raw: String): String =
        runCatching { String(java.util.Base64.getDecoder().decode(raw)) }
            .getOrDefault(raw)
            .takeIf { it.all { c -> c.code in 32..126 } } ?: raw

    /** 从 depin.RewardPaid 事件读取 task_type。 */
    private fun taskTypeOf(txResponse: JsonObject): String {
        val events = txResponse.getAsJsonArray("events") ?: return ""
        for (element in events) {
            val event = element.asJsonObject
            if (event["type"].asString == "depin.RewardPaid") {
                val attr = event.getAsJsonArray("attributes").firstOrNull {
                    val key = it.asJsonObject["key"].asString
                    key == "task_type" || decodeEventField(key) == "task_type"
                } ?: return ""
                val value = attr.asJsonObject["value"].asString
                return if (value.all { c -> c.code in 32..126 }) value else decodeEventField(value)
            }
        }
        return ""
    }

    /**
     * 把测试账户置于「可发币贡献」状态：
     * depin 设备注册 → phonenode 节点注册 → phonenode 自签 attestation → depin MsgAttestDevice。
     * 各注册类错误按幂等容忍（ErrDeviceExists / ErrNodeExists）。
     */
    private fun ensureFullOnboarding(addr: String, chainId: String, signer: Secp256k1Signer) {
        // [1] depin.MsgRegisterDevice
        submitTolerant(
            addr, chainId, signer,
            listOf(DepinTx.registerDevice(addr, addr, "DevnetTest", "Android14")),
            200_000, "register-device", "device already registered",
        )

        // [2] phonenode.MsgRegisterNode
        submitTolerant(
            addr, chainId, signer,
            listOf(PhonenodeTx.registerNode(addr, addr, "DevnetTest", "Android14", "depin")),
            200_000, "register-node", "node already registered",
        )

        // [3] phonenode.MsgSubmitAttestation（设备自签 "deviceIdHash|nonce"）
        // deviceIdHash 按账户派生：SybilDeviceBinding 是全局 1:1，常量会撞其它测试账户的绑定。
        val deviceIdHash = Hex.encode(Sha256.digest("devnet-test-device|$addr".toByteArray()))
        val nonce = "devnetnonce${System.currentTimeMillis()}"
        val sigHex = Hex.encode(
            signer.signDigest(Sha256.digest("$deviceIdHash|$nonce".toByteArray()))
        )
        val attInfo = queryAccount(addr)
        val attest = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(
                    PhonenodeTx.submitAttestation(
                        addr, "devnet-root|test", nonce, deviceIdHash,
                        Hex.encode(signer.publicKeyCompressed()), sigHex,
                    ),
                ),
                chainId = chainId,
                accountNumber = attInfo["accountNumber"].asLong,
                sequence = attInfo["sequence"].asLong,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 250_000,
                memo = "attest-node",
            ),
            signer,
        )
        assertEquals("node attest broadcast", 0, broadcast(attest.txRawBytes)["code"].asInt)
        val nodeAttResponse = waitForReceipt(attest.txHashHex).getAsJsonObject("tx_response")
        assertEquals(
            "phonenode 自签认证应成功：codespace=${nodeAttResponse["codespace"]} ${nodeAttResponse["raw_log"]}",
            0, nodeAttResponse["code"].asInt,
        )

        // [4] depin.MsgAttestDevice（devnet SoftOracle：challenge+signature 非空即可）
        val depInfo = queryAccount(addr)
        val depAttest = DirectTxBuilder.build(
            DirectTxInput(
                messages = listOf(
                    DepinTx.attestDevice(addr, addr, "devnet-challenge-${System.currentTimeMillis()}", "00"),
                ),
                chainId = chainId,
                accountNumber = depInfo["accountNumber"].asLong,
                sequence = depInfo["sequence"].asLong,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = 250_000,
                memo = "attest-device",
            ),
            signer,
        )
        assertEquals("device attest broadcast", 0, broadcast(depAttest.txRawBytes)["code"].asInt)
        val depResponse = waitForReceipt(depAttest.txHashHex).getAsJsonObject("tx_response")
        assertEquals("depin 认证应成功：${depResponse["raw_log"]}", 0, depResponse["code"].asInt)
    }

    /** 广播一笔交易；若回执非 0，仅容忍 expectedTolerance 指示的「已存在」类幂等错误。 */
    private fun submitTolerant(
        addr: String,
        chainId: String,
        signer: Secp256k1Signer,
        messages: List<ByteArray>,
        gas: Long,
        memo: String,
        expectedTolerance: String,
    ) {
        val info = queryAccount(addr)
        val tx = DirectTxBuilder.build(
            DirectTxInput(
                messages = messages,
                chainId = chainId,
                accountNumber = info["accountNumber"].asLong,
                sequence = info["sequence"].asLong,
                fee = listOf(Coin("umc", "5000")),
                gasLimit = gas,
                memo = memo,
            ),
            signer,
        )
        assertEquals("$memo broadcast", 0, broadcast(tx.txRawBytes)["code"].asInt)
        val response = waitForReceipt(tx.txHashHex).getAsJsonObject("tx_response")
        if (response["code"].asInt != 0) {
            assertTrue(
                "$memo 失败只允许是「$expectedTolerance」：${response["raw_log"]}",
                response["raw_log"].asString.contains(expectedTolerance),
            )
        }
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
        val body = httpPost("$rest/cosmos/tx/v1beta1/txs", payload.toString())
        return body.getAsJsonObject("tx_response")
    }

    private fun waitForHeight(txHash: String, timeoutMs: Long = 60_000, intervalMs: Long = 1_000): Long {
        val receipt = waitForReceipt(txHash, timeoutMs, intervalMs)
        val response = receipt.getAsJsonObject("tx_response")
        val height = response["height"].asString.toLong()
        assertEquals("交易执行失败：${response["raw_log"]}", 0, response["code"].asInt)
        return height
    }

    /** 轮询 REST 回执，返回完整 { tx, tx_response } 结构。 */
    private fun waitForReceipt(txHash: String, timeoutMs: Long = 60_000, intervalMs: Long = 1_000): JsonObject {
        val deadline = System.currentTimeMillis() + timeoutMs
        var lastError = ""
        while (System.currentTimeMillis() < deadline) {
            try {
                val body = httpGet("$rest/cosmos/tx/v1beta1/txs/$txHash")
                val height = body.getAsJsonObject("tx_response")["height"].asString.toLong()
                if (height > 0) {
                    // 关键：GetTx 可见时，auth/bank 查询状态可能仍落后一个区块。
                    // 不 settle 就查账户会拿到过期 sequence，签出的交易会被 CheckTx 拒掉。
                    Thread.sleep(SETTLE_MS)
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
            conn.readTimeout = 10_000
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
