package com.mcchain.miner.network

import com.google.gson.Gson
import com.google.gson.JsonObject
import com.mcchain.miner.data.pref.SecurePrefs
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import java.net.URLEncoder
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

/**
 * mcchain-indexer 只读 HTTP 客户端。
 *
 * 索引器是独立后端进程（`mcchain-indexer serve`），App 只按地址查询已落盘的
 * depin 贡献事件；未配置地址时明确返回 NotConfigured，而不是展示 0 或示例数据。
 */
@Singleton
class IndexerClient @Inject constructor(
    private val securePrefs: SecurePrefs,
) {
    sealed interface ContributionsResult {
        /** 用户尚未配置索引器地址。 */
        data object NotConfigured : ContributionsResult

        /** 查询成功；total 是该地址在索引器中的全部匹配条数（用于分页）。 */
        data class Success(val events: List<IndexerEvent>, val total: Int) : ContributionsResult

        data class Failure(val message: String) : ContributionsResult
    }

    /** 索引器返回的单条链上事件。 */
    data class IndexerEvent(
        val height: Long,
        val type: String,
        val attrs: Map<String, String>,
    )

    private val client = OkHttpClient.Builder()
        .connectTimeout(5, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .build()

    private val gson = Gson()

    private fun endpointOrNull(): String? = normalize(null)

    /** 允许测试对话框传入未保存的地址；传 null 时读取已保存配置。 */
    private fun normalize(raw: String?): String? {
        val value = raw ?: runCatching { securePrefs.indexerEndpoint }.getOrNull() ?: return null
        return value.trim().trimEnd('/').takeIf { it.isNotBlank() }
    }

    /** 查询某地址的贡献事件（depin.RewardPaid / depin.DefenseBlocked），按高度倒序。 */
    suspend fun getContributions(
        address: String,
        limit: Int = 20,
        offset: Int = 0,
    ): ContributionsResult = withContext(Dispatchers.IO) {
        val base = endpointOrNull() ?: return@withContext ContributionsResult.NotConfigured
        if (!base.startsWith("http://") && !base.startsWith("https://")) {
            return@withContext ContributionsResult.Failure("索引器地址必须以 http:// 或 https:// 开头")
        }
        val encoded = URLEncoder.encode(address, "UTF-8")
        val url = "$base/v1/contributions?address=$encoded&limit=$limit&offset=$offset"
        val request = Request.Builder().url(url).get().build()

        runCatching {
            client.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    return@withContext ContributionsResult.Failure(
                        "索引器返回 HTTP ${response.code}，请确认已启动 mcchain-indexer serve"
                    )
                }
                val body = response.body?.string().orEmpty()
                val root = gson.fromJson(body, JsonObject::class.java)
                val array = root.getAsJsonArray("events")
                    ?: return@withContext ContributionsResult.Success(emptyList(), 0)
                val events = array.mapNotNull { element ->
                    runCatching { element.asJsonObject }.getOrNull()?.toIndexerEvent()
                }
                val total = runCatching { root.get("total").asInt }.getOrDefault(events.size)
                ContributionsResult.Success(events, total)
            }
        }.getOrElse {
            ContributionsResult.Failure(
                "无法连接索引器：${it.message ?: it.javaClass.simpleName}"
            )
        }
    }

    /** 探测索引器 /health，用于设置页的连通性测试；target 为空时使用已保存地址。 */
    suspend fun ping(target: String? = null): Boolean = withContext(Dispatchers.IO) {
        val base = normalize(target) ?: return@withContext false
        if (!base.startsWith("http://") && !base.startsWith("https://")) return@withContext false
        runCatching {
            val request = Request.Builder().url("$base/health").get().build()
            client.newCall(request).execute().use { it.isSuccessful }
        }.getOrDefault(false)
    }

    private fun JsonObject.toIndexerEvent(): IndexerEvent {
        val attrsObject = runCatching { getAsJsonObject("attrs") }.getOrNull()
        val attrs = attrsObject?.entrySet()?.associate { (key, value) ->
            key to runCatching { value.asString }.getOrDefault("")
        }.orEmpty()
        return IndexerEvent(
            height = runCatching { get("height").asLong }.getOrDefault(0L),
            type = runCatching { get("type").asString }.getOrDefault(""),
            attrs = attrs,
        )
    }
}
