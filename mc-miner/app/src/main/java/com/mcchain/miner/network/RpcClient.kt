package com.mcchain.miner.network

import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.*
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
    private val gson: Gson
) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(10, java.util.concurrent.TimeUnit.SECONDS)
        .readTimeout(30, java.util.concurrent.TimeUnit.SECONDS)
        .build()

    private val JSON = "application/json; charset=utf-8".toMediaType()
    private var endpoint: String = "http://127.0.0.1:${MCParams.DEFAULT_RPC_PORT}"

    private val _latestHeight = MutableStateFlow(0L)
    val latestHeight: StateFlow<Long> = _latestHeight

    fun setEndpoint(url: String) {
        endpoint = url.trimEnd('/')
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
     * 获取节点状态
     */
    suspend fun getNodeStatus(): NodeStatusSnapshot? {
        val response = rpcCall("status", mapOf()) ?: return null
        val result = response.getAsJsonObject("result")
        val syncInfo = result.getAsJsonObject("sync_info")
        val nodeInfo = result.getAsJsonObject("node_info")

        return NodeStatusSnapshot(
            currentHeight = syncInfo.get("latest_block_height").asLong,
            latestBlockTime = parseIsoTime(syncInfo.get("latest_block_time").asString),
            syncedPeers = result.getAsJsonObject("sync_info")?.get("n_peers")?.asInt ?: 0,
            chainId = nodeInfo.get("network").asString,
            nodeVersion = nodeInfo.get("version").asString,
            catchingUp = syncInfo.get("catching_up").asBoolean,
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
        val account = response.getAsJsonObject("account")
            ?: response.getAsJsonObject("account")
            ?: return null
        val baseAccount = account.getAsJsonObject("base_account") ?: account
        val accountNumber = baseAccount.get("account_number").asLong
        val sequence = baseAccount.get("sequence").asLong
        return Triple(accountNumber, sequence, emptyList())
    }

    /**
     * 获取余额
     */
    suspend fun getBalance(address: String, denom: String = MCParams.DENOM): Long {
        val path = "/cosmos/bank/v1beta1/balances/$address"
        val response = restGet(path) ?: return 0
        val balances = response.getAsJsonArray("balances")
        for (b in balances) {
            val obj = b.asJsonObject
            if (obj.get("denom").asString == denom) {
                return obj.get("amount").asLong
            }
        }
        return 0
    }

    /**
     * 获取质押信息
     */
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
                    total += coinObj.get("amount").asLong
                }
            }
        }
        return total
    }

    /**
     * 广播交易
     */
    suspend fun broadcastTx(txJson: String): TxBroadcastResult {
        val body = mapOf(
            "jsonrpc" to "2.0",
            "id" to 1,
            "method" to "broadcast_tx_sync",
            "params" to mapOf("tx" to android.util.Base64.encodeToString(
                txJson.toByteArray(), android.util.Base64.NO_WRAP
            ))
        )

        val response = rpcCallRaw(gson.toJson(body))
        val result = gson.fromJson(response, JsonObject::class.java)
        val resObj = result.getAsJsonObject("result")

        return TxBroadcastResult(
            hash = resObj?.get("hash")?.asString ?: "",
            code = resObj?.get("code")?.asInt ?: -1,
            log = resObj?.get("log")?.asString ?: ""
        )
    }

    /**
     * 获取交易结果
     */
    suspend fun getTxResult(txHash: String): TxResult? {
        val response = rpcCall("tx", mapOf(
            "hash" to txHash,
            "prove" to false
        )) ?: return null

        val result = response.getAsJsonObject("result")
        val txResult = result.getAsJsonObject("tx_result")
        return TxResult(
            hash = result.get("hash").asString,
            height = result.get("height").asLong,
            code = txResult.get("code").asInt,
            gasUsed = txResult.get("gas_used").asLong,
            gasWanted = txResult.get("gas_wanted").asLong,
            log = txResult.get("log")?.asString ?: ""
        )
    }

    // === 内部方法 ===

    private suspend fun rpcCall(method: String, params: Map<String, Any>): JsonObject? =
        withContext(Dispatchers.IO) {
            val requestObj = mapOf(
                "jsonrpc" to "2.0",
                "id" to 1,
                "method" to method,
                "params" to params
            )
            val body = gson.toJson(requestObj).toRequestBody(JSON)
            val request = Request.Builder()
                .url(endpoint)
                .post(body)
                .build()

            try {
                val response = client.newCall(request).await()
                response.body?.string()?.let { gson.fromJson(it, JsonObject::class.java) }
            } catch (e: Exception) {
                null
            }
        }

    private suspend fun rpcCallRaw(jsonBody: String): String = withContext(Dispatchers.IO) {
        val request = Request.Builder()
            .url(endpoint)
            .post(jsonBody.toRequestBody(JSON))
            .build()
        try {
            client.newCall(request).await().body?.string() ?: "{}"
        } catch (e: Exception) {
            "{}"
        }
    }

    private suspend fun restGet(path: String): JsonObject? = withContext(Dispatchers.IO) {
        val baseUrl = endpoint.replace(":${MCParams.DEFAULT_RPC_PORT}", ":${MCParams.DEFAULT_API_PORT}")
        val url = "$baseUrl$path"
        val request = Request.Builder().url(url).get().build()
        try {
            val response = client.newCall(request).await()
            response.body?.string()?.let { gson.fromJson(it, JsonObject::class.java) }
        } catch (e: Exception) {
            null
        }
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
}

data class RpcCoin(val denom: String, val amount: Long)

data class TxBroadcastResult(
    val hash: String,
    val code: Int,
    val log: String
)

data class TxResult(
    val hash: String,
    val height: Long,
    val code: Int,
    val gasUsed: Long,
    val gasWanted: Long,
    val log: String
)
