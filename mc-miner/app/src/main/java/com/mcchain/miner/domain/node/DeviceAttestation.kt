package com.mcchain.miner.domain.node

import android.content.Context
import android.os.Build
import android.provider.Settings
import com.mcchain.miner.BuildConfig
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.pref.SecurePrefs
import dagger.hilt.android.qualifiers.ApplicationContext
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 设备身份与硬件认证。
 *
 * Phase 0 现状：硬件证明尚未接入（Android Key Attestation / Play Integrity 均未调用）。
 * - debug 构建返回显式标记 `simulated=true` 的桩结果，便于 UI 联调；
 * - release 构建一律返回失败，禁止把本地哈希冒充硬件证明，也禁止写入「已认证」状态。
 * 真实实现见开发方案 Phase 2（证书链 + nonce + oracle /attest）。
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
     * 执行设备认证。
     *
     * 注意：当前不是硬件证明。release 构建直接失败（fail-closed），
     * debug 构建返回 `simulated=true` 的桩，调用方必须在 UI 上标注「模拟」。
     */
    suspend fun performAttestation(): AttestationResult {
        val deviceId = getDeviceId()
        if (!BuildConfig.DEBUG) {
            return AttestationResult(
                success = false,
                token = "",
                deviceId = deviceId,
                nonce = "",
                error = "硬件证明未接入（见 STATUS.md / 移动端 Phase 2）"
            )
        }

        val nonce = UUID.randomUUID().toString()
        val debugToken = "debug-simulated-${UUID.randomUUID()}"
        securePrefs.attestationToken = debugToken
        securePrefs.attestationExpiry = System.currentTimeMillis() +
                MCParams.PHONENODE_ATTESTATION_VALIDITY_SECONDS * 1000
        return AttestationResult(
            success = true,
            token = debugToken,
            deviceId = deviceId,
            nonce = nonce,
            simulated = true
        )
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
    val error: String? = null,
    val simulated: Boolean = false
)

data class HardwareCheck(
    val passed: Boolean,
    val ramGb: Int,
    val storageGb: Long,
    val cpuCores: Int,
    val issues: List<String>
)
