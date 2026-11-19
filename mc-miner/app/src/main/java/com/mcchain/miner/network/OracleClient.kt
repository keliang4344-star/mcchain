package com.mcchain.miner.network

import com.google.gson.Gson
import com.google.gson.JsonObject
import com.google.gson.JsonParser
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.shared.crypto.Secp256k1Verifier
import com.mcchain.miner.shared.util.Sha256
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import okhttp3.Call
import okhttp3.Callback
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.resume
import kotlin.coroutines.resumeWithException

/** 与该预言机交互失败（未配置、网络错误、验证拒绝）。消息可直接展示给用户。 */
class OracleException(message: String) : Exception(message)

/**
 * 认证预言机客户端。
 *
 * 对齐 `internal/oraclesvc` 的 `/sign` 契约：
 *   POST {device_addr, challenge, attestation:{root,payload,challenge,device_addr}}
 *   → {device_addr, challenge, signature_base64}
 * 预言机先验证硬件 attestation（Android Key Attestation 证书链 + challenge 绑定），
 * 再对 `deviceAddr|challenge` 签名；该签名随后作为 depin.MsgAttestDevice 的 signature 上链，
 * 由链上 TeeOracle 验签。
 *
 * 客户端会拉取 `/pubkey` 并对返回签名做本地验签，避免中间人替换签名后白付手续费。
 */
@Singleton
class OracleClient @Inject constructor(
    private val gson: Gson,
    private val securePrefs: SecurePrefs,
) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(20, TimeUnit.SECONDS)
        .build()

    private val JSON = "application/json; charset=utf-8".toMediaType()

    data class AttestationClaim(
        val root: String,
        val payload: String,
        val challenge: String,
        val deviceAddr: String,
    )

    data class OracleSignature(
        val deviceAddr: String,
        val challenge: String,
        val signatureBase64: String,
        val oracleAddress: String,
        val publicKeyBase64: String,
    )

    @Volatile
    private var cachedPubKey: String? = null

    /** 请求预言机对 deviceAddr|challenge 签名（内部先提交硬件证明）。 */
    suspend fun requestSignature(claim: AttestationClaim): OracleSignature {
        val base = endpoint()
        val body = mapOf(
            "device_addr" to claim.deviceAddr,
            "challenge" to claim.challenge,
            "attestation" to mapOf(
                "root" to claim.root,
                "payload" to claim.payload,
                "challenge" to claim.challenge,
                "device_addr" to claim.deviceAddr,
            ),
        )
        val response = postJson("$base/sign", gson.toJson(body))
        val signature = response["signature_base64"]?.asString
            ?: throw OracleException("预言机返回缺少 signature_base64")
        val pubKey = cachedPubKey ?: fetchPubKey(base)
        return OracleSignature(
            deviceAddr = response["device_addr"]?.asString ?: claim.deviceAddr,
            challenge = response["challenge"]?.asString ?: claim.challenge,
            signatureBase64 = signature,
            oracleAddress = "",
            publicKeyBase64 = pubKey,
        )
    }

    /** 本地校验预言机签名（secp256k1 对 SHA256(deviceAddr|challenge)）。 */
    fun verifySignature(signature: OracleSignature): Boolean {
        val pubKey = runCatching {
            android.util.Base64.decode(signature.publicKeyBase64, android.util.Base64.NO_WRAP)
        }.getOrNull() ?: return false
        val raw = runCatching {
            android.util.Base64.decode(signature.signatureBase64, android.util.Base64.NO_WRAP)
        }.getOrNull() ?: return false
        val digest = Sha256.digest(
            (signature.deviceAddr + "|" + signature.challenge).toByteArray(Charsets.UTF_8)
        )
        return Secp256k1Verifier.verify(pubKey, digest, raw)
    }

    private suspend fun fetchPubKey(base: String): String = withContext(Dispatchers.IO) {
        val response = request("$base/pubkey", "GET", null)
        val pub = response["pubkey_base64"]?.asString
            ?: throw OracleException("预言机 /pubkey 返回缺少 pubkey_base64")
        cachedPubKey = pub
        pub
    }

    private fun endpoint(): String {
        val raw = runCatching { securePrefs.oracleEndpoint }.getOrNull().orEmpty()
        val normalized = raw.trim().trimEnd('/')
        if (normalized.isEmpty()) throw OracleException("未配置认证预言机地址（设置 → 认证预言机）")
        return normalized
    }

    private suspend fun postJson(url: String, body: String): JsonObject =
        request(url, "POST", body)

    private suspend fun request(url: String, method: String, body: String?): JsonObject =
        withContext(Dispatchers.IO) {
            val builder = Request.Builder().url(url).header("Accept", "application/json")
            val token = runCatching { securePrefs.oracleToken }.getOrNull().orEmpty()
            if (token.isNotBlank()) builder.header("Authorization", "Bearer $token")
            when (method) {
                "POST" -> builder.post(body.orEmpty().toRequestBody(JSON))
                else -> builder.get()
            }

            val response = try {
                client.newCall(builder.build()).await()
            } catch (e: Exception) {
                throw OracleException("无法连接预言机：${e.message}")
            }
            response.use {
                val text = it.body?.string().orEmpty()
                if (!it.isSuccessful) {
                    // 预言机拒绝时返回 403 + 原因，直接透出便于定位（证书链/challenge/根不受信）。
                    throw OracleException("预言机拒绝（HTTP ${it.code}）：${text.take(300)}")
                }
                runCatching { JsonParser.parseString(text).asJsonObject }
                    .getOrElse { throw OracleException("预言机返回非法 JSON：${text.take(200)}") }
            }
        }

    private suspend fun Call.await(): Response = suspendCancellableCoroutine { cont ->
        enqueue(object : Callback {
            override fun onResponse(call: Call, response: Response) {
                cont.resume(response)
            }

            override fun onFailure(call: Call, e: IOException) {
                cont.resumeWithException(e)
            }
        })
        cont.invokeOnCancellation { cancel() }
    }
}
