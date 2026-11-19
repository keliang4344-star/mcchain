package com.mcchain.miner.domain.edgeai

import com.mcchain.miner.data.db.AccountDao
import com.mcchain.miner.data.db.EdgeAiTaskDao
import com.mcchain.miner.data.model.AiTaskStatus
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.domain.node.DeviceAttestation
import com.mcchain.miner.domain.wallet.TransactionService
import com.mcchain.miner.network.RpcClient
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.TimeoutCancellationException
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout
import javax.inject.Inject
import javax.inject.Singleton

/**
 * EdgeAI 任务编排（方案 §1.2 TaskExecutor + Phase 4）：
 * 拉取链上任务 → 白名单过滤 → 本地确定性执行（限时、单并发）→ MsgSubmitResult 上链。
 *
 * 链上结算（争议窗口 100 块后拨付 85%）由 edgeai 模块 BeginBlock 完成，App 只读余额，
 * 不做任何本地记账。
 */
@Singleton
class EdgeAiService @Inject constructor(
    private val rpcClient: RpcClient,
    private val edgeAiTaskDao: EdgeAiTaskDao,
    private val transactionService: TransactionService,
    private val deviceAttestation: DeviceAttestation,
    private val securePrefs: SecurePrefs,
    private val accountDao: AccountDao,
) {
    enum class RunOutcome { NO_TASK, SKIP_NOT_ATTESTED, SKIP_NO_FUNDS, SKIP_UNSUPPORTED, SUBMITTED, DUPLICATE, EXEC_TIMEOUT, FAILED }

    data class RunReport(val outcome: RunOutcome, val taskId: String = "", val message: String = "")

    /** 从链上同步 open 任务到本地（description 存于 taskType 字段供白名单解析）。 */
    suspend fun syncTasks(): Int {
        val tasks = rpcClient.getEdgeAiTasks()
        tasks.forEach { edgeAiTaskDao.upsert(it) }
        return tasks.size
    }

    /**
     * 执行一个到期的白名单任务并提交结果。
     * 前置闸门（链上会拒绝的都在这里先拦掉，避免白付手续费）：
     *  - 设备认证有效（SubmitResult 要求 phonenode.IsAttested）；
     *  - 余额覆盖手续费；
     *  - 任务在类型白名单内；单任务执行限时 [EXEC_TIMEOUT_MS]（方案 §5.2）。
     */
    suspend fun runDueTask(): RunReport {
        if (!deviceAttestation.isAttestationValid()) {
            return RunReport(RunOutcome.SKIP_NOT_ATTESTED, message = "设备未认证或已过期，暂不接单")
        }
        val account = accountDao.getDefaultAccount()
            ?: return RunReport(RunOutcome.SKIP_NOT_ATTESTED, message = "钱包未初始化")
        if (account.balance <= FEE_RESERVE_UMC) {
            return RunReport(RunOutcome.SKIP_NO_FUNDS, message = "余额不足以支付任务手续费")
        }

        val candidates = edgeAiTaskDao.getAvailableTasks(limit = 8)
        var sawUnsupported = false
        for (task in candidates) {
            val spec = TaskSpec.parse(task.taskType)
            if (spec == null) {
                sawUnsupported = true
                continue
            }
            val resultHash = try {
                withContext(Dispatchers.Default) {
                    withTimeout(EXEC_TIMEOUT_MS) { TaskExecutor.execute(spec) }
                }
            } catch (e: TimeoutCancellationException) {
                return RunReport(RunOutcome.EXEC_TIMEOUT, taskId = task.taskId, message = "任务执行超时，已放弃")
            } catch (e: Exception) {
                return RunReport(RunOutcome.FAILED, taskId = task.taskId, message = "执行失败：${e.message}")
            }

            edgeAiTaskDao.updateStatus(task.taskId, AiTaskStatus.IN_PROGRESS)
            val nonce = securePrefs.attestationToken.orEmpty()
            val submit = transactionService.submitEdgeAiResult(task.taskId, resultHash, nonce)
            return when {
                submit.success -> {
                    edgeAiTaskDao.updateStatus(task.taskId, AiTaskStatus.AWAITING_VERIFICATION)
                    RunReport(RunOutcome.SUBMITTED, taskId = task.taskId, message = submit.message)
                }
                submit.message.contains("已提交过") -> {
                    // 链上 ErrDuplicateResult：幂等收敛，不再重试。
                    edgeAiTaskDao.updateStatus(task.taskId, AiTaskStatus.COMPLETED)
                    RunReport(RunOutcome.DUPLICATE, taskId = task.taskId, message = submit.message)
                }
                else -> {
                    edgeAiTaskDao.updateStatus(task.taskId, AiTaskStatus.OPEN)
                    RunReport(RunOutcome.FAILED, taskId = task.taskId, message = submit.message)
                }
            }
        }
        return if (sawUnsupported) {
            RunReport(RunOutcome.SKIP_UNSUPPORTED, message = "链上任务均不在执行白名单内")
        } else {
            RunReport(RunOutcome.NO_TASK)
        }
    }

    private companion object {
        /** 方案 §5.2：单任务限时 60s，超时取消。 */
        const val EXEC_TIMEOUT_MS = 60_000L

        /** 提交一笔结果至少需要的余额（手续费 5000umc 的整数倍余量）。 */
        const val FEE_RESERVE_UMC = 10_000L
    }
}
