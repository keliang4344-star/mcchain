package com.mcchain.miner.data.db

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import com.mcchain.miner.data.model.*
import kotlinx.coroutines.flow.Flow

@Dao
interface BlockDao {
    @Query("SELECT * FROM blocks ORDER BY height DESC LIMIT :limit")
    suspend fun getRecentBlocks(limit: Int = 50): List<BlockInfo>

    @Query("SELECT * FROM blocks WHERE height = :height")
    suspend fun getBlock(height: Long): BlockInfo?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertBlock(block: BlockInfo)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertBlocks(blocks: List<BlockInfo>)

    @Query("SELECT MAX(height) FROM blocks")
    suspend fun getLatestHeight(): Long?

    @Query("DELETE FROM blocks WHERE height < :beforeHeight")
    suspend fun pruneBlocks(beforeHeight: Long): Int

    @Query("SELECT COUNT(*) FROM blocks")
    suspend fun getBlockCount(): Long
}

@Dao
interface TxDao {
    @Query("SELECT * FROM transactions WHERE height BETWEEN :from AND :to ORDER BY height DESC")
    suspend fun getTransactions(from: Long, to: Long): List<TxRecord>

    @Query("SELECT * FROM transactions WHERE fromAddress = :address OR toAddress = :address ORDER BY height DESC LIMIT :limit")
    suspend fun getTransactionsForAddress(address: String, limit: Int = 100): List<TxRecord>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertTx(tx: TxRecord)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertTxs(txs: List<TxRecord>)

    @Query("SELECT * FROM transactions WHERE status = 'PENDING'")
    suspend fun getPendingTxs(): List<TxRecord>

    @Query("UPDATE transactions SET status = :status WHERE hash = :hash")
    suspend fun updateTxStatus(hash: String, status: TxStatus)

    @Query("DELETE FROM transactions WHERE height < :beforeHeight")
    suspend fun pruneTxs(beforeHeight: Long): Int
}

@Dao
interface PeerDao {
    @Query("SELECT * FROM peers")
    suspend fun getAllPeers(): List<PeerNode>

    @Query("SELECT * FROM peers ORDER BY latency ASC LIMIT :limit")
    suspend fun getBestPeers(limit: Int = 10): List<PeerNode>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertPeer(peer: PeerNode)

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertPeers(peers: List<PeerNode>)

    @Query("DELETE FROM peers WHERE lastSeen < :olderThan")
    suspend fun removeStalePeers(olderThan: Long): Int

    @Query("SELECT COUNT(*) FROM peers")
    suspend fun getPeerCount(): Int
}

@Dao
interface AccountDao {
    @Query("SELECT * FROM accounts")
    suspend fun getAllAccounts(): List<WalletAccount>

    @Query("SELECT * FROM accounts WHERE address = :address")
    suspend fun getAccount(address: String): WalletAccount?

    @Query("SELECT * FROM accounts WHERE isDefault = 1 LIMIT 1")
    suspend fun getDefaultAccount(): WalletAccount?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertAccount(account: WalletAccount)

    @Query("UPDATE accounts SET balance = :balance, stakeBalance = :stake, rewardBalance = :reward, sequence = :sequence, accountNumber = :accountNumber WHERE address = :address")
    suspend fun updateBalances(
        address: String, balance: Long, stake: Long, reward: Long,
        sequence: Long, accountNumber: Long
    )

    @Query("UPDATE accounts SET isDefault = 0")
    suspend fun clearDefaultFlags()

    @Query("UPDATE accounts SET isDefault = 1 WHERE address = :address")
    suspend fun setDefault(address: String)
}

@Dao
interface PhoneNodeDao {
    @Query("SELECT * FROM phone_nodes")
    suspend fun getAll(): List<PhoneNodeRecord>

    @Query("SELECT * FROM phone_nodes WHERE deviceId = :deviceId")
    suspend fun getByDeviceId(deviceId: String): PhoneNodeRecord?

    @Query("SELECT * FROM phone_nodes WHERE nodeAddress = :address")
    suspend fun getByAddress(address: String): PhoneNodeRecord?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(record: PhoneNodeRecord)

    @Query("UPDATE phone_nodes SET status = :status, lastHeartbeat = :timestamp WHERE deviceId = :deviceId")
    suspend fun updateStatus(deviceId: String, status: NodeStatus, timestamp: Long)

    @Query("UPDATE phone_nodes SET status = :status, slashCount = slashCount + 1 WHERE deviceId = :deviceId")
    suspend fun markSlashed(deviceId: String, status: NodeStatus)

    @Query("SELECT * FROM phone_nodes WHERE status = 'ACTIVE'")
    suspend fun getActiveNodes(): List<PhoneNodeRecord>

    @Query("SELECT * FROM phone_nodes WHERE status = 'ACTIVE' AND lastHeartbeat < :threshold")
    suspend fun getHeartbeatOverdue(threshold: Long): List<PhoneNodeRecord>
}

@Dao
interface ContributionDao {
    @Query("SELECT * FROM contributions WHERE deviceId = :deviceId ORDER BY reportedAt DESC LIMIT :limit")
    suspend fun getByDevice(deviceId: String, limit: Int = 100): List<Contribution>

    @Query("SELECT * FROM contributions WHERE status = 'VERIFIED' AND verifiedAt > :sinceDeadline")
    suspend fun getVerifiedSince(sinceDeadline: Long): List<Contribution>

    @Query("SELECT * FROM contributions WHERE status = :status ORDER BY reportedAt DESC LIMIT :limit")
    suspend fun getByStatus(status: ContribStatus, limit: Int = 50): List<Contribution>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(contribution: Contribution)

    @Query("UPDATE contributions SET status = :status, reward = :reward, verifiedAt = :verifiedAt WHERE id = :id")
    suspend fun updateVerification(id: String, status: ContribStatus, reward: Long, verifiedAt: Long)

    @Query("SELECT COALESCE(SUM(reward), 0) FROM contributions WHERE deviceId = :deviceId AND status = 'PAID'")
    suspend fun getTotalReward(deviceId: String): Long

    @Query("SELECT COALESCE(SUM(reward), 0) FROM contributions WHERE status = 'VERIFIED' AND verifiedAt > :since")
    suspend fun getPendingReward(since: Long): Long

    @Query("SELECT COUNT(*) FROM contributions WHERE deviceId = :deviceId")
    suspend fun getCountByDevice(deviceId: String): Int
}

@Dao
interface EdgeAiTaskDao {
    @Query("SELECT * FROM edgeai_tasks WHERE status = 'OPEN' OR status = 'IN_PROGRESS' ORDER BY reward DESC LIMIT :limit")
    suspend fun getAvailableTasks(limit: Int = 20): List<EdgeAiTask>

    @Query("SELECT * FROM edgeai_tasks WHERE taskId = :taskId")
    suspend fun getTask(taskId: String): EdgeAiTask?

    @Query("SELECT * FROM edgeai_tasks ORDER BY createdAt DESC LIMIT :limit")
    suspend fun getRecentTasks(limit: Int = 50): List<EdgeAiTask>

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(task: EdgeAiTask)

    @Query("UPDATE edgeai_tasks SET status = :status WHERE taskId = :taskId")
    suspend fun updateStatus(taskId: String, status: AiTaskStatus)
}

@Dao
interface NodeStatusDao {
    @Query("SELECT * FROM node_status WHERE id = 1")
    suspend fun get(): NodeStatusSnapshot?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsert(status: NodeStatusSnapshot)
}
