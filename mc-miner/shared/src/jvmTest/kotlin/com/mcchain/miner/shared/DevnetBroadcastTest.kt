package com.mcchain.miner.shared

import com.google.gson.JsonObject
import com.google.gson.JsonParser
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosMsgs
import com.mcchain.miner.shared.protocol.DepinTx
import com.mcchain.miner.shared.tx.DirectTxBuilder
import com.mcchain.miner.shared.tx.DirectTxInput
import com.mcchain.miner.shared.util.Hex
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
                if (height > 0) return body
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
