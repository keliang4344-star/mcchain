package com.mcchain.miner.data.pref

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 加密偏好存储。
 *
 * 安全约束（fail-closed）：
 *  - 钱包助记词/私钥只允许写入 EncryptedSharedPreferences（MasterKey 由 Android Keystore 保护）；
 *  - 若 Keystore / EncryptedSharedPreferences 初始化失败，**不得**回退到明文 SharedPreferences——
 *    那等于把助记词以明文落盘，一次备份/root 即可全量失窃；
 *  - 初始化失败时 prefs 为 null，任何读写密钥的操作都会抛出明确异常，由 UI 提示用户，
 *    而不是静默降级。
 */
@Singleton
class SecurePrefs @Inject constructor(
    @ApplicationContext context: Context
) {
    private val prefs: SharedPreferences? = try {
        val masterKey = MasterKey.Builder(context)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        EncryptedSharedPreferences.create(
            context,
            "keystore",
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
        )
    } catch (e: Exception) {
        // 明确记录并拒绝明文回退；是否禁用钱包由调用方根据 isSecureStorageAvailable 决定。
        android.util.Log.e("SecurePrefs", "EncryptedSharedPreferences unavailable; refusing plaintext fallback", e)
        null
    }

    /** 加密存储是否可用。为 false 时禁止生成/导入/读取钱包密钥。 */
    val isSecureStorageAvailable: Boolean
        get() = prefs != null

    private fun requirePrefs(): SharedPreferences = prefs ?: throw IllegalStateException(
        "Secure storage unavailable: Android Keystore/EncryptedSharedPreferences could not be initialised; " +
            "refusing to read or write wallet secrets in plaintext"
    )

    // === 钱包密钥 ===
    var encryptedMnemonic: String?
        get() = requirePrefs().getString(KEY_MNEMONIC, null)
        set(value) = requirePrefs().edit().putString(KEY_MNEMONIC, value).apply()

    var encryptedPrivateKey: String?
        get() = requirePrefs().getString(KEY_PRIVATE_KEY, null)
        set(value) = requirePrefs().edit().putString(KEY_PRIVATE_KEY, value).apply()

    // === 节点配置 ===
    var seedNodes: Set<String>
        get() = requirePrefs().getStringSet(KEY_SEED_NODES, emptySet()) ?: emptySet()
        set(value) = requirePrefs().edit().putStringSet(KEY_SEED_NODES, value).apply()

    var rpcEndpoint: String
        get() = requirePrefs().getString(KEY_RPC_ENDPOINT, "") ?: ""
        set(value) = requirePrefs().edit().putString(KEY_RPC_ENDPOINT, value).apply()

    // === 挖矿开关 ===
    var miningEnabled: Boolean
        get() = requirePrefs().getBoolean(KEY_MINING_ENABLED, true)
        set(value) = requirePrefs().edit().putBoolean(KEY_MINING_ENABLED, value).apply()

    var nodeSyncEnabled: Boolean
        get() = requirePrefs().getBoolean(KEY_NODE_SYNC_ENABLED, true)
        set(value) = requirePrefs().edit().putBoolean(KEY_NODE_SYNC_ENABLED, value).apply()

    var wifiOnly: Boolean
        get() = requirePrefs().getBoolean(KEY_WIFI_ONLY, true)
        set(value) = requirePrefs().edit().putBoolean(KEY_WIFI_ONLY, value).apply()

    var batteryConfig: String
        get() = requirePrefs().getString(KEY_BATTERY_CONFIG, "medium") ?: "medium"
        set(value) = requirePrefs().edit().putString(KEY_BATTERY_CONFIG, value).apply()

    // === 设备标识 ===
    var deviceId: String?
        get() = requirePrefs().getString(KEY_DEVICE_ID, null)
        set(value) = requirePrefs().edit().putString(KEY_DEVICE_ID, value).apply()

    var attestationToken: String?
        get() = requirePrefs().getString(KEY_ATTEST_TOKEN, null)
        set(value) = requirePrefs().edit().putString(KEY_ATTEST_TOKEN, value).apply()

    var attestationExpiry: Long
        get() = requirePrefs().getLong(KEY_ATTEST_EXPIRY, 0)
        set(value) = requirePrefs().edit().putLong(KEY_ATTEST_EXPIRY, value).apply()

    // === 最后同步高度 ===
    var lastSyncedHeight: Long
        get() = requirePrefs().getLong(KEY_SYNCED_HEIGHT, 0)
        set(value) = requirePrefs().edit().putLong(KEY_SYNCED_HEIGHT, value).apply()

    var lastHeartbeatTime: Long
        get() = requirePrefs().getLong(KEY_HEARTBEAT_TIME, 0)
        set(value) = requirePrefs().edit().putLong(KEY_HEARTBEAT_TIME, value).apply()

    // === 首次启动 ===
    var isFirstLaunch: Boolean
        get() = requirePrefs().getBoolean(KEY_FIRST_LAUNCH, true)
        set(value) = requirePrefs().edit().putBoolean(KEY_FIRST_LAUNCH, value).apply()

    // === 清除所有 ===
    fun clearAll() {
        requirePrefs().edit().clear().apply()
    }

    companion object {
        private const val KEY_MNEMONIC = "encrypted_mnemonic"
        private const val KEY_PRIVATE_KEY = "encrypted_private_key"
        private const val KEY_SEED_NODES = "seed_nodes"
        private const val KEY_RPC_ENDPOINT = "rpc_endpoint"
        private const val KEY_MINING_ENABLED = "mining_enabled"
        private const val KEY_NODE_SYNC_ENABLED = "node_sync_enabled"
        private const val KEY_WIFI_ONLY = "wifi_only"
        private const val KEY_BATTERY_CONFIG = "battery_config"
        private const val KEY_DEVICE_ID = "device_id"
        private const val KEY_ATTEST_TOKEN = "attest_token"
        private const val KEY_ATTEST_EXPIRY = "attest_expiry"
        private const val KEY_SYNCED_HEIGHT = "synced_height"
        private const val KEY_HEARTBEAT_TIME = "heartbeat_time"
        private const val KEY_FIRST_LAUNCH = "first_launch"
    }
}
