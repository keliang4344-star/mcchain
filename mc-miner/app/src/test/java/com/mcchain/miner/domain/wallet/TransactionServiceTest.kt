package com.mcchain.miner.domain.wallet

import android.util.Base64
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.NodeStatusSnapshot
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.TxStatus
import com.mcchain.miner.data.model.TxType
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.network.Delegation
import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.network.TxBroadcastResult
import com.mcchain.miner.network.TxResult
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.mockk
import io.mockk.mockkStatic
import io.mockk.unmockkAll
import io.mockk.verify
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.IOException

/**
 * TransactionService 集成层覆盖：
 *  - 每个入口的输入校验与中文错误文案；
 *  - 链 ID 校验、签名账户一致性、手续费硬校验；
 *  - 广播成功 / 重复广播 / sequence 重试 / 离线排队 / 回执失败；
 *  - 离线队列 replayPending 的全部分支。
 *
 * RpcClient / TxBuilder / WalletRepository / SecurePrefs 用 MockK 替身，
 * Base64 用静态 mock 隔离 Android 框架（JVM 单测无真实实现）。
 */
class TransactionServiceTest {

    private val rpc = mockk<RpcClient>(relaxed = true)
    private val txBuilder = mockk<TxBuilder>(relaxed = true)
    private val feeProvider = mockk<DynamicFeeProvider>(relaxed = true)
    private val repo = mockk<WalletRepository>(relaxed = true)
    private val prefs = mockk<SecurePrefs>(relaxed = true)

    private lateinit var service: TransactionService

    private val address = "mc14hka32hqq4ejgd8dgu70nkljwtnzsnvh2wg87y"
    private val other = "mc19rl4cm2hmr8afy4kldpxz3fka4jguq0au9f863"
    // AddressRules 要求验证人地址长度 48..62：10 位 HRP + 38 位数据。
    private val validator = "mcvaloper1" + "q".repeat(38)
    private val otherValidator = "mcvaloper1" + "w".repeat(38)

    private val account = WalletAccount(
        address = address,
        accountIndex = 0,
        publicKeyHex = "02",
        balance = 100_000,
        sequence = 1,
        accountNumber = 1,
        isDefault = true,
    )

    @Before
    fun setUp() {
        service = TransactionService(rpc, txBuilder, feeProvider, repo, prefs)

        mockkStatic(Base64::class)
        every { Base64.encodeToString(any(), any()) } returns "cmF3"
        every { Base64.decode(any<String>(), any()) } returns byteArrayOf(1, 2, 3)

        coEvery { feeProvider.refresh() } returns Unit
        coEvery { repo.getDefaultAccount() } returns account
        coEvery { repo.getPendingTxs() } returns emptyList()
        coEvery { repo.insertTx(any()) } returns Unit
        coEvery { repo.updateTxStatus(any(), any()) } returns Unit
        coEvery { repo.updateBalances(any(), any(), any(), any(), any(), any()) } returns Unit
        coEvery { rpc.getNodeStatus(any()) } returns nodeStatus(MCParams.CHAIN_ID)
        coEvery { rpc.getAccount(any()) } returns Triple(1L, 1L, emptyList())
        coEvery { rpc.getDelegations(any()) } returns emptyList()
        coEvery { rpc.getBalanceOrNull(any(), any()) } returns 100_000L
        coEvery { rpc.getBalance(any(), any()) } returns 0L
        coEvery { rpc.getStakingInfo(any()) } returns 0L
        coEvery { rpc.getPendingRewards(any()) } returns 0L
    }

    @After
    fun tearDown() {
        unmockkAll()
    }

    private fun nodeStatus(chainId: String) = NodeStatusSnapshot(
        currentHeight = 1,
        latestBlockTime = 0,
        syncedPeers = 1,
        chainId = chainId,
        nodeVersion = "test",
        catchingUp = false,
        totalStorageBytes = 0,
        bandwidthUpBps = 0,
        bandwidthDownBps = 0,
        lastUpdated = 0,
    )

    private fun builtTx(fee: Long = 5_000, from: String = address) = TxBuilder.BuiltTx(
        txRaw = byteArrayOf(1, 2, 3),
        txHashHex = "ab".repeat(32),
        fromAddress = from,
        feeUmc = fee,
    )

    private fun receipt(code: Int = 0, log: String = "", height: Long = 10) = TxResult(
        hash = "ab".repeat(32),
        height = height,
        code = code,
        gasUsed = 100,
        gasWanted = 200,
        log = log,
        codespace = if (code == 0) "" else "sdk",
    )

    private fun txRecord(
        hash: String = "ab".repeat(32),
        status: TxStatus = TxStatus.PENDING,
        type: TxType = TxType.SEND,
        timestamp: Long = System.currentTimeMillis(),
    ) = TxRecord(
        hash = hash,
        height = 0,
        timestamp = timestamp,
        fromAddress = address,
        toAddress = other,
        amount = 1,
        denom = "umc",
        fee = 5_000,
        memo = "",
        type = type,
        status = status,
        gasUsed = 0,
        rawJson = "cmF3",
    )

    // === 输入校验 ===

    @Test
    fun sendValidationFailuresAreReadable() = runTest {
        coEvery { repo.getDefaultAccount() } returns null
        assertTrue(service.send(other, 1).message.contains("钱包未初始化"))

        coEvery { repo.getDefaultAccount() } returns account
        assertTrue(service.send("bad", 1).message.contains("收款地址格式无效"))
        assertTrue(service.send(address, 1).message.contains("不能向自己转账"))
        assertTrue(service.send(other, 0).message.contains("金额必须大于 0"))
        assertTrue(service.send(other, 100_001).message.contains("余额不足"))
    }

    @Test
    fun delegateValidationFailuresAreReadable() = runTest {
        assertTrue(service.delegate("bad", 1).message.contains("验证人地址"))
        assertTrue(service.delegate(validator, 0).message.contains("金额必须大于 0"))
        assertTrue(service.delegate(validator, 100_001).message.contains("余额不足"))
    }

    @Test
    fun undelegateChecksDelegationAndFee() = runTest {
        assertTrue(service.undelegate(validator, 0).message.contains("解除金额必须大于 0"))

        coEvery { rpc.getDelegations(address) } returns listOf(Delegation(otherValidator, 50_000))
        assertTrue(service.undelegate(validator, 1_000).message.contains("当前没有向该验证人的委托"))

        coEvery { rpc.getDelegations(address) } returns listOf(Delegation(validator, 50_000))
        assertTrue(service.undelegate(validator, 60_000).message.contains("超过该验证人的委托数量"))

        coEvery { repo.getDefaultAccount() } returns account.copy(balance = 4_999)
        assertTrue(service.undelegate(validator, 1_000).message.contains("不足以支付网络手续费"))
    }

    @Test
    fun redelegateValidatesAddressesAndAmount() = runTest {
        assertTrue(service.redelegate("bad", otherValidator, 1).message.contains("源验证人地址无效"))
        assertTrue(service.redelegate(validator, "bad", 1).message.contains("目标验证人地址无效"))
        assertTrue(service.redelegate(validator, validator, 1).message.contains("不能相同"))
        assertTrue(service.redelegate(validator, otherValidator, 0).message.contains("转委托金额必须大于 0"))

        coEvery { rpc.getDelegations(address) } returns listOf(Delegation(validator, 10))
        assertTrue(service.redelegate(validator, otherValidator, 11).message.contains("超过该验证人的委托数量"))

        coEvery { repo.getDefaultAccount() } returns account.copy(balance = 100)
        assertTrue(service.redelegate(validator, otherValidator, 1).message.contains("不足以支付网络手续费"))
    }

    @Test
    fun claimRewardsRequiresDelegationsAndPending() = runTest {
        assertTrue(service.claimAllRewards().message.contains("没有委托"))

        coEvery { rpc.getDelegations(address) } returns listOf(Delegation(validator, 100))
        coEvery { rpc.getPendingRewards(address) } returns 0L
        assertTrue(service.claimAllRewards().message.contains("没有可领取的奖励"))
    }

    @Test
    fun deviceRegistrationSkipsWhenAlreadyDoneOrUnfunded() = runTest {
        coEvery { repo.getDefaultAccount() } returns null
        assertNull(service.ensureDeviceRegistered("Pixel", "Android 15"))

        coEvery { repo.getDefaultAccount() } returns account
        every { prefs.registeredDeviceAddress } returns address
        assertNull(service.ensureDeviceRegistered("Pixel", "Android 15"))

        every { prefs.registeredDeviceAddress } returns null
        coEvery { repo.getPendingTxs() } returns listOf(
            txRecord(type = TxType.REGISTER_DEVICE),
        )
        assertNull(service.ensureDeviceRegistered("Pixel", "Android 15"))

        coEvery { repo.getPendingTxs() } returns emptyList()
        coEvery { repo.getDefaultAccount() } returns account.copy(balance = 5_000)
        assertNull(service.ensureDeviceRegistered("Pixel", "Android 15"))
    }

    @Test
    fun nodeRegistrationSkipsWhenAlreadyDoneOrUnfunded() = runTest {
        coEvery { repo.getDefaultAccount() } returns null
        assertNull(service.ensureNodeRegistered("Pixel", "Android 15"))

        coEvery { repo.getDefaultAccount() } returns account
        every { prefs.registeredNodeAddress } returns address
        assertNull(service.ensureNodeRegistered("Pixel", "Android 15"))

        every { prefs.registeredNodeAddress } returns null
        coEvery { repo.getDefaultAccount() } returns account.copy(balance = 5_000)
        assertNull(service.ensureNodeRegistered("Pixel", "Android 15"))
    }

    @Test
    fun attestDeviceRejectsBlankMaterials() = runTest {
        assertTrue(service.attestDevice("", "sig").message.contains("认证材料为空"))
        assertTrue(service.attestDevice("challenge", " ").message.contains("认证材料为空"))
    }

    @Test
    fun contributionRejectsInvalidIdAndScore() = runTest {
        assertTrue(
            service.submitContribution("", "inference", 50).message.contains("贡献幂等键长度无效"),
        )
        assertTrue(
            service.submitContribution("x".repeat(129), "inference", 50)
                .message.contains("贡献幂等键长度无效"),
        )
        assertTrue(service.submitContribution("task", "inference", -1).message.contains("分数越界"))
        assertTrue(service.submitContribution("task", "inference", 101).message.contains("分数越界"))
    }

    @Test
    fun edgeAiResultRejectsBlankFields() = runTest {
        assertTrue(
            service.submitEdgeAiResult("", "hash", "nonce").message.contains("任务 ID 或结果指纹为空"),
        )
        assertTrue(
            service.submitEdgeAiResult("task", " ", "nonce").message.contains("任务 ID 或结果指纹为空"),
        )
    }

    @Test
    fun referralValidatesInviteeCodeAndFee() = runTest {
        assertTrue(service.createReferral("bad", "code").message.contains("被邀请人地址无效"))
        assertTrue(service.createReferral(address, "code").message.contains("不能邀请自己"))
        assertTrue(service.createReferral(other, "x".repeat(33)).message.contains("渠道码过长"))

        coEvery { repo.getDefaultAccount() } returns account.copy(balance = 100)
        assertTrue(service.createReferral(other, "code").message.contains("不足以支付网络手续费"))
    }

    @Test
    fun claimReferralChecksPendingAndFee() = runTest {
        coEvery { rpc.getReferralPendingRewards(address) } returns null
        assertTrue(service.claimReferralRewards().message.contains("无法查询待领返佣"))

        coEvery { rpc.getReferralPendingRewards(address) } returns 0L
        assertTrue(service.claimReferralRewards().message.contains("没有可领取的返佣"))

        coEvery { rpc.getReferralPendingRewards(address) } returns 100L
        coEvery { repo.getDefaultAccount() } returns account.copy(balance = 100)
        assertTrue(service.claimReferralRewards().message.contains("不足以支付网络手续费"))
    }

    @Test
    fun createTaskValidatesDescriptionAndReward() = runTest {
        assertTrue(service.createEdgeAiTask(" ", 1).message.contains("任务描述不能为空"))
        assertTrue(service.createEdgeAiTask("x".repeat(501), 1).message.contains("任务描述过长"))
        assertTrue(service.createEdgeAiTask("task", -1).message.contains("任务奖励不能为负数"))
        assertTrue(service.createEdgeAiTask("task", 96_000).message.contains("余额不足以同时托管"))
    }

    @Test
    fun disputeValidatesFieldsAndFee() = runTest {
        assertTrue(service.openEdgeAiDispute("", "reason").message.contains("请输入任务 ID"))
        assertTrue(service.openEdgeAiDispute("task", " ").message.contains("请填写争议理由"))
        assertTrue(service.openEdgeAiDispute("task", "x".repeat(201)).message.contains("争议理由过长"))

        coEvery { repo.getDefaultAccount() } returns account.copy(balance = 100)
        assertTrue(service.openEdgeAiDispute("task", "reason").message.contains("不足以支付网络手续费"))
    }

    @Test
    fun allSubmitPathsRejectMissingWallet() = runTest {
        coEvery { repo.getDefaultAccount() } returns null
        val results = listOf(
            service.send(other, 1),
            service.delegate(validator, 1),
            service.undelegate(validator, 1),
            service.redelegate(validator, otherValidator, 1),
            service.claimAllRewards(),
            service.attestDevice("challenge", "sig"),
            service.submitPhonenodeAttestation("root", "nonce", "dev", "pub", "sig"),
            service.submitContribution("task", "inference", 50),
            service.sendHeartbeat("device"),
            service.submitEdgeAiResult("task", "hash", "nonce"),
            service.createReferral(other, "code"),
            service.claimReferralRewards(),
            service.createEdgeAiTask("task", 1),
            service.openEdgeAiDispute("task", "reason"),
        )
        results.forEach { assertTrue(it.message.contains("钱包未初始化")) }
    }

    // === 提交流程 ===

    @Test
    fun missingAccountInfoFailsReadably() = runTest {
        coEvery { rpc.getAccount(address) } returns null
        val result = service.send(other, 1_000)
        assertFalse(result.success)
        assertTrue(result.message.contains("无法获取链上账户信息"))
    }

    @Test
    fun chainIdMismatchBlocksSigning() = runTest {
        coEvery { rpc.getNodeStatus(any()) } returns nodeStatus("mcchain-other-1")
        val result = service.send(other, 1_000)
        assertFalse(result.success)
        assertTrue(result.message.contains("节点链 ID"))
        coVerify(exactly = 0) {
            txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any())
        }
    }

    @Test
    fun mismatchedSigningAccountIsBlocked() = runTest {
        coEvery { txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx(from = other)

        val result = service.send(other, 1_000)
        assertFalse(result.success)
        assertTrue(result.message.contains("账户状态不一致"))
        coVerify(exactly = 0) { rpc.broadcastTxRaw(any()) }
    }

    @Test
    fun insufficientLiquidBalanceBlocksBroadcast() = runTest {
        coEvery { txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx(fee = 5_000)
        coEvery { rpc.getBalanceOrNull(any(), any()) } returns 100L

        val result = service.send(other, 1_000)
        assertFalse(result.success)
        assertTrue(result.message.contains("可用余额不足以支付网络手续费"))
        coVerify(exactly = 0) { rpc.broadcastTxRaw(any()) }
    }

    @Test
    fun sendSuccessRecordsAndRefreshes() = runTest {
        coEvery { txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } returns TxBroadcastResult("ab".repeat(32), 0, "")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns receipt()

        val result = service.send(other, 1_000)

        assertTrue(result.success)
        assertFalse(result.pending)
        assertEquals("交易已确认", result.message)
        assertEquals("ab".repeat(32), result.txHash)
        coVerify {
            repo.insertTx(
                match {
                    it.status == TxStatus.SUCCESS && it.fee == 5_000L && it.rawJson == "cmF3"
                },
            )
        }
    }

    @Test
    fun networkFailureQueuesSignedTxForReplay() = runTest {
        coEvery { txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } throws IOException("offline")

        val result = service.send(other, 1_000)

        assertTrue(result.success)
        assertTrue(result.pending)
        assertTrue(result.message.contains("已排队"))
        coVerify {
            repo.insertTx(match { it.status == TxStatus.PENDING && it.rawJson == "cmF3" })
        }
    }

    @Test
    fun duplicateBroadcastConvergesAsSuccess() = runTest {
        coEvery { txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } returns
            TxBroadcastResult("", 5, "tx already exists in cache", "sdk")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns receipt()

        val result = service.send(other, 1_000)

        assertTrue(result.success)
        assertEquals("交易已确认", result.message)
        assertEquals("节点未回传 txhash 时使用本地计算值", "ab".repeat(32), result.txHash)
    }

    @Test
    fun sequenceMismatchRetriesWithFreshSequence() = runTest {
        coEvery { txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        var broadcasts = 0
        coEvery { rpc.broadcastTxRaw(any()) } answers {
            broadcasts++
            if (broadcasts == 1) {
                TxBroadcastResult("", 3, "account sequence mismatch", "sdk")
            } else {
                TxBroadcastResult("ab".repeat(32), 0, "")
            }
        }
        coEvery { rpc.waitForTx(any(), any(), any()) } returns receipt()

        val result = service.send(other, 1_000)

        assertTrue(result.success)
        assertEquals("sequence 冲突必须重签一次", 2, broadcasts)
        coVerify(exactly = 2) {
            txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any())
        }
    }

    @Test
    fun failedReceiptReturnsChineseReason() = runTest {
        coEvery { txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } returns TxBroadcastResult("ab".repeat(32), 0, "")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns
            receipt(code = 5, log = "insufficient funds")

        val result = service.send(other, 1_000)

        assertFalse(result.success)
        assertTrue(result.message.contains("余额不足"))
        coVerify {
            repo.insertTx(match { it.status == TxStatus.FAILED })
        }
    }

    @Test
    fun missingReceiptKeepsPending() = runTest {
        coEvery { txBuilder.buildSendTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } returns TxBroadcastResult("ab".repeat(32), 0, "")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns null

        val result = service.send(other, 1_000)

        assertTrue(result.success)
        assertTrue(result.pending)
        assertTrue(result.message.contains("等待确认"))
    }

    @Test
    fun claimAllRewardsBuildsOneTxForAllDelegations() = runTest {
        coEvery { rpc.getDelegations(address) } returns listOf(
            Delegation(validator, 100),
            Delegation(otherValidator, 200),
        )
        coEvery { rpc.getPendingRewards(address) } returns 300L
        coEvery { txBuilder.buildClaimRewardsTx(any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } returns TxBroadcastResult("ab".repeat(32), 0, "")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns receipt()

        val result = service.claimAllRewards()

        assertTrue(result.success)
        coVerify(exactly = 1) {
            txBuilder.buildClaimRewardsTx(any(), any(), any(), any(), any(), any())
        }
    }

    @Test
    fun deviceRegistrationMarksAddressAfterSuccess() = runTest {
        every { prefs.registeredDeviceAddress } returns null
        coEvery { repo.getPendingTxs() } returns emptyList()
        coEvery { txBuilder.buildRegisterDeviceTx(any(), any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } returns TxBroadcastResult("ab".repeat(32), 0, "")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns receipt()

        val result = service.ensureDeviceRegistered("Pixel", "Android 15")

        assertTrue(result?.success == true)
        assertEquals("设备已上链注册", result?.message)
        verify { prefs.registeredDeviceAddress = address }
    }

    @Test
    fun contributionDuplicateConvergesAsIdempotent() = runTest {
        coEvery {
            txBuilder.buildSubmitContributionTx(any(), any(), any(), any(), any(), any(), any(), any())
        } returns builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } returns TxBroadcastResult("ab".repeat(32), 0, "")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns
            receipt(code = 1102, log = "task id already exists")

        val result = service.submitContribution("task-1", "inference", 88)

        assertTrue(result.success)
        assertTrue(result.idempotent)
        assertTrue(result.message.contains("贡献已记录"))
    }

    @Test
    fun heartbeatSuccessUsesLatestHeight() = runTest {
        coEvery { rpc.getLatestBlockHeight() } returns 100L
        coEvery { txBuilder.buildHeartbeatTx(any(), any(), any(), any(), any(), any()) } returns
            builtTx()
        coEvery { rpc.broadcastTxRaw(any()) } returns TxBroadcastResult("ab".repeat(32), 0, "")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns receipt()

        val result = service.sendHeartbeat("device-1")

        assertTrue(result.success)
        coVerify(exactly = 1) {
            txBuilder.buildHeartbeatTx(
                match { it.startsWith("phonenode|device-1|100|") },
                any(), any(), any(), any(), any(),
            )
        }
    }

    // === 离线队列 ===

    @Test
    fun replayPendingReturnsZeroWhenQueueEmpty() = runTest {
        assertEquals(0, service.replayPending())
    }

    @Test
    fun replayPendingResolvesConfirmedTx() = runTest {
        val tx = txRecord()
        coEvery { repo.getPendingTxs() } returns listOf(tx)
        coEvery { rpc.getTxResult(tx.hash) } returns receipt(height = 20)

        assertEquals(1, service.replayPending())
        coVerify { repo.updateTxStatus(tx.hash, TxStatus.SUCCESS) }
    }

    @Test
    fun replayPendingFailsExpiredTx() = runTest {
        val now = 1_000_000_000L
        val tx = txRecord(timestamp = now - 11 * 60 * 1_000L)
        coEvery { repo.getPendingTxs() } returns listOf(tx)
        coEvery { rpc.getTxResult(any()) } returns null

        assertEquals(1, service.replayPending(now))
        coVerify { repo.updateTxStatus(tx.hash, TxStatus.FAILED) }
    }

    @Test
    fun replayPendingSkipsUndecodableRaw() = runTest {
        every { Base64.decode(any<String>(), any()) } throws IllegalArgumentException("bad")
        val tx = txRecord()
        coEvery { repo.getPendingTxs() } returns listOf(tx)
        coEvery { rpc.getTxResult(any()) } returns null

        assertEquals(0, service.replayPending())
    }

    @Test
    fun replayPendingHandlesAlreadyKnownTx() = runTest {
        val tx = txRecord()
        coEvery { repo.getPendingTxs() } returns listOf(tx)
        coEvery { rpc.getTxResult(any()) } returns null
        coEvery { rpc.broadcastTxRaw(any()) } returns
            TxBroadcastResult("", 5, "tx already exists in cache", "sdk")
        coEvery { rpc.waitForTx(any(), any(), any()) } returns receipt(height = 30)

        assertEquals(1, service.replayPending())
        coVerify { repo.updateTxStatus(tx.hash, TxStatus.SUCCESS) }
    }

    @Test
    fun replayPendingFailsPermanentRejection() = runTest {
        val tx = txRecord()
        coEvery { repo.getPendingTxs() } returns listOf(tx)
        coEvery { rpc.getTxResult(any()) } returns null
        coEvery { rpc.broadcastTxRaw(any()) } returns
            TxBroadcastResult("", 3, "account sequence mismatch", "sdk")

        assertEquals(1, service.replayPending())
        coVerify { repo.updateTxStatus(tx.hash, TxStatus.FAILED) }
    }

    @Test
    fun replayPendingKeepsNetworkFailuresQueued() = runTest {
        val tx = txRecord()
        coEvery { repo.getPendingTxs() } returns listOf(tx)
        coEvery { rpc.getTxResult(any()) } returns null
        coEvery { rpc.broadcastTxRaw(any()) } throws IOException("offline")

        assertEquals(0, service.replayPending())
        coVerify(exactly = 0) { repo.updateTxStatus(any(), any()) }
    }
}

