package com.mcchain.miner.data.repository

import com.mcchain.miner.data.db.ContributionDao
import com.mcchain.miner.data.db.EdgeAiTaskDao
import com.mcchain.miner.data.model.AiTaskStatus
import com.mcchain.miner.data.model.ContribStatus
import com.mcchain.miner.data.model.Contribution
import com.mcchain.miner.data.model.EdgeAiTask
import com.mcchain.miner.network.RpcClient
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class MinerRepository @Inject constructor(
    private val contributionDao: ContributionDao,
    private val edgeAiTaskDao: EdgeAiTaskDao,
    private val rpcClient: RpcClient
) {
    suspend fun getContributions(deviceId: String, limit: Int = 100): List<Contribution> =
        contributionDao.getByDevice(deviceId, limit)

    suspend fun getContributionsByStatus(status: ContribStatus, limit: Int = 50): List<Contribution> =
        contributionDao.getByStatus(status, limit)

    suspend fun getVerifiedSince(sinceDeadline: Long): List<Contribution> =
        contributionDao.getVerifiedSince(sinceDeadline)

    suspend fun getTotalReward(deviceId: String): Long =
        contributionDao.getTotalReward(deviceId)

    suspend fun getPendingReward(since: Long): Long =
        contributionDao.getPendingReward(since)

    suspend fun getCountByDevice(deviceId: String): Int =
        contributionDao.getCountByDevice(deviceId)

    suspend fun upsertContribution(contribution: Contribution) =
        contributionDao.upsert(contribution)

    suspend fun updateVerification(id: String, status: ContribStatus, reward: Long, verifiedAt: Long) =
        contributionDao.updateVerification(id, status, reward, verifiedAt)

    suspend fun getAvailableTasks(limit: Int = 20): List<EdgeAiTask> =
        edgeAiTaskDao.getAvailableTasks(limit)

    suspend fun getRecentTasks(limit: Int = 50): List<EdgeAiTask> =
        edgeAiTaskDao.getRecentTasks(limit)

    suspend fun getTask(taskId: String): EdgeAiTask? =
        edgeAiTaskDao.getTask(taskId)

    suspend fun upsertTask(task: EdgeAiTask) =
        edgeAiTaskDao.upsert(task)

    suspend fun updateTaskStatus(taskId: String, status: AiTaskStatus) =
        edgeAiTaskDao.updateStatus(taskId, status)
}
