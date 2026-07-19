package com.mcchain.miner.data.repository

import com.mcchain.miner.data.db.BlockDao
import com.mcchain.miner.data.db.NodeStatusDao
import com.mcchain.miner.data.db.PeerDao
import com.mcchain.miner.data.db.PhoneNodeDao
import com.mcchain.miner.data.model.BlockInfo
import com.mcchain.miner.data.model.NodeStatus
import com.mcchain.miner.data.model.NodeStatusSnapshot
import com.mcchain.miner.data.model.PeerNode
import com.mcchain.miner.data.model.PhoneNodeRecord
import com.mcchain.miner.network.RpcClient
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class NodeRepository @Inject constructor(
    private val nodeStatusDao: NodeStatusDao,
    private val phoneNodeDao: PhoneNodeDao,
    private val peerDao: PeerDao,
    private val blockDao: BlockDao,
    private val rpcClient: RpcClient
) {
    suspend fun getNodeStatus(): NodeStatusSnapshot? =
        nodeStatusDao.get()

    suspend fun upsertNodeStatus(status: NodeStatusSnapshot) =
        nodeStatusDao.upsert(status)

    suspend fun getPhoneNode(deviceId: String): PhoneNodeRecord? =
        phoneNodeDao.getByDeviceId(deviceId)

    suspend fun getPhoneNodeByAddress(address: String): PhoneNodeRecord? =
        phoneNodeDao.getByAddress(address)

    suspend fun getAllPhoneNodes(): List<PhoneNodeRecord> =
        phoneNodeDao.getAll()

    suspend fun getActiveNodes(): List<PhoneNodeRecord> =
        phoneNodeDao.getActiveNodes()

    suspend fun getHeartbeatOverdue(threshold: Long): List<PhoneNodeRecord> =
        phoneNodeDao.getHeartbeatOverdue(threshold)

    suspend fun upsertPhoneNode(record: PhoneNodeRecord) =
        phoneNodeDao.upsert(record)

    suspend fun updateNodeStatus(deviceId: String, status: NodeStatus, timestamp: Long) =
        phoneNodeDao.updateStatus(deviceId, status, timestamp)

    suspend fun markSlashed(deviceId: String, status: NodeStatus) =
        phoneNodeDao.markSlashed(deviceId, status)

    suspend fun getAllPeers(): List<PeerNode> =
        peerDao.getAllPeers()

    suspend fun getBestPeers(limit: Int = 10): List<PeerNode> =
        peerDao.getBestPeers(limit)

    suspend fun insertPeer(peer: PeerNode) =
        peerDao.insertPeer(peer)

    suspend fun insertPeers(peers: List<PeerNode>) =
        peerDao.insertPeers(peers)

    suspend fun removeStalePeers(olderThan: Long): Int =
        peerDao.removeStalePeers(olderThan)

    suspend fun getPeerCount(): Int =
        peerDao.getPeerCount()

    suspend fun getRecentBlocks(limit: Int = 50): List<BlockInfo> =
        blockDao.getRecentBlocks(limit)

    suspend fun getBlock(height: Long): BlockInfo? =
        blockDao.getBlock(height)

    suspend fun getLatestHeight(): Long? =
        blockDao.getLatestHeight()

    suspend fun insertBlock(block: BlockInfo) =
        blockDao.insertBlock(block)

    suspend fun insertBlocks(blocks: List<BlockInfo>) =
        blockDao.insertBlocks(blocks)

    suspend fun pruneBlocks(beforeHeight: Long): Int =
        blockDao.pruneBlocks(beforeHeight)

    suspend fun getBlockCount(): Long =
        blockDao.getBlockCount()
}
