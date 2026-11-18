package com.mcchain.miner.domain.wallet

import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.TxStatus
import com.mcchain.miner.data.model.TxType
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.network.RpcClient
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 链上交易编排：账户查询 -> sequence 获取 -> 构造/签名 -> REST 广播 -> 回执轮询 -> 本地记账。
 *
 * 所有交易都使用链上实时的 account_number / sequence，避免本地缓存过期导致
 * `account sequence mismatch`；广播失败会按错误类型给出可读提示。
 */
@Singleton
class TransactionService @Inject constructor(
    private val rpcClient: RpcClient,
    private val txBuilder: TxBuilder,
    private val walletRepository: WalletRepository,
) {
    data class SubmitResult(
        val success: Boolean,
        val pending: Boolean = false,
        val txHash: String = "",
        val message: String = "",
    )

    /**
     * 刷新余额 / 质押 / 待领奖励 / sequence。
     * @return 账户在链上存在且查询成功时为 true
     */
    suspend fun refreshAccount(address: String): Boolean {
        val info = rpcClient.getAccount(address) ?: return false
        val balance = rpcClient.getBalance(address)
        val stake = rpcClient.getStakingInfo(address)
        val rewards = rpcClient.getPendingRewards(address)
        walletRepository.updateBalances(
            address = address,
            balance = balance,
            stake = stake,
            reward = rewards,
            sequence = info.second,
            accountNumber = info.first,
        )
        return true
    }

    suspend fun send(toAddress: String, amountUmc: Long): SubmitResult {
        val account = defaultAccount() ?: return failure("钱包未初始化，请先创建或导入助记词")
        val to = toAddress.trim()
        if (!to.startsWith(MCParams.BECH32_PREFIX + "1")) return failure("收款地址格式无效")
        if (to == account.address) return failure("不能向自己转账")
        if (amountUmc <= 0) return failure("金额必须大于 0")
        if (account.balance > 0 && amountUmc > account.balance) return failure("余额不足")

        val info = rpcClient.getAccount(account.address)
            ?: return failure("无法获取链上账户信息，请检查网络或节点地址")
        val built = runCatching {
            txBuilder.buildSendTx(to, amountUmc, info.first, info.second)
        }.getOrElse { return failure(it.message ?: "交易构造失败") }

        return submit(TxType.SEND, account.address, to, amountUmc, "", built)
    }

    suspend fun delegate(validatorAddress: String, amountUmc: Long): SubmitResult {
        val account = defaultAccount() ?: return failure("钱包未初始化，请先创建或导入助记词")
        val validator = validatorAddress.trim()
        if (!validator.startsWith(MCParams.BECH32_PREFIX + "valoper1") &&
            !validator.startsWith(MCParams.BECH32_PREFIX + "1")
        ) {
            return failure("验证人地址格式无效")
        }
        if (amountUmc <= 0) return failure("金额必须大于 0")
        if (account.balance > 0 && amountUmc > account.balance) return failure("余额不足")

        val info = rpcClient.getAccount(account.address)
            ?: return failure("无法获取链上账户信息，请检查网络或节点地址")
        val built = runCatching {
            txBuilder.buildDelegateTx(validator, amountUmc, info.first, info.second)
        }.getOrElse { return failure(it.message ?: "交易构造失败") }

        return submit(TxType.DELEGATE, account.address, validator, amountUmc, "", built)
    }

    /** 领取当前账户全部委托奖励（一笔交易包含多条 withdraw 消息）。 */
    suspend fun claimAllRewards(): SubmitResult {
        val account = defaultAccount() ?: return failure("钱包未初始化，请先创建或导入助记词")
        val delegations = rpcClient.getDelegations(account.address)
        if (delegations.isEmpty()) return failure("当前账户没有委托，无法领取奖励")

        val pending = rpcClient.getPendingRewards(account.address)
        if (pending <= 0) return failure("当前没有可领取的奖励")

        val info = rpcClient.getAccount(account.address)
            ?: return failure("无法获取链上账户信息，请检查网络或节点地址")
        val built = runCatching {
            txBuilder.buildClaimRewardsTx(
                validatorAddresses = delegations.map { it.validatorAddress },
                accountNumber = info.first,
                sequence = info.second,
                // 每条 withdraw 约 60k gas，按委托数量动态放大上限。
                gas = TxBuilder.DEFAULT_GAS_CLAIM + 60_000L * delegations.size,
            )
        }.getOrElse { return failure(it.message ?: "交易构造失败") }

        return submit(TxType.CLAIM_REWARDS, account.address, "", pending, "claim-rewards", built)
    }

    private suspend fun submit(
        type: TxType,
        from: String,
        to: String,
        amount: Long,
        memo: String,
        built: TxBuilder.BuiltTx,
    ): SubmitResult {
        val broadcast = rpcClient.broadcastTxRaw(built.txRaw)
        if (broadcast.code != 0) {
            return failure(classify(broadcast.log))
        }
        if (broadcast.hash.isBlank()) {
            return failure("节点未返回交易哈希，请检查节点 REST 配置")
        }

        val receipt = rpcClient.waitForTx(broadcast.hash)
        val status = when {
            receipt == null -> TxStatus.PENDING
            receipt.code == 0 -> TxStatus.SUCCESS
            else -> TxStatus.FAILED
        }

        walletRepository.insertTx(
            TxRecord(
                hash = broadcast.hash,
                height = receipt?.height ?: 0L,
                timestamp = System.currentTimeMillis(),
                fromAddress = from,
                toAddress = to,
                amount = amount,
                denom = MCParams.DENOM,
                fee = TxBuilder.DEFAULT_FEE_UMC.toLong(),
                memo = memo,
                type = type,
                status = status,
                gasUsed = receipt?.gasUsed ?: 0L,
                rawJson = android.util.Base64.encodeToString(built.txRaw, android.util.Base64.NO_WRAP),
            )
        )
        refreshAccount(from)

        return when (status) {
            TxStatus.SUCCESS -> SubmitResult(true, txHash = broadcast.hash, message = "交易已确认")
            TxStatus.PENDING -> SubmitResult(
                success = true,
                pending = true,
                txHash = broadcast.hash,
                message = "交易已广播，等待确认",
            )
            else -> SubmitResult(
                success = false,
                txHash = broadcast.hash,
                message = classify(receipt?.log.orEmpty()),
            )
        }
    }

    private suspend fun defaultAccount() = walletRepository.getDefaultAccount()

    private fun classify(rawLog: String): String {
        val log = rawLog.trim()
        return when {
            log.contains("account sequence mismatch", ignoreCase = true) ->
                "交易序号不匹配（sequence mismatch），请刷新后重试"
            log.contains("insufficient fee", ignoreCase = true) ->
                "手续费不足，请检查链上最低 gas price"
            log.contains("insufficient funds", ignoreCase = true) ->
                "余额不足，无法支付金额与手续费"
            log.contains("invalid", ignoreCase = true) ->
                "交易无效：$log"
            log.isBlank() -> "广播失败，请稍后重试"
            else -> log
        }
    }

    private fun failure(message: String) = SubmitResult(success = false, message = message)
}
