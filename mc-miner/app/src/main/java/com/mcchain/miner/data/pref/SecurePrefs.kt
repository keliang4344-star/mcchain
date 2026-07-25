package com.mcchain.miner.data.pref

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class SecurePrefs @Inject constructor(
    @ApplicationContext context: Context
) {
    private val prefs: SharedPreferences = try {
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
        android.util.Log.w("SecurePrefs", "EncryptedSharedPreferences unavailable, falling back to plain SharedPreferences", e)
        context.getSharedPreferences("keystore_fallback", Context.MODE_PRIVATE)
    }

    // === 钱包密钥 ===
    var encryptedMnemonic: String?
        get() = prefs.getString(KEY_MNEMONIC, null)
        set(value) = prefs.edit().putString(KEY_MNEMONIC, value).apply()

    var encryptedPrivateKey: String?
        get() = prefs.getString(KEY_PRIVATE_KEY, null)
        set(value) = prefs.edit().putString(KEY_PRIVATE_KEY, value).apply()

    // === 节点配置 ===
    var seedNodes: Set<String>
        get() = prefs.getStringSet(KEY_SEED_NODES, emptySet()) ?: emptySet()
        set(value) = prefs.edit().putStringSet(KEY_SEED_NODES, value).apply()

    var rpcEndpoint: String
        get() = prefs.getString(KEY_RPC_ENDPOINT, "") ?: ""
        set(value) = prefs.edit().putString(KEY_RPC_ENDPOINT, value).apply()

    // === 挖矿开关 ===
    var miningEnabled: Boolean
        get() = prefs.getBoolean(KEY_MINING_ENABLED, true)
        set(value) = prefs.edit().putBoolean(KEY_MINING_ENABLED, value).apply()

    var nodeSyncEnabled: Boolean
        get() = prefs.getBoolean(KEY_NODE_SYNC_ENABLED, true)
        set(value) = prefs.edit().putBoolean(KEY_NODE_SYNC_ENABLED, value).apply()

    var wifiOnly: Boolean
        get() = prefs.getBoolean(KEY_WIFI_ONLY, true)
        set(value) = prefs.edit().putBoolean(KEY_WIFI_ONLY, value).apply()

    var batteryConfig: String
        get() = prefs.getString(KEY_BATTERY_CONFIG, "medium") ?: "medium"
        set(value) = prefs.edit().putString(KEY_BATTERY_CONFIG, value).apply()

    // === 设备标识 ===
    var deviceId: String?
        get() = prefs.getString(KEY_DEVICE_ID, null)
        set(value) = prefs.edit().putString(KEY_DEVICE_ID, value).apply()

    var attestationToken: String?
        get() = prefs.getString(KEY_ATTEST_TOKEN, null)
        set(value) = prefs.edit().putString(KEY_ATTEST_TOKEN, value).apply()

    var attestationExpiry: Long
        get() = prefs.getLong(KEY_ATTEST_EXPIRY, 0)
        set(value) = prefs.edit().putLong(KEY_ATTEST_EXPIRY, value).apply()

    // === 最后同步高度 ===
    var lastSyncedHeight: Long
        get() = prefs.getLong(KEY_SYNCED_HEIGHT, 0)
        set(value) = prefs.edit().putLong(KEY_SYNCED_HEIGHT, value).apply()

    var lastHeartbeatTime: Long
        get() = prefs.getLong(KEY_HEARTBEAT_TIME, 0)
        set(value) = prefs.edit().putLong(KEY_HEARTBEAT_TIME, value).apply()

    // === 首次启动 ===
    var isFirstLaunch: Boolean
        get() = prefs.getBoolean(KEY_FIRST_LAUNCH, true)
        set(value) = prefs.edit().putBoolean(KEY_FIRST_LAUNCH, value).apply()

    // === 清除所有 ===
    fun clearAll() {
        prefs.edit().clear().apply()
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
