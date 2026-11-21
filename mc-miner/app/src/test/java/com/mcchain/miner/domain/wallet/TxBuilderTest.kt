package com.mcchain.miner.domain.wallet

import io.mockk.every
import io.mockk.mockk
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

/**
 * TxBuilder 覆盖：16 类交易消息都能构造出可广播的 TxRaw，
 * 并校验默认 gas、动态手续费、显式手续费与异常路径。
 *
 * 仅 JVM 单测：WalletManager / DynamicFeeProvider 用 MockK 替身，
 * 签名与 protobuf 编码走真实的 shared 实现。
 */
class TxBuilderTest {

    private val mnemonic =
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
            .split(" ")

    private val walletManager = mockk<WalletManager>()
    private val feeProvider = mockk<DynamicFeeProvider>()
    private lateinit var wallet: DerivedWallet
    private lateinit var builder: TxBuilder

    private val toAddress = "mc14hka32hqq4ejgd8dgu70nkljwtnzsnvh2wg87y"
    private val validator = "mcvaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"
    private val otherValidator = "mcvaloper1wwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwwww"

    @Before
    fun setUp() {
        wallet = WalletDerivation.derive(mnemonic)
        every { walletManager.getStoredWallet() } returns wallet
        // 费用 = 1000 + gas limit，便于断言每个入口传入的默认 gas limit。
        every { feeProvider.feeFor(any()) } answers { 1_000L + firstArg<Long>() }
        builder = TxBuilder(walletManager, feeProvider)
    }

    private fun assertBuilt(tx: TxBuilder.BuiltTx, expectedFee: Long) {
        assertTrue("txRaw 不能为空", tx.txRaw.isNotEmpty())
        assertEquals("txhash 必须是 64 位 hex", 64, tx.txHashHex.length)
        assertTrue(
            "txhash 只能包含 hex",
            tx.txHashHex.all { it.isDigit() || it.lowercaseChar() in 'a'..'f' },
        )
        assertEquals(wallet.address, tx.fromAddress)
        assertEquals(expectedFee, tx.feeUmc)
    }

    @Test
    fun allTransactionKindsBuildValidSignedTxWithDefaultGas() {
        val txs = listOf(
            builder.buildSendTx(toAddress, 1_000_000, 7, 3),
            builder.buildDelegateTx(validator, 2_000_000, 7, 3),
            builder.buildUndelegateTx(validator, 3_000_000, 7, 3),
            builder.buildRedelegateTx(validator, otherValidator, 4_000_000, 7, 3),
            builder.buildClaimRewardsTx(listOf(validator, otherValidator), 7, 3),
            builder.buildRegisterDeviceTx("Pixel", "Android 15", 7, 3),
            builder.buildAttestDeviceTx("challenge", "c2ln", 7, 3),
            builder.buildSubmitContributionTx("task-1", "inference", 88, 7, 3),
            builder.buildRegisterNodeTx("Pixel", "Android 15", "depin", 7, 3),
            builder.buildPhonenodeAttestTx("root", "nonce", "dev", "pub", "sig", 7, 3),
            builder.buildSubmitResultTx("task-1", "hash", "nonce", 7, 3),
            builder.buildCreateTaskTx("推理任务", 5_000, 7, 3),
            builder.buildCreateReferralTx(toAddress, "code-1", 7, 3),
            builder.buildClaimReferralTx(7, 3),
            builder.buildOpenDisputeTx("task-1", "理由", 7, 3),
            builder.buildHeartbeatTx("phonenode|dev|100|1", 7, 3),
        )
        val expectedFees = listOf(
            1_000L + TxBuilder.DEFAULT_GAS_SEND,
            1_000L + TxBuilder.DEFAULT_GAS_DELEGATE,
            1_000L + TxBuilder.DEFAULT_GAS_UNDELEGATE,
            1_000L + TxBuilder.DEFAULT_GAS_REDELEGATE,
            1_000L + TxBuilder.DEFAULT_GAS_CLAIM,
            1_000L + TxBuilder.DEFAULT_GAS_REGISTER,
            1_000L + TxBuilder.DEFAULT_GAS_ATTEST,
            1_000L + TxBuilder.DEFAULT_GAS_CONTRIB,
            1_000L + TxBuilder.DEFAULT_GAS_REGISTER,
            1_000L + TxBuilder.DEFAULT_GAS_ATTEST,
            1_000L + TxBuilder.DEFAULT_GAS_EDGEAI,
            1_000L + TxBuilder.DEFAULT_GAS_EDGEAI,
            1_000L + TxBuilder.DEFAULT_GAS_REFERRAL,
            1_000L + TxBuilder.DEFAULT_GAS_REFERRAL,
            1_000L + TxBuilder.DEFAULT_GAS_DISPUTE,
            1_000L + TxBuilder.DEFAULT_GAS_HEARTBEAT,
        )
        assertEquals("交易类型数量与预期不一致", expectedFees.size, txs.size)
        txs.zip(expectedFees).forEach { (tx, fee) -> assertBuilt(tx, fee) }
        assertEquals("16 类交易的 txhash 必须互不相同", 16, txs.map { it.txHashHex }.toSet().size)
    }

    @Test
    fun explicitFeeOverridesDynamicProvider() {
        val tx = builder.buildSendTx(toAddress, 1, 1, 1, feeAmount = "7777")
        assertBuilt(tx, 7_777L)
    }

    @Test
    fun invalidExplicitFeeFallsBackToZeroForDisplay() {
        val tx = builder.buildSendTx(toAddress, 1, 1, 1, feeAmount = "not-a-number")
        assertEquals("无法解析的手续费只影响本地展示，不得崩溃", 0L, tx.feeUmc)
        assertTrue(tx.txRaw.isNotEmpty())
    }

    @Test
    fun customGasChangesFee() {
        val tx = builder.buildSendTx(toAddress, 1, 1, 1, gas = 42_000L)
        assertBuilt(tx, 43_000L)
    }

    @Test
    fun missingWalletIsRejectedWithReadableReason() {
        every { walletManager.getStoredWallet() } returns null
        val error = assertThrows(IllegalStateException::class.java) {
            builder.buildSendTx(toAddress, 1, 1, 1)
        }
        assertTrue(error.message.orEmpty().contains("钱包未初始化"))
    }

    @Test
    fun claimRewardsRequiresAtLeastOneValidator() {
        assertThrows(IllegalArgumentException::class.java) {
            builder.buildClaimRewardsTx(emptyList(), 1, 1)
        }
    }

    @Test
    fun heartbeatHashDependsOnLeaf() {
        val first = builder.buildHeartbeatTx("phonenode|dev|100|1", 1, 2)
        val second = builder.buildHeartbeatTx("phonenode|dev|101|2", 1, 2)
        assertNotEquals("不同心跳内容必须产生不同 txhash", first.txHashHex, second.txHashHex)
    }

    @Test
    fun sequenceAndAccountNumberChangeSignature() {
        val same = builder.buildSendTx(toAddress, 1, 1, 1)
        val nextSequence = builder.buildSendTx(toAddress, 1, 1, 2)
        val nextAccount = builder.buildSendTx(toAddress, 1, 2, 1)
        assertEquals(
            "account_number / sequence 必须参与签名",
            3,
            setOf(same.txHashHex, nextSequence.txHashHex, nextAccount.txHashHex).size,
        )
    }
}
