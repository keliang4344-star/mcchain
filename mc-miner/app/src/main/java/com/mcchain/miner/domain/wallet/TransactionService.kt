package com.mcchain.miner.domain.wallet

import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.TxStatus
import com.mcchain.miner.data.model.TxType
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.network.RpcClient
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 链上交易编排：账户查询 -> sequence 获取 -> 构造/签名 -> REST 广播 -> 回执轮询 -> 本地记账。
 *
 * 三条约束：
 *  - 同一钱包的 sequence 必须串行：所有提交都经由 [submitLock]，避免并发签名导致后一笔携带过期 sequence；
 *  - 每笔交易签名前都重新拉取链上 account_number / sequence，遇到 sequence mismatch 自动重签一次；
 *  - 网络不可用时把已签名交易落盘排队，由 [replayPending] 按同一 txhash 重放（链上只会执行一次）。
 */
@Singleton
class TransactionService @Inject constructor(
    private val rpcClient: RpcClient,
    private val txBuilder: TxBuilder,
    private val walletRepository: WalletRepository,
    private val securePrefs: SecurePrefs,
) {
    /** 单消费者锁：保证同一钱包的交易按提交顺序使用 sequence。 */
    private val submitLock = Mutex()

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

    suspend fun send(toAddress: String, amountUmc: Long): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val to = toAddress.trim()
        if (!to.startsWith(MCParams.BECH32_PREFIX + "1")) return@withLock failure("收款地址格式无效")
        if (to == account.address) return@withLock failure("不能向自己转账")
        if (amountUmc <= 0) return@withLock failure("金额必须大于 0")
        if (account.balance > 0 && amountUmc > account.balance) return@withLock failure("余额不足")

        submit(TxType.SEND, account.address, to, amountUmc, "") { accountNumber, sequence ->
            txBuilder.buildSendTx(to, amountUmc, accountNumber, sequence)
        }
    }

    suspend fun delegate(validatorAddress: String, amountUmc: Long): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val validator = validatorAddress.trim()
        if (!validator.startsWith(MCParams.BECH32_PREFIX + "valoper1") &&
            !validator.startsWith(MCParams.BECH32_PREFIX + "1")
        ) {
            return@withLock failure("验证人地址格式无效")
        }
        if (amountUmc <= 0) return@withLock failure("金额必须大于 0")
        if (account.balance > 0 && amountUmc > account.balance) return@withLock failure("余额不足")

        submit(TxType.DELEGATE, account.address, validator, amountUmc, "") { accountNumber, sequence ->
            txBuilder.buildDelegateTx(validator, amountUmc, accountNumber, sequence)
        }
    }

    /** 领取当前账户全部委托奖励（一笔交易包含多条 withdraw 消息）。 */
    suspend fun claimAllRewards(): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val delegations = rpcClient.getDelegations(account.address)
        if (delegations.isEmpty()) return@withLock failure("当前账户没有委托，无法领取奖励")

        val pending = rpcClient.getPendingRewards(account.address)
        if (pending <= 0) return@withLock failure("当前没有可领取的奖励")

        submit(TxType.CLAIM_REWARDS, account.address, "", pending, "claim-rewards") { accountNumber, sequence ->
            txBuilder.buildClaimRewardsTx(
                validatorAddresses = delegations.map { it.validatorAddress },
                accountNumber = accountNumber,
                sequence = sequence,
                // 每条 withdraw 约 60k gas，按委托数量动态放大上限。
                gas = TxBuilder.DEFAULT_GAS_CLAIM + 60_000L * delegations.size,
            )
        }
    }

    /**
     * 设备注册（depin.MsgRegisterDevice）。仅在同一地址首次注册时发起，
     * 成功后把地址记入安全存储，避免每次刷新都白付手续费。
     */
    suspend fun ensureDeviceRegistered(model: String, os: String): SubmitResult? = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock null
        if (securePrefs.registeredDeviceAddress == account.address) return@withLock null
        // 已有一笔注册交易在离线队列中时不再重复构造，避免同 sequence 撞车。
        if (walletRepository.getPendingTxs().any { it.type == TxType.REGISTER_DEVICE }) return@withLock null
        // 余额不足以支付手续费时先不发起，等用户充值后下次刷新自动注册。
        if (account.balance <= TxBuilder.DEFAULT_FEE_UMC.toLong()) return@withLock null

        val result = submit(TxType.REGISTER_DEVICE, account.address, account.address, 0, "register-device") {
                accountNumber, sequence ->
            txBuilder.buildRegisterDeviceTx(model, os, accountNumber, sequence)
        }
        // 链上已存在（ErrDeviceExists）视为幂等成功：重复注册不需要重试。
        if (result.success || result.message.contains("设备已上链注册")) {
            securePrefs.registeredDeviceAddress = account.address
            return@withLock SubmitResult(true, txHash = result.txHash, message = "设备已上链注册")
        }
        result
    }

    /**
     * 设备认证上链（depin.MsgAttestDevice）。
     * challenge/signature 必须来自认证预言机；同一 challenge 只能使用一次，重放会被链上拒绝。
     */
    suspend fun attestDevice(challenge: String, signatureBase64: String): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        if (challenge.isBlank() || signatureBase64.isBlank()) {
            return@withLock failure("认证材料为空，请重新发起设备认证")
        }
        submit(TxType.ATTEST_NODE, account.address, account.address, 0, "attest-device") {
                accountNumber, sequence ->
            txBuilder.buildAttestDeviceTx(challenge, signatureBase64, accountNumber, sequence)
        }
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
    suspend fun replayPending(now: Long = System.currentTimeMillis()): Int = submitLock.withLock {
        val pending = walletRepository.getPendingTxs()
        if (pending.isEmpty()) return@withLock 0

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
                if (isPermanentRejection(broadcast.code, broadcast.codespace, broadcast.log)) {
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
        resolved
    }

    /**
     * 构造并提交一笔交易。
     *
     * @param build 用链上实时 account_number / sequence 构造签名交易；
     *              遇到 sequence mismatch 时会重新拉取账户并再次调用。
     */
    private suspend fun submit(
        type: TxType,
        from: String,
        to: String,
        amount: Long,
        memo: String,
        maxAttempts: Int = 2,
        build: suspend (accountNumber: Long, sequence: Long) -> TxBuilder.BuiltTx,
    ): SubmitResult {
        var lastLog = ""
        repeat(maxAttempts) { attempt ->
            val info = rpcClient.getAccount(from)
                ?: return failure("无法获取链上账户信息，请检查网络或节点地址")
            val built = runCatching { build(info.first, info.second) }
                .getOrElse { return failure(it.message ?: "交易构造失败") }

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
                // sequence 被并发交易抢先消耗：重新拉取账户后重签一次。
                if (isSequenceMismatch(result.code, result.codespace, result.log) && attempt < maxAttempts - 1) {
                    lastLog = result.log
                    return@repeat
                }
                return failure(classify(result.code, result.codespace, result.log))
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
                    message = classify(
                        receipt?.code ?: -1,
                        receipt?.codespace ?: "",
                        receipt?.log.orEmpty(),
                    ),
                )
            }
        }
        return failure(classify(0, "", lastLog))
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

    private fun isSequenceMismatch(code: Int, codespace: String, log: String): Boolean =
        (codespace == CODESPACE_SDK &&
            (code == SDK_ERR_INVALID_SEQUENCE || code == SDK_ERR_WRONG_SEQUENCE)) ||
            log.contains("account sequence mismatch", ignoreCase = true) ||
            log.contains("incorrect account sequence", ignoreCase = true)

    private fun isInsufficientFunds(code: Int, codespace: String, log: String): Boolean =
        (codespace == CODESPACE_SDK && code == SDK_ERR_INSUFFICIENT_FUNDS) ||
            log.contains("insufficient funds", ignoreCase = true)

    private fun isInsufficientFee(code: Int, codespace: String, log: String): Boolean =
        (codespace == CODESPACE_SDK && code == SDK_ERR_INSUFFICIENT_FEE) ||
            log.contains("insufficient fee", ignoreCase = true)

    /**
     * 这些拒绝原因不会随时间自行恢复（sequence 已被消耗 / 余额永久不足 / 签名不匹配），
     * 队列里应直接置为失败，避免无限重试。
     */
    private fun isPermanentRejection(code: Int, codespace: String, log: String): Boolean =
        isSequenceMismatch(code, codespace, log) ||
            isInsufficientFunds(code, codespace, log) ||
            isInsufficientFee(code, codespace, log) ||
            log.contains("signature verification failed", ignoreCase = true)

    private suspend fun defaultAccount() = walletRepository.getDefaultAccount()

    /**
     * 把链上错误翻译成用户能理解的提示。
     *
     * 同时看 ABCI code + codespace（精确）与 raw_log 文本（兜底）：
     * 同一数字 code 在不同模块会重复（depin 1100 / phonenode 1100），
     * 所以仅在 codespace 匹配时才信任 code。
     */
    private fun classify(code: Int, codespace: String, rawLog: String): String {
        val log = rawLog.trim()
        return when {
            isSequenceMismatch(code, codespace, log) ->
                "交易序号不匹配（sequence mismatch），请刷新后重试"
            isInsufficientFee(code, codespace, log) ->
                "手续费不足，请检查链上最低 gas price"
            isInsufficientFunds(code, codespace, log) ->
                "余额不足，无法支付金额与手续费"
            (codespace == CODESPACE_DEPIN && code == DEPIN_ERR_DEVICE_EXISTS) ||
                log.contains("device already registered", ignoreCase = true) ->
                "设备已上链注册，无需重复注册"
            codespace == CODESPACE_DEPIN && code == DEPIN_ERR_DEVICE_NOT_FOUND ->
                "设备尚未上链注册，请稍后重试"
            codespace == CODESPACE_DEPIN && code == DEPIN_ERR_DEVICE_NOT_ATTESTED ->
                "设备尚未通过认证，暂时不能提交贡献"
            (codespace == CODESPACE_DEPIN &&
                (code == DEPIN_ERR_INVALID_ATTESTATION || code == DEPIN_ERR_ATTESTATION_FAILED)) ||
                log.contains("invalid attestation", ignoreCase = true) ->
                "设备认证无效或已过期，请重新认证"
            codespace == CODESPACE_DEPIN && code == DEPIN_ERR_PHONENODE_NOT_REGISTERED ->
                "当前钱包未注册为节点，请先注册节点角色"
            codespace == CODESPACE_PHONENODE && code == PHONENODE_ERR_NODE_EXISTS ->
                "节点已注册，无需重复注册"
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

        const val CODESPACE_SDK = "sdk"
        const val CODESPACE_DEPIN = "depin"
        const val CODESPACE_PHONENODE = "phonenode"

        // Cosmos SDK types/errors/errors.go
        const val SDK_ERR_INVALID_SEQUENCE = 3
        const val SDK_ERR_INSUFFICIENT_FUNDS = 5
        const val SDK_ERR_INSUFFICIENT_FEE = 13
        const val SDK_ERR_WRONG_SEQUENCE = 32

        // x/depin/types/errors.go
        const val DEPIN_ERR_DEVICE_EXISTS = 1100
        const val DEPIN_ERR_DEVICE_NOT_FOUND = 1101
        const val DEPIN_ERR_DEVICE_NOT_ATTESTED = 1105
        const val DEPIN_ERR_INVALID_ATTESTATION = 1106
        const val DEPIN_ERR_PHONENODE_NOT_REGISTERED = 1107
        const val DEPIN_ERR_ATTESTATION_FAILED = 1108

        // x/phonenode/types/errors.go
        const val PHONENODE_ERR_NODE_EXISTS = 1100
    }
}
