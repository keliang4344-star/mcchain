package com.mcchain.miner.domain.wallet

import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.TxStatus
import com.mcchain.miner.data.model.TxType
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.data.pref.SecurePrefs
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
    private val securePrefs: SecurePrefs,
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

    /**
     * 设备注册（depin.MsgRegisterDevice）。仅在同一地址首次注册时发起，
     * 成功后把地址记入安全存储，避免每次刷新都白付手续费。
     */
    suspend fun ensureDeviceRegistered(model: String, os: String): SubmitResult? {
        val account = defaultAccount() ?: return null
        if (securePrefs.registeredDeviceAddress == account.address) return null
        // 已有一笔注册交易在离线队列中时不再重复构造，避免同 sequence 撞车。
        if (walletRepository.getPendingTxs().any { it.type == TxType.REGISTER_DEVICE }) return null
        // 余额不足以支付手续费时先不发起，等用户充值后下次刷新自动注册。
        if (account.balance <= TxBuilder.DEFAULT_FEE_UMC.toLong()) return null

        val info = rpcClient.getAccount(account.address) ?: return null
        val built = runCatching {
            txBuilder.buildRegisterDeviceTx(model, os, info.first, info.second)
        }.getOrNull() ?: return null

        val result = submit(TxType.REGISTER_DEVICE, account.address, account.address, 0, "register-device", built)
        // 链上已存在（ErrDeviceExists）视为幂等成功：重复注册不需要重试。
        if (result.success || result.message.contains("device already registered", ignoreCase = true)) {
            securePrefs.registeredDeviceAddress = account.address
            return SubmitResult(true, txHash = result.txHash, message = "设备已上链注册")
        }
        return result
    }

    /**
     * 重放离线队列中的交易。
     *
     * 幂等性来自交易字节本身：同一 sequence/账户号的同一笔交易签名结果固定，
     * txhash 也固定，链上只会接受一次；重复广播会被节点以 "tx already exists in cache"
     * 或 "sequence mismatch" 拒绝，不会产生第二笔转账。
     *
     * @return 本次处理（确认成功或失败）的交易数
     */
    suspend fun replayPending(now: Long = System.currentTimeMillis()): Int {
        val pending = walletRepository.getPendingTxs()
        if (pending.isEmpty()) return 0

        var resolved = 0
        for (tx in pending) {
            val receipt = rpcClient.getTxResult(tx.hash)
            if (receipt != null) {
                walletRepository.updateTxStatus(
                    tx.hash,
                    if (receipt.code == 0) TxStatus.SUCCESS else TxStatus.FAILED,
                )
                resolved++
                continue
            }
            // 超过 TTL 仍未上链：多半是签名时 sequence 已过期，重放也不会成功。
            if (now - tx.timestamp > PENDING_TTL_MS) {
                walletRepository.updateTxStatus(tx.hash, TxStatus.FAILED)
                resolved++
                continue
            }
            val raw = runCatching {
                android.util.Base64.decode(tx.rawJson, android.util.Base64.NO_WRAP)
            }.getOrNull() ?: continue

            val broadcast = runCatching { rpcClient.broadcastTxRaw(raw) }.getOrNull() ?: continue
            if (broadcast.code != 0 && !isAlreadyKnown(broadcast.log)) {
                if (isPermanentRejection(broadcast.log)) {
                    walletRepository.updateTxStatus(tx.hash, TxStatus.FAILED)
                    resolved++
                }
                continue
            }
            val confirmed = rpcClient.waitForTx(tx.hash, timeoutMs = 10_000L)
            if (confirmed != null) {
                walletRepository.updateTxStatus(
                    tx.hash,
                    if (confirmed.code == 0) TxStatus.SUCCESS else TxStatus.FAILED,
                )
                resolved++
            }
        }
        if (resolved > 0) defaultAccount()?.let { refreshAccount(it.address) }
        return resolved
    }

    private suspend fun submit(
        type: TxType,
        from: String,
        to: String,
        amount: Long,
        memo: String,
        built: TxBuilder.BuiltTx,
    ): SubmitResult {
        val broadcast = runCatching { rpcClient.broadcastTxRaw(built.txRaw) }.getOrNull()
        val networkDown = broadcast == null || (broadcast.code < 0 && broadcast.hash.isBlank())
        if (networkDown) {
            // 已签名交易先落盘排队，网络恢复后由 replayPending() 按同一 txhash 重放。
            walletRepository.insertTx(
                record(type, from, to, amount, memo, built, TxStatus.PENDING, 0L, built.txHashHex)
            )
            return SubmitResult(
                success = true,
                pending = true,
                txHash = built.txHashHex,
                message = "网络不可用：交易已排队，恢复后自动重放",
            )
        }

        val result = broadcast!!
        val duplicate = result.code != 0 && isAlreadyKnown(result.log)
        if (result.code != 0 && !duplicate) {
            return failure(classify(result.log))
        }
        // 重复广播时节点可能不回 txhash，用本地计算值兜底。
        val txHash = result.hash.ifBlank { built.txHashHex }

        val receipt = rpcClient.waitForTx(txHash)
        val status = when {
            receipt == null -> TxStatus.PENDING
            receipt.code == 0 -> TxStatus.SUCCESS
            else -> TxStatus.FAILED
        }

        walletRepository.insertTx(
            record(type, from, to, amount, memo, built, status, receipt?.height ?: 0L, txHash,
                receipt?.gasUsed ?: 0L)
        )
        refreshAccount(from)

        return when (status) {
            TxStatus.SUCCESS -> SubmitResult(true, txHash = txHash, message = "交易已确认")
            TxStatus.PENDING -> SubmitResult(
                success = true,
                pending = true,
                txHash = txHash,
                message = "交易已广播，等待确认",
            )
            else -> SubmitResult(
                success = false,
                txHash = txHash,
                message = classify(receipt?.log.orEmpty()),
            )
        }
    }

    private fun record(
        type: TxType,
        from: String,
        to: String,
        amount: Long,
        memo: String,
        built: TxBuilder.BuiltTx,
        status: TxStatus,
        height: Long,
        hash: String,
        gasUsed: Long = 0L,
    ) = TxRecord(
        hash = hash,
        height = height,
        timestamp = System.currentTimeMillis(),
        fromAddress = from,
        toAddress = to,
        amount = amount,
        denom = MCParams.DENOM,
        fee = TxBuilder.DEFAULT_FEE_UMC.toLong(),
        memo = memo,
        type = type,
        status = status,
        gasUsed = gasUsed,
        rawJson = android.util.Base64.encodeToString(built.txRaw, android.util.Base64.NO_WRAP),
    )

    /** 同一笔交易字节被节点判定为「已在 mempool / 已上链」——幂等重放时属正常现象。 */
    private fun isAlreadyKnown(log: String): Boolean =
        log.contains("already exists", ignoreCase = true) ||
            log.contains("tx already exists in cache", ignoreCase = true)

    /**
     * 这些拒绝原因不会随时间自行恢复（sequence 已被消耗 / 余额永久不足），
     * 队列里应直接置为失败，避免无限重试。
     */
    private fun isPermanentRejection(log: String): Boolean =
        log.contains("account sequence mismatch", ignoreCase = true) ||
            log.contains("insufficient funds", ignoreCase = true) ||
            log.contains("signature verification failed", ignoreCase = true)

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

    private companion object {
        /** 离线队列保留时长：超过仍未上链视为签名时的 sequence 已过期。 */
        const val PENDING_TTL_MS = 10 * 60 * 1000L
    }
}
