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

    /**
     * 当前活动账户在 BIP-44 地址链上的索引（m/44'/118'/0'/0/index）。
     * 签名私钥始终由该索引派生，必须与 Room 中 isDefault 的账户保持一致；
     * 不一致时由 WalletViewModel 在刷新时校正。
     */
    var activeAccountIndex: Int
        get() = requirePrefs().getInt(KEY_ACTIVE_ACCOUNT_INDEX, 0)
        set(value) = requirePrefs().edit().putInt(KEY_ACTIVE_ACCOUNT_INDEX, value).apply()

    // === 节点配置 ===
    var seedNodes: Set<String>
        get() = requirePrefs().getStringSet(KEY_SEED_NODES, emptySet()) ?: emptySet()
        set(value) = requirePrefs().edit().putStringSet(KEY_SEED_NODES, value).apply()

    var rpcEndpoint: String
        get() = requirePrefs().getString(KEY_RPC_ENDPOINT, "") ?: ""
        set(value) = requirePrefs().edit().putString(KEY_RPC_ENDPOINT, value).apply()

    /** 只读索引器 HTTP 地址（mcchain-indexer serve）。留空表示未配置，贡献历史不可用。 */
    var indexerEndpoint: String
        get() = requirePrefs().getString(KEY_INDEXER_ENDPOINT, "") ?: ""
        set(value) = requirePrefs().edit().putString(KEY_INDEXER_ENDPOINT, value).apply()

    /** 认证预言机地址（internal/oraclesvc 的 /sign）。留空表示未配置。 */
    var oracleEndpoint: String
        get() = requirePrefs().getString(KEY_ORACLE_ENDPOINT, "") ?: ""
        set(value) = requirePrefs().edit().putString(KEY_ORACLE_ENDPOINT, value).apply()

    /** 预言机 Bearer Token（ORACLE_SIGN_TOKEN）；留空表示服务端未启用鉴权。 */
    var oracleToken: String
        get() = requirePrefs().getString(KEY_ORACLE_TOKEN, "") ?: ""
        set(value) = requirePrefs().edit().putString(KEY_ORACLE_TOKEN, value).apply()

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

    /**
     * 是否显示前台服务常驻通知（默认 true）。
     * 由 E0 在 MinerService / NodeService 构建通知前读取；关闭时服务仍运行，只是不展示常驻通知。
     */
    var notificationEnabled: Boolean
        get() = requirePrefs().getBoolean(KEY_NOTIFICATION_ENABLED, true)
        set(value) = requirePrefs().edit().putBoolean(KEY_NOTIFICATION_ENABLED, value).apply()

    /**
     * 是否在设备重启后自动恢复服务（默认 false，需用户显式开启）。
     * 由 E0 在 BootReceiver 中读取；为 true 时才依据 nodeSyncEnabled / miningEnabled 启动对应服务。
     */
    var autoStartEnabled: Boolean
        get() = requirePrefs().getBoolean(KEY_AUTO_START_ENABLED, false)
        set(value) = requirePrefs().edit().putBoolean(KEY_AUTO_START_ENABLED, value).apply()

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

    /** 已完成 depin.MsgRegisterDevice 上链的钱包地址；null 表示尚未注册。 */
    var registeredDeviceAddress: String?
        get() = requirePrefs().getString(KEY_REGISTERED_DEVICE_ADDRESS, null)
        set(value) = requirePrefs().edit().putString(KEY_REGISTERED_DEVICE_ADDRESS, value).apply()

    /** 已完成 phonenode.MsgRegisterNode 上链的钱包地址；null 表示尚未注册。 */
    var registeredNodeAddress: String?
        get() = requirePrefs().getString(KEY_REGISTERED_NODE_ADDRESS, null)
        set(value) = requirePrefs().edit().putString(KEY_REGISTERED_NODE_ADDRESS, value).apply()

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
        private const val KEY_ACTIVE_ACCOUNT_INDEX = "active_account_index"
        private const val KEY_SEED_NODES = "seed_nodes"
        private const val KEY_RPC_ENDPOINT = "rpc_endpoint"
        private const val KEY_INDEXER_ENDPOINT = "indexer_endpoint"
        private const val KEY_ORACLE_ENDPOINT = "oracle_endpoint"
        private const val KEY_ORACLE_TOKEN = "oracle_token"
        private const val KEY_MINING_ENABLED = "mining_enabled"
        private const val KEY_NODE_SYNC_ENABLED = "node_sync_enabled"
        private const val KEY_WIFI_ONLY = "wifi_only"
        private const val KEY_BATTERY_CONFIG = "battery_config"
        private const val KEY_NOTIFICATION_ENABLED = "notification_enabled"
        private const val KEY_AUTO_START_ENABLED = "auto_start_enabled"
        private const val KEY_DEVICE_ID = "device_id"
        private const val KEY_ATTEST_TOKEN = "attest_token"
        private const val KEY_ATTEST_EXPIRY = "attest_expiry"
        private const val KEY_REGISTERED_DEVICE_ADDRESS = "registered_device_address"
        private const val KEY_REGISTERED_NODE_ADDRESS = "registered_node_address"
        private const val KEY_SYNCED_HEIGHT = "synced_height"
        private const val KEY_HEARTBEAT_TIME = "heartbeat_time"
        private const val KEY_FIRST_LAUNCH = "first_launch"
    }
}
