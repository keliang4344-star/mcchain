package com.mcchain.miner.network

import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.*
import com.mcchain.miner.data.pref.SecurePrefs
import com.google.gson.Gson
import com.google.gson.JsonObject
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Cosmos SDK Tendermint RPC 客户端。
 * 支持状态查询、交易广播、区块订阅。
 */
@Singleton
class RpcClient @Inject constructor(
    private val gson: Gson,
    private val securePrefs: SecurePrefs,
) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(10, java.util.concurrent.TimeUnit.SECONDS)
        .readTimeout(30, java.util.concurrent.TimeUnit.SECONDS)
        .build()

    private val JSON = "application/json; charset=utf-8".toMediaType()
    private val defaultEndpoint = "http://127.0.0.1:${MCParams.DEFAULT_RPC_PORT}"

    /** 启动时读取设置里保存的节点地址；读取失败（安全存储不可用）则退回默认值。 */
    private var endpoint: String = runCatching { securePrefs.rpcEndpoint }
        .getOrNull()
        ?.takeIf { it.isNotBlank() }
        ?: defaultEndpoint

    private val _latestHeight = MutableStateFlow(0L)
    val latestHeight: StateFlow<Long> = _latestHeight

    fun setEndpoint(url: String) {
        endpoint = url.trim().trimEnd('/').ifBlank { defaultEndpoint }
    }

    /**
     * 获取最新区块高度
     */
    suspend fun getLatestBlockHeight(): Long {
        val response = rpcCall("status", mapOf())
        val height = response?.getAsJsonObject("result")
            ?.getAsJsonObject("sync_info")
            ?.get("latest_block_height")
            ?.asLong ?: 0L
        _latestHeight.value = height
        return height
    }

    /**
     * 连通性探测：能取到非零区块高度即视为 RPC 可达。
     */
    suspend fun ping(): Boolean = runCatching { getLatestBlockHeight() > 0L }.getOrDefault(false)

    /**
     * 针对指定端点做探测，不修改当前客户端端点（供设置页「测试连接」使用）。
     */
    suspend fun ping(target: String): Boolean = withContext(Dispatchers.IO) {
        runCatching {
            val body = gson.toJson(
                mapOf("jsonrpc" to "2.0", "id" to 1, "method" to "status", "params" to emptyMap<String, Any>())
            ).toRequestBody(JSON)
            val request = Request.Builder()
                .url(target.trim().trimEnd('/'))
                .post(body)
                .build()
            client.newCall(request).execute().use { response ->
                response.isSuccessful && (response.body?.string()?.contains("latest_block_height") == true)
            }
        }.getOrDefault(false)
    }

    /**
     * 获取节点状态。[target] 非空时探测该地址，不改变当前客户端端点（供设置页「测试连接」）。
     */
    suspend fun getNodeStatus(target: String? = null): NodeStatusSnapshot? {
        val response = rpcCall("status", mapOf(), target) ?: return null
        val result = response.getAsJsonObject("result") ?: return null
        val syncInfo = result.getAsJsonObject("sync_info") ?: return null
        val nodeInfo = result.getAsJsonObject("node_info") ?: return null

        return NodeStatusSnapshot(
            currentHeight = syncInfo.get("latest_block_height")?.asLong ?: 0L,
            latestBlockTime = parseIsoTime(syncInfo.get("latest_block_time")?.asString ?: ""),
            syncedPeers = syncInfo.get("n_peers")?.asInt ?: 0,
            chainId = nodeInfo.get("network")?.asString ?: "",
            nodeVersion = nodeInfo.get("version")?.asString ?: "",
            catchingUp = syncInfo.get("catching_up")?.asBoolean ?: false,
            totalStorageBytes = 0,
            bandwidthUpBps = 0,
            bandwidthDownBps = 0,
            lastUpdated = System.currentTimeMillis()
        )
    }

    /**
     * 获取账户信息
     */
    suspend fun getAccount(address: String): Triple<Long, Long, List<RpcCoin>>? {
        val path = "/cosmos/auth/v1beta1/accounts/$address"
        val response = restGet(path) ?: return null
        val account = response.getAsJsonObject("account") ?: return null
        val baseAccount = account.getAsJsonObject("base_account") ?: account
        val accountNumber = baseAccount.get("account_number")?.asLong ?: return null
        val sequence = baseAccount.get("sequence")?.asLong ?: return null
        return Triple(accountNumber, sequence, emptyList())
    }

    /**
     * 获取余额；查询失败返回 0（用于展示）。
     */
    suspend fun getBalance(address: String, denom: String = MCParams.DENOM): Long =
        getBalanceOrNull(address, denom) ?: 0L

    /**
     * 获取余额；查询失败返回 null（用于交易前预检，避免把「查询失败」误判为「余额为 0」）。
     */
    suspend fun getBalanceOrNull(address: String, denom: String = MCParams.DENOM): Long? {
        val path = "/cosmos/bank/v1beta1/balances/$address"
        val response = restGet(path) ?: return null
        val balances = response.getAsJsonArray("balances") ?: return null
        for (b in balances) {
            val obj = b.asJsonObject
            if (obj.get("denom")?.asString == denom) {
                return obj.get("amount")?.asLong ?: 0L
            }
        }
        return 0L
    }

    /**
     * 拉取已绑定（BONDED）验证人列表，供委托页选择。
     * REST: /cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED
     */
    suspend fun getValidators(limit: Int = 200): List<ValidatorInfo> {
        val response = restGet(
            "/cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED&pagination.limit=$limit"
        ) ?: return emptyList()
        val validators = response.getAsJsonArray("validators") ?: return emptyList()
        return validators.mapNotNull { element ->
            val obj = element.asJsonObject
            val address = obj.get("operator_address")?.asString ?: return@mapNotNull null
            val moniker = obj.getAsJsonObject("description")?.get("moniker")?.asString.orEmpty()
            val commissionRate = obj.getAsJsonObject("commission")
                ?.getAsJsonObject("commission_rates")
                ?.get("rate")?.asString?.toDoubleOrNull() ?: 0.0
            val tokens = obj.get("tokens")?.asString?.toLongOrNull() ?: 0L
            val jailed = obj.get("jailed")?.asBoolean ?: false
            ValidatorInfo(
                operatorAddress = address,
                moniker = moniker,
                commissionRate = commissionRate,
                tokensUmc = tokens,
                jailed = jailed,
            )
        }
    }

    /**
     * 链上设备认证记录（phonenode.Attestation）。
     * REST: /mcchain/phonenode/attestation/{address}
     */
    suspend fun getAttestation(address: String): ChainAttestation? {
        val response = restGet("/mcchain/phonenode/attestation/$address") ?: return null
        val obj = response.getAsJsonObject("attestation") ?: return null
        return ChainAttestation(
            rootHash = obj.stringOrEmpty("root_hash", "rootHash"),
            nonce = obj.stringOrEmpty("nonce"),
            deviceIdHash = obj.stringOrEmpty("device_id_hash", "deviceIdHash"),
            expirySeconds = obj.longOrZero("expiry"),
            status = obj.stringOrEmpty("status"),
        )
    }

    /**
     * 链上罚没记录（phonenode.Slashes）。
     * REST: /mcchain/phonenode/slashes/{address}
     */
    suspend fun getSlashes(address: String): List<ChainSlash> {
        val response = restGet("/mcchain/phonenode/slashes/$address") ?: return emptyList()
        val records = response.getAsJsonArray("records") ?: return emptyList()
        return records.mapNotNull { element ->
            val obj = element.asJsonObject
            ChainSlash(
                address = obj.stringOrEmpty("address"),
                reason = obj.stringOrEmpty("reason"),
                penaltyBps = obj.get("penalty_bps")?.asInt
                    ?: obj.get("penaltyBps")?.asInt ?: 0,
                timeSeconds = obj.longOrZero("time"),
            )
        }
    }

    /**
     * 节点最低 gas price（umc/gas）。
     * REST: /cosmos/base/node/v1beta1/config -> minimum_gas_prices。
     * 取不到返回 null，调用方回退到内置默认值。
     */
    suspend fun getMinGasPrice(): Double? {
        val response = restGet("/cosmos/base/node/v1beta1/config") ?: return null
        val prices = response.getAsJsonArray("minimum_gas_prices") ?: return null
        for (element in prices) {
            val obj = element.asJsonObject
            if (obj.get("denom")?.asString == MCParams.DENOM) {
                return obj.get("amount")?.asString?.toDoubleOrNull()
            }
        }
        return null
    }

    /**
     * 链上交易历史（LCD /cosmos/tx/v1beta1/txs，按 message.sender 过滤）。
     *
     * 返回 null 表示查询失败（节点未开启交易索引、接口不可达或参数不被支持）；
     * 返回空列表表示查询成功但该地址确实没有记录。两种情况的 UI 文案不同。
     */
    suspend fun getTxHistoryOrNull(address: String, limit: Int = 20): List<ChainTxRecord>? {
        if (address.isBlank()) return null
        val query = java.net.URLEncoder.encode("message.sender='$address'", "UTF-8")
        val response = restGet(
            "/cosmos/tx/v1beta1/txs?query=$query&pagination.limit=$limit&order_by=ORDER_BY_DESC"
        ) ?: return null
        val responses = response.getAsJsonArray("tx_responses") ?: return emptyList()
        return responses.mapNotNull { element ->
            val obj = runCatching { element.asJsonObject }.getOrNull() ?: return@mapNotNull null
            val hash = obj.get("txhash")?.asString ?: return@mapNotNull null
            val messages = obj.getAsJsonObject("tx")
                ?.getAsJsonObject("body")
                ?.getAsJsonArray("messages")
            val firstMessage = messages?.firstOrNull()?.asJsonObject
            ChainTxRecord(
                hash = hash,
                height = obj.get("height")?.asString?.toLongOrNull() ?: 0L,
                timestampMs = parseRfc3339(obj.get("timestamp")?.asString.orEmpty()),
                code = obj.get("code")?.asInt ?: 0,
                typeUrl = firstMessage?.get("@type")?.asString.orEmpty(),
                amountUmc = firstMessage?.coinAmount(MCParams.DENOM),
                counterparty = firstMessage?.counterparty().orEmpty(),
            )
        }
    }

    suspend fun getStakingInfo(address: String): Long {
        val path = "/cosmos/staking/v1beta1/delegations/$address"
        val response = restGet(path) ?: return 0
        var total = 0L
        val delegations = response.getAsJsonArray("delegation_responses")
            ?: response.getAsJsonArray("delegations") ?: return 0
        for (d in delegations) {
            val obj = d.asJsonObject
            val balance = obj.getAsJsonObject("balance") ?: continue
            val amount = balance.get("amount")?.asLong ?: 0
            total += amount
        }
        return total
    }

    /**
     * 获取待领奖励
     */
    suspend fun getPendingRewards(address: String): Long {
        val path = "/cosmos/distribution/v1beta1/delegators/$address/rewards"
        val response = restGet(path) ?: return 0
        var total = 0L
        val rewards = response.getAsJsonArray("rewards") ?: return 0
        for (r in rewards) {
            val obj = r.asJsonObject
            val rewardArray = obj.getAsJsonArray("reward") ?: continue
            for (coin in rewardArray) {
                val coinObj = coin.asJsonObject
                if (coinObj.get("denom").asString == MCParams.DENOM) {
                    // distribution 返回的是 decimal 字符串（如 "123.456000000000000000"），
                    // 不能直接 asLong，否则抛 NumberFormatException。
                    total += coinObj.get("amount")?.asString?.substringBefore('.')?.toLongOrNull() ?: 0L
                }
            }
        }
        return total
    }

    /**
     * 通过 REST `/cosmos/tx/v1beta1/txs` 广播 protobuf TxRaw 字节。
     * 使用 BROADCAST_MODE_SYNC：只做 CheckTx，回执通过 [waitForTx] 轮询。
     */
    suspend fun broadcastTxRaw(txRaw: ByteArray): TxBroadcastResult {
        val payload = gson.toJson(
            mapOf(
                "tx_bytes" to android.util.Base64.encodeToString(txRaw, android.util.Base64.NO_WRAP),
                "mode" to "BROADCAST_MODE_SYNC",
            )
        )
        val response = restPost("/cosmos/tx/v1beta1/txs", payload)
            ?: return TxBroadcastResult("", -1, "无法连接 REST 节点")
        val txResponse = response.getAsJsonObject("tx_response")
            ?: return TxBroadcastResult("", -1, response.toString())

        return TxBroadcastResult(
            hash = txResponse.get("txhash")?.asString ?: "",
            code = txResponse.get("code")?.asInt ?: -1,
            log = txResponse.get("raw_log")?.asString
                ?: txResponse.get("log")?.asString
                ?: "",
            codespace = txResponse.get("codespace")?.asString ?: "",
        )
    }

    /**
     * 轮询交易回执，直到上链或超时（返回 null 表示仍在 mempool / 超时）。
     */
    suspend fun waitForTx(
        txHash: String,
        timeoutMs: Long = 30_000L,
        intervalMs: Long = 2_000L,
    ): TxResult? {
        if (txHash.isBlank()) return null
        val deadline = System.currentTimeMillis() + timeoutMs
        while (System.currentTimeMillis() < deadline) {
            val result = getTxResult(txHash)
            if (result != null) return result
            delay(intervalMs)
        }
        return null
    }

    /**
     * 获取交易结果
     */
    suspend fun getTxResult(txHash: String): TxResult? {
        val response = rpcCall("tx", mapOf(
            "hash" to txHash,
            "prove" to false
        )) ?: return null

        val result = response.getAsJsonObject("result") ?: return null
        val txResult = result.getAsJsonObject("tx_result") ?: return null
        return TxResult(
            hash = result.get("hash")?.asString ?: txHash,
            height = result.get("height")?.asLong ?: 0L,
            code = txResult.get("code")?.asInt ?: -1,
            gasUsed = txResult.get("gas_used")?.asLong ?: 0L,
            gasWanted = txResult.get("gas_wanted")?.asLong ?: 0L,
            log = txResult.get("log")?.asString ?: "",
            codespace = txResult.get("codespace")?.asString ?: "",
        )
    }

    /**
     * 查询委托列表，用于领取全部奖励。
     */
    suspend fun getDelegations(address: String): List<Delegation> {
        val path = "/cosmos/staking/v1beta1/delegations/$address"
        val response = restGet(path) ?: return emptyList()
        val delegations = response.getAsJsonArray("delegation_responses") ?: return emptyList()
        return delegations.mapNotNull { element ->
            val obj = element.asJsonObject
            val delegation = obj.getAsJsonObject("delegation") ?: return@mapNotNull null
            val validator = delegation.get("validator_address")?.asString ?: return@mapNotNull null
            val amount = obj.getAsJsonObject("balance")?.get("amount")?.asLong ?: 0L
            Delegation(validator, amount)
        }
    }

    /**
     * 拉取链上 open 状态的 EdgeAI 任务。
     * REST 形态：/tasks 返回 id 列表，/task/{id} 返回 task_json（snake_case 字段）。
     */
    suspend fun getEdgeAiTasks(limit: Int = 50): List<EdgeAiTask> {
        val body = restGet("/mcchain/edgeai/tasks") ?: return emptyList()
        val ids = body.getAsJsonArray("task_ids") ?: return emptyList()
        return ids.take(limit).mapNotNull { element ->
            val id = runCatching { element.asString }.getOrNull() ?: return@mapNotNull null
            val taskBody = restGet("/mcchain/edgeai/task/$id") ?: return@mapNotNull null
            val taskJson = taskBody.get("task_json")?.asString ?: return@mapNotNull null
            val task = runCatching { gson.fromJson(taskJson, JsonObject::class.java).asJsonObject }
                .getOrNull() ?: return@mapNotNull null
            if (task.get("status")?.asString != "open") return@mapNotNull null
            val description = task.get("description")?.asString ?: ""
            EdgeAiTask(
                taskId = id,
                creator = task.get("creator")?.asString ?: "",
                taskType = description,
                modelId = "",
                inputHash = "",
                inputSizeBytes = description.toByteArray().size.toLong(),
                reward = task.get("reward")?.asLong ?: 0L,
                maxSubmissions = 1,
                submissionCount = 0,
                deadlineHeight = (task.get("created_at_block")?.asLong ?: 0L) + EDGEAI_TASK_EXPIRE_BLOCKS,
                disputeDeadline = 0L,
                status = AiTaskStatus.OPEN,
                createdAt = (task.get("created_at")?.asLong ?: 0L) * 1000L,
            )
        }
    }

    // === 内部方法 ===

    private suspend fun rpcCall(
        method: String,
        params: Map<String, Any>,
        target: String? = null,
    ): JsonObject? =
        withContext(Dispatchers.IO) {
            val requestObj = mapOf(
                "jsonrpc" to "2.0",
                "id" to 1,
                "method" to method,
                "params" to params
            )
            val body = gson.toJson(requestObj).toRequestBody(JSON)
            val url = target?.trim()?.trimEnd('/')?.takeIf { it.isNotBlank() } ?: endpoint
            val request = Request.Builder()
                .url(url)
                .post(body)
                .build()

            try {
                client.newCall(request).await().use { response ->
                    if (!response.isSuccessful) return@withContext null
                    response.body?.string()?.let { gson.fromJson(it, JsonObject::class.java) }
                }
            } catch (e: Exception) {
                null
            }
        }

    private suspend fun restGet(path: String): JsonObject? = withContext(Dispatchers.IO) {
        val request = Request.Builder().url("${restBaseUrl()}$path").get().build()
        try {
            client.newCall(request).await().use { response ->
                if (!response.isSuccessful) return@withContext null
                response.body?.string()?.let { gson.fromJson(it, JsonObject::class.java) }
            }
        } catch (e: Exception) {
            null
        }
    }

    private suspend fun restPost(path: String, jsonBody: String): JsonObject? = withContext(Dispatchers.IO) {
        val request = Request.Builder()
            .url("${restBaseUrl()}$path")
            .post(jsonBody.toRequestBody(JSON))
            .build()
        try {
            client.newCall(request).await().use { response ->
                if (!response.isSuccessful) return@withContext null
                response.body?.string()?.let { gson.fromJson(it, JsonObject::class.java) }
            }
        } catch (e: Exception) {
            null
        }
    }

    private fun restBaseUrl(): String = try {
        val uri = java.net.URI(endpoint)
        val apiPort = if (uri.port > 0) {
            uri.port - (MCParams.DEFAULT_RPC_PORT - MCParams.DEFAULT_API_PORT)
        } else {
            MCParams.DEFAULT_API_PORT
        }
        java.net.URI(uri.scheme, uri.userInfo, uri.host, apiPort, uri.path, uri.query, uri.fragment)
            .toString()
            .trimEnd('/')
    } catch (e: Exception) {
        endpoint.replace(":${MCParams.DEFAULT_RPC_PORT}", ":${MCParams.DEFAULT_API_PORT}")
    }

    private suspend fun Call.await(): Response = suspendCancellableCoroutine { cont ->
        enqueue(object : Callback {
            override fun onResponse(call: Call, response: Response) {
                cont.resume(response, {})
            }
            override fun onFailure(call: Call, e: IOException) {
                cont.resumeWith(Result.failure(e))
            }
        })
        cont.invokeOnCancellation { cancel() }
    }

    private fun parseIsoTime(isoTime: String): Long {
        return try {
            java.time.Instant.parse(isoTime).epochSecond
        } catch (e: Exception) {
            System.currentTimeMillis() / 1000
        }
    }

    private companion object {
        /** x/edgeai/types/keys.go TaskExpireBlocks：超时未结算任务过期退托管。 */
        const val EDGEAI_TASK_EXPIRE_BLOCKS = 10_000L
    }
}

data class RpcCoin(val denom: String, val amount: Long)

data class TxBroadcastResult(
    val hash: String,
    val code: Int,
    val log: String,
    val codespace: String = "",
)

data class TxResult(
    val hash: String,
    val height: Long,
    val code: Int,
    val gasUsed: Long,
    val gasWanted: Long,
    val log: String,
    val codespace: String = "",
)

data class Delegation(
    val validatorAddress: String,
    val amount: Long
)

data class ValidatorInfo(
    val operatorAddress: String,
    val moniker: String,
    val commissionRate: Double,
    val tokensUmc: Long,
    val jailed: Boolean,
)

data class ChainAttestation(
    val rootHash: String,
    val nonce: String,
    val deviceIdHash: String,
    val expirySeconds: Long,
    val status: String,
)

data class ChainSlash(
    val address: String,
    val reason: String,
    val penaltyBps: Int,
    val timeSeconds: Long,
)

/** 一条已上链的交易记录（只包含 LCD 直接返回的字段）。 */
data class ChainTxRecord(
    val hash: String,
    val height: Long,
    val timestampMs: Long,
    val code: Int,
    val typeUrl: String,
    val amountUmc: Long?,
    val counterparty: String,
) {
    val success: Boolean get() = code == 0
}

/** 兼容 gRPC-gateway 的 snake_case 与 camelCase 两种 JSON 字段名。 */
private fun JsonObject.stringOrEmpty(vararg keys: String): String {
    for (key in keys) {
        get(key)?.asString?.let { return it }
    }
    return ""
}

private fun JsonObject.longOrZero(key: String): Long = get(key)?.asLong ?: 0L

/** 提取消息中的 umc 金额；amount 既可能是 Coin 对象，也可能是 Coin 数组。 */
private fun JsonObject.coinAmount(denom: String): Long? {
    val node = get("amount") ?: return null
    val coin = when {
        node.isJsonArray -> node.asJsonArray.firstOrNull { it.isJsonObject }?.asJsonObject
        node.isJsonObject -> node.asJsonObject
        else -> null
    } ?: return null
    if (coin.get("denom")?.asString != denom) return null
    return coin.get("amount")?.asString?.toLongOrNull() ?: coin.get("amount")?.asLong
}

/** 交易对手：转账取收款地址，质押类消息取验证人地址。 */
private fun JsonObject.counterparty(): String {
    for (key in listOf("to_address", "validator_address", "destination_validator_address")) {
        get(key)?.asString?.takeIf { it.isNotBlank() }?.let { return it }
    }
    return ""
}

/** 解析 LCD 返回的 RFC3339 时间；解析失败返回 0，UI 显示「—」。 */
private fun parseRfc3339(value: String): Long = runCatching {
    java.time.Instant.parse(value).toEpochMilli()
}.getOrDefault(0L)
