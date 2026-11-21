package com.mcchain.miner.domain.node

import android.content.Context
import android.os.Build
import android.provider.Settings
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.domain.wallet.WalletManager
import com.mcchain.miner.network.OracleClient
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.util.AttestationPolicy
import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Sha256
import dagger.hilt.android.qualifiers.ApplicationContext
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 设备身份与硬件认证（Phase 2）。
 *
 * 流程：
 *  1. challenge = SHA256(chainId|deviceId|timestamp|random)，一次性使用（链上会记录消费）；
 *  2. 在 AndroidKeyStore 生成绑定 `deviceAddr|challenge` 的密钥（StrongBox 优先，回退 TEE）；
 *  3. 把证书链 PEM 提交给认证预言机 `/sign`，由预言机验证可信根后对 `deviceAddr|challenge` 签名；
 *  4. 本地用预言机公钥验签，返回可直接用于 depin.MsgAttestDevice 的 challenge + signature。
 *
 * fail-closed：设备只提供软件级证明（无 TEE/StrongBox）、证书 challenge 不匹配、
 * 预言机不可用或验签失败时，一律返回失败，绝不产生「本地成功、链上失败」的假认证。
 */
@Singleton
class DeviceAttestation @Inject constructor(
    @ApplicationContext private val context: Context,
    private val securePrefs: SecurePrefs,
    private val keyAttestation: AndroidKeyAttestation,
    private val oracleClient: OracleClient,
    private val walletManager: WalletManager,
) {
    /**
     * 获取或创建设备唯一标识
     */
    fun getDeviceId(): String {
        securePrefs.deviceId?.let { return it }

        // 组合 Android ID + 硬件序列号生成设备指纹
        val androidId = Settings.Secure.getString(
            context.contentResolver, Settings.Secure.ANDROID_ID
        ) ?: "unknown"

        val hardwareId = "${Build.MANUFACTURER}_${Build.MODEL}_${Build.HARDWARE}"
        val combined = "$androidId|$hardwareId"
        val hash = java.security.MessageDigest.getInstance("SHA-256")
            .digest(combined.toByteArray())
            .joinToString("") { "%02x".format(it) }
            .take(40)

        val deviceId = "phone_$hash"
        securePrefs.deviceId = deviceId
        return deviceId
    }

    /**
     * 执行硬件认证。
     *
     * @param deviceAddress 钱包地址（链上设备身份，必须与交易签名者一致）
     */
    suspend fun performAttestation(deviceAddress: String): AttestationResult {
        val deviceId = getDeviceId()
        if (!deviceAddress.startsWith(MCParams.BECH32_PREFIX + "1")) {
            return failure(deviceId, "", "设备地址无效：请先创建或导入钱包")
        }

        val challenge = newChallenge(deviceId)
        val alias = "mc-attest-${deviceId.take(24)}"
        val attested = try {
            keyAttestation.generate(alias, "$deviceAddress|$challenge".toByteArray(Charsets.UTF_8))
        } catch (e: KeyAttestationException) {
            return failure(deviceId, challenge, e.message ?: "硬件认证密钥生成失败")
        } catch (e: Exception) {
            return failure(deviceId, challenge, "硬件认证失败：${e.message ?: e.javaClass.simpleName}")
        }

        if (attested.securityLevel == AndroidKeyAttestation.SecurityLevel.SOFTWARE) {
            keyAttestation.deleteKey(alias)
            return failure(
                deviceId,
                challenge,
                "设备未提供硬件级证明（securityLevel=software）：需要 TEE 或 StrongBox 才能上链",
            )
        }

        val signature = try {
            oracleClient.requestSignature(
                OracleClient.AttestationClaim(
                    root = "android_key_attestation",
                    payload = attested.certificateChainPem,
                    challenge = challenge,
                    deviceAddr = deviceAddress,
                )
            )
        } catch (e: Exception) {
            return failure(deviceId, challenge, e.message ?: "预言机认证失败")
        }

        if (!oracleClient.verifySignature(signature)) {
            return failure(deviceId, challenge, "预言机签名本地验签失败，已拒绝提交（疑似中间人）")
        }

        securePrefs.attestationToken = challenge
        securePrefs.attestationExpiry = System.currentTimeMillis() +
            MCParams.PHONENODE_ATTESTATION_VALIDITY_SECONDS * 1000

        // phonenode 自签 attestation 材料（depin 防线 Layer2 前置）：
        // 设备钱包私钥对 "deviceIdHash|selfNonce" 签名，链上用注册公钥验签并绑定。
        val wallet = walletManager.getStoredWallet()
            ?: return failure(deviceId, challenge, "钱包密钥不可用，无法生成设备自签认证")
        val deviceIdHash = Hex.encode(Sha256.digest(deviceId.toByteArray(Charsets.UTF_8)))
        val selfNonce = UUID.randomUUID().toString().replace("-", "")
        val selfSignatureHex = Hex.encode(
            Secp256k1Signer(wallet.privateKey)
                .signDigest(Sha256.digest("$deviceIdHash|$selfNonce".toByteArray(Charsets.UTF_8)))
        )

        return AttestationResult(
            success = true,
            token = challenge,
            deviceId = deviceId,
            nonce = challenge,
            challenge = challenge,
            signatureBase64 = signature.signatureBase64,
            securityLevel = attested.securityLevel.name,
            selfNonce = selfNonce,
            deviceIdHash = deviceIdHash,
            devicePubKeyHex = Hex.encode(wallet.publicKey),
            selfSignatureHex = selfSignatureHex,
            rootHash = "android_key_attestation|${attested.securityLevel.name.lowercase()}|${deviceIdHash.take(16)}",
        )
    }

    /**
     * 检查认证是否有效（仅本地有效期，链上状态以 depin 设备记录为准）。
     */
    fun isAttestationValid(): Boolean {
        val expiry = securePrefs.attestationExpiry
        val token = securePrefs.attestationToken
        return AttestationPolicy.isValid(token, expiry, System.currentTimeMillis())
    }

    /**
     * 检查设备是否满足最低硬件要求
     */
    fun checkHardwareRequirements(): HardwareCheck {
        val totalRamGb = getTotalRamGb()
        val availableStorageGb = getAvailableStorageGb()
        val cpuCores = Runtime.getRuntime().availableProcessors()

        val issues = mutableListOf<String>()
        if (totalRamGb < 4) issues.add("RAM 不足（至少 4GB，当前 ${totalRamGb}GB）")
        if (availableStorageGb < 8) issues.add("存储空间不足（至少 8GB，当前 ${availableStorageGb}GB）")
        if (cpuCores < 4) issues.add("CPU 核心数不足（至少 4 核，当前 $cpuCores 核）")

        return HardwareCheck(
            passed = issues.isEmpty(),
            ramGb = totalRamGb,
            storageGb = availableStorageGb,
            cpuCores = cpuCores,
            issues = issues
        )
    }

    /** 每次认证使用新的 challenge；链上按 (device, challenge) 记录消费，重放会被拒绝。 */
    private fun newChallenge(deviceId: String): String {
        val seed = "${MCParams.CHAIN_ID}|$deviceId|${System.currentTimeMillis()}|${UUID.randomUUID()}"
        return Hex.encode(Sha256.digest(seed.toByteArray(Charsets.UTF_8)))
    }

    private fun failure(deviceId: String, challenge: String, message: String) = AttestationResult(
        success = false,
        token = "",
        deviceId = deviceId,
        nonce = challenge,
        challenge = challenge,
        error = message,
    )

    private fun getTotalRamGb(): Int {
        val memInfo = android.app.ActivityManager.MemoryInfo()
        val am = context.getSystemService(Context.ACTIVITY_SERVICE) as android.app.ActivityManager
        am.getMemoryInfo(memInfo)
        return (memInfo.totalMem / (1024 * 1024 * 1024)).toInt()
    }

    private fun getAvailableStorageGb(): Long {
        val stat = android.os.StatFs(context.filesDir.absolutePath)
        return stat.availableBlocksLong * stat.blockSizeLong / (1024 * 1024 * 1024)
    }
}

data class AttestationResult(
    val success: Boolean,
    val token: String,
    val deviceId: String,
    val nonce: String,
    val error: String? = null,
    /** 本次认证的链上 challenge（一次性）。 */
    val challenge: String = "",
    /** 预言机对 deviceAddr|challenge 的 secp256k1 签名（base64），作为 MsgAttestDevice.signature。 */
    val signatureBase64: String = "",
    /** TEE / STRONGBOX / SOFTWARE。 */
    val securityLevel: String = "",
    /** phonenode 自签 attestation：一次性 nonce（链上重放索引）。 */
    val selfNonce: String = "",
    /** SHA256(deviceId) 小写 hex，链上设备绑定键。 */
    val deviceIdHash: String = "",
    /** 钱包压缩公钥 hex（33 字节）。 */
    val devicePubKeyHex: String = "",
    /** 设备对 "deviceIdHash|selfNonce" 的 secp256k1 签名 hex（64 字节 r||s）。 */
    val selfSignatureHex: String = "",
    /** attestation 根留痕（硬件来源标识，链上仅存字符串）。 */
    val rootHash: String = "",
)

data class HardwareCheck(
    val passed: Boolean,
    val ramGb: Int,
    val storageGb: Long,
    val cpuCores: Int,
    val issues: List<String>
)
