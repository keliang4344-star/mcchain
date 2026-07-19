package com.mcchain.miner.data.model

import androidx.room.Entity
import androidx.room.PrimaryKey

/**
 * 区块元数据
 */
@Entity(tableName = "blocks")
data class BlockInfo(
    @PrimaryKey
    val height: Long,
    val hash: String,
    val time: Long,           // unix seconds
    val numTxs: Int,
    val proposer: String,
    val appHash: String,
    val syncStatus: SyncStatus = SyncStatus.COMPLETE
)

enum class SyncStatus {
    HEADER_ONLY, // 仅拉取区块头
    COMPLETE     // 完整区块
}

/**
 * 交易记录
 */
@Entity(tableName = "transactions")
data class TxRecord(
    @PrimaryKey
    val hash: String,
    val height: Long,
    val timestamp: Long,
    val fromAddress: String,
    val toAddress: String,
    val amount: Long,         // umc
    val denom: String,
    val fee: Long,
    val memo: String,
    val type: TxType,
    val status: TxStatus,
    val gasUsed: Long,
    val rawJson: String
)

enum class TxType { SEND, DELEGATE, UNDELEGATE, REDELEGATE, CLAIM_REWARDS, ATTEST_NODE, REPORT_CONTRIB, SUBMIT_TASK, UNKNOWN }
enum class TxStatus { PENDING, SUCCESS, FAILED }

/**
 * 对等节点
 */
@Entity(tableName = "peers")
data class PeerNode(
    @PrimaryKey
    val nodeId: String,
    val address: String,
    val moniker: String,
    val lastSeen: Long,
    val inbound: Boolean,
    val isValidator: Boolean,
    val latency: Long
)

/**
 * 钱包账户
 */
@Entity(tableName = "accounts")
data class WalletAccount(
    @PrimaryKey
    val address: String,
    val accountIndex: Int,
    val publicKeyHex: String,
    val balance: Long = 0,       // umc
    val stakeBalance: Long = 0,  // umc
    val rewardBalance: Long = 0, // umc
    val sequence: Long = 0,
    val accountNumber: Long = 0,
    val label: String = "",
    val isDefault: Boolean = false
)

/**
 * 电话号码设备注册
 */
@Entity(tableName = "phone_nodes")
data class PhoneNodeRecord(
    @PrimaryKey
    val deviceId: String,
    val nodeAddress: String,
    val attestationToken: String,
    val attestationTime: Long,
    val attestationExpiry: Long,
    val lastHeartbeat: Long,
    val status: NodeStatus,
    val stakedAmount: Long,      // umc
    val slashCount: Int = 0,
    val reputation: Int = 100
)

enum class NodeStatus {
    UNREGISTERED, // 未注册
    PENDING,      // 认证中
    ACTIVE,       // 在线贡献中
    GRACE,        // 进入宽限期
    JAILED,       // 已关押
    SLASHED       // 已罚没
}

/**
 * DePIN 贡献记录
 */
@Entity(tableName = "contributions")
data class Contribution(
    @PrimaryKey
    val id: String,             // tx_hash + device_id
    val deviceId: String,
    val taskType: TaskType,
    val taskId: String,
    val amount: Long,           // umc
    val proofHash: String,
    val reportedAt: Long,
    val verifiedAt: Long?,
    val status: ContribStatus,
    val reward: Long = 0        // umc
)

enum class TaskType {
    AI_INFERENCE,    // AI 推理
    DATA_STORAGE,    // 数据存储
    BANDWIDTH,       // 带宽贡献
    COMPUTATION      // 通用计算
}

enum class ContribStatus {
    SUBMITTED,
    VERIFYING,
    VERIFIED,
    DISPUTED,
    REJECTED,
    PAID
}

/**
 * EdgeAI 任务
 */
@Entity(tableName = "edgeai_tasks")
data class EdgeAiTask(
    @PrimaryKey
    val taskId: String,
    val creator: String,
    val taskType: String,       // "inference", "training", "fine_tuning"
    val modelId: String,
    val inputHash: String,
    val inputSizeBytes: Long,
    val reward: Long,           // umc
    val maxSubmissions: Int,
    val submissionCount: Int = 0,
    val deadlineHeight: Long,
    val disputeDeadline: Long,
    val status: AiTaskStatus,
    val createdAt: Long
)

enum class AiTaskStatus {
    OPEN,
    IN_PROGRESS,
    AWAITING_VERIFICATION,
    COMPLETED,
    DISPUTED,
    CANCELLED
}

/**
 * 节点运行状态快照
 */
@Entity(tableName = "node_status")
data class NodeStatusSnapshot(
    @PrimaryKey
    val id: Int = 1,           // singleton
    val currentHeight: Long,
    val latestBlockTime: Long,
    val syncedPeers: Int,
    val chainId: String,
    val nodeVersion: String,
    val catchingUp: Boolean,
    val totalStorageBytes: Long,
    val bandwidthUpBps: Long,
    val bandwidthDownBps: Long,
    val lastUpdated: Long
)
