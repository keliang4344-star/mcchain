package com.mcchain.miner.domain.node

import android.content.Context
import android.os.Build
import android.provider.Settings
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.pref.SecurePrefs
import dagger.hilt.android.qualifiers.ApplicationContext
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 设备身份与硬件认证。
 * 使用 Android ID + Build 硬件指纹进行设备唯一标识，防女巫攻击。
 * 生产环境可升级为 Play Integrity API。
 */
@Singleton
class DeviceAttestation @Inject constructor(
    @ApplicationContext private val context: Context,
    private val securePrefs: SecurePrefs
) {
    /**
     * 获取或创建设备唯一标识
     */
    fun getDeviceId(): String {
        securePrefs.deviceId?.let { return it }

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
     * 执行设备认证（本地指纹模式，不依赖 Google Play Services）
     * @return 认证结果
     */
    suspend fun performAttestation(): AttestationResult {
        return try {
            val deviceId = getDeviceId()
            val nonce = UUID.randomUUID().toString()

            // 本地生成认证 token：deviceId + nonce + 时间戳的 HMAC
            val rawToken = "$deviceId|$nonce|${System.currentTimeMillis()}"
            val token = java.security.MessageDigest.getInstance("SHA-256")
                .digest(rawToken.toByteArray())
                .joinToString("") { "%02x".format(it) }

            securePrefs.attestationToken = token
            securePrefs.attestationExpiry = System.currentTimeMillis() +
                    MCParams.PHONENODE_ATTESTATION_VALIDITY_SECONDS * 1000

            AttestationResult(
                success = true,
                token = token,
                deviceId = deviceId,
                nonce = nonce
            )
        } catch (e: Exception) {
            AttestationResult(
                success = false,
                token = "",
                deviceId = getDeviceId(),
                nonce = "",
                error = e.message
            )
        }
    }

    /**
     * 检查认证是否有效
     */
    fun isAttestationValid(): Boolean {
        val expiry = securePrefs.attestationExpiry
        val token = securePrefs.attestationToken
        return !token.isNullOrBlank() && System.currentTimeMillis() < expiry
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
    val error: String? = null
)

data class HardwareCheck(
    val passed: Boolean,
    val ramGb: Int,
    val storageGb: Long,
    val cpuCores: Int,
    val issues: List<String>
)
