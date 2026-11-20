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
    private val feeProvider: DynamicFeeProvider,
    private val walletRepository: WalletRepository,
    private val securePrefs: SecurePrefs,
) {
    /** 单消费者锁：保证同一钱包的交易按提交顺序使用 sequence。 */
    private val submitLock = Mutex()

    data class SubmitResult(
        val success: Boolean,
        val pending: Boolean = false,
        /** 链上判定为「已记录/已消费」：请求幂等收敛，不算失败也不算新成功。 */
        val idempotent: Boolean = false,
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
        validateValidator(validator)?.let { return@withLock failure(it) }
        if (amountUmc <= 0) return@withLock failure("金额必须大于 0")
        if (account.balance > 0 && amountUmc > account.balance) return@withLock failure("余额不足")

        submit(TxType.DELEGATE, account.address, validator, amountUmc, "") { accountNumber, sequence ->
            txBuilder.buildDelegateTx(validator, amountUmc, accountNumber, sequence)
        }
    }

    /** 解除委托（MsgUndelegate）：金额进入 21 天解绑期，期间不再产生奖励。 */
    suspend fun undelegate(validatorAddress: String, amountUmc: Long): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val validator = validatorAddress.trim()
        validateValidator(validator)?.let { return@withLock failure(it) }
        if (amountUmc <= 0) return@withLock failure("解除金额必须大于 0")
        ensureDelegationAmount(account.address, validator, amountUmc)?.let { return@withLock failure(it) }
        ensureFeeBalance(account.balance)?.let { return@withLock failure(it) }

        submit(TxType.UNDELEGATE, account.address, validator, amountUmc, "undelegate") {
                accountNumber, sequence ->
            txBuilder.buildUndelegateTx(validator, amountUmc, accountNumber, sequence)
        }
    }

    /** 转委托（MsgBeginRedelegate）：源与目标验证人不能相同，金额不得超过源委托数量。 */
    suspend fun redelegate(
        sourceValidator: String,
        destinationValidator: String,
        amountUmc: Long,
    ): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val source = sourceValidator.trim()
        val destination = destinationValidator.trim()
        validateValidator(source)?.let { return@withLock failure("源验证人地址无效：$it") }
        validateValidator(destination)?.let { return@withLock failure("目标验证人地址无效：$it") }
        if (source == destination) return@withLock failure("转委托的源与目标验证人不能相同")
        if (amountUmc <= 0) return@withLock failure("转委托金额必须大于 0")
        ensureDelegationAmount(account.address, source, amountUmc)?.let { return@withLock failure(it) }
        ensureFeeBalance(account.balance)?.let { return@withLock failure(it) }

        submit(TxType.REDELEGATE, account.address, "$source -> $destination", amountUmc, "redelegate") {
                accountNumber, sequence ->
            txBuilder.buildRedelegateTx(source, destination, amountUmc, accountNumber, sequence)
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
     * 移动节点注册（phonenode.MsgRegisterNode）。depin 贡献发币闸口要求地址已注册，
     * 与设备注册一样：成功后记入安全存储，ErrNodeExists 视为幂等成功。
     */
    suspend fun ensureNodeRegistered(model: String, os: String): SubmitResult? = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock null
        if (securePrefs.registeredNodeAddress == account.address) return@withLock null
        if (account.balance <= TxBuilder.DEFAULT_FEE_UMC.toLong()) return@withLock null

        val result = submit(TxType.UNKNOWN, account.address, account.address, 0, "register-node") {
                accountNumber, sequence ->
            txBuilder.buildRegisterNodeTx(model, os, accountNumber = accountNumber, sequence = sequence)
        }
        if (result.success || result.message.contains("节点已注册")) {
            securePrefs.registeredNodeAddress = account.address
            return@withLock SubmitResult(true, txHash = result.txHash, message = "节点已上链注册")
        }
        result
    }

    /** 设备自签 attestation 上链（phonenode.MsgSubmitAttestation），depin 防线 Layer2 前置条件。 */
    suspend fun submitPhonenodeAttestation(
        rootHash: String,
        nonce: String,
        deviceIdHash: String,
        devicePubKeyHex: String,
        signatureHex: String,
    ): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        submit(TxType.ATTEST_NODE, account.address, account.address, 0, "attest-node") {
                accountNumber, sequence ->
            txBuilder.buildPhonenodeAttestTx(
                rootHash, nonce, deviceIdHash, devicePubKeyHex, signatureHex,
                accountNumber, sequence,
            )
        }
    }

    /**
     * 贡献提交（depin.MsgSubmitContribution）。
     *
     * 先认证后提交：设备认证（链上 Attested 且未过期）是计奖前置条件，未认证直接跳过，
     * 避免产生注定被拒的交易。taskId 是链上幂等键：重复提交返回 ErrTaskExists，
     * 这里收敛为 idempotent 成功，调用方不得重试。
     */
    suspend fun submitContribution(taskId: String, chainTaskType: String, score: Int): SubmitResult =
        submitLock.withLock {
            val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
            if (taskId.isBlank() || taskId.length > MAX_TASK_ID_LENGTH) {
                return@withLock failure("贡献幂等键长度无效（1..$MAX_TASK_ID_LENGTH）")
            }
            if (score !in 0..100) return@withLock failure("分数越界：score 必须在 [0,100]")

            val result = submit(TxType.REPORT_CONTRIB, account.address, "", 0, "contribution") {
                    accountNumber, sequence ->
                txBuilder.buildSubmitContributionTx(taskId, chainTaskType, score, accountNumber, sequence)
            }
            if (result.message.contains("贡献已记录")) {
                return@withLock SubmitResult(
                    success = true,
                    idempotent = true,
                    txHash = result.txHash,
                    message = "贡献已记录（幂等收敛）",
                )
            }
            result
        }

    /**
     * 在线心跳（phonenode.MsgSubmitStateProof）。
     * 注册节点若超过 OfflineGraceBlocks（100 块 ≈ 400s）不心跳会被离线 slash：
     * attestation 被吊销且进入冷却期禁止重认证，设备彻底失去接单/计奖资格。
     */
    suspend fun sendHeartbeat(deviceId: String): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val height = rpcClient.getLatestBlockHeight()
        val leaf = "phonenode|$deviceId|$height|${System.currentTimeMillis()}"
        submit(TxType.UNKNOWN, account.address, account.address, 0, "") { accountNumber, sequence ->
            txBuilder.buildHeartbeatTx(leaf, accountNumber, sequence)
        }
    }

    /** EdgeAI 结果提交（edgeai.MsgSubmitResult）。attestationNonce 用 depin 认证的 challenge。 */
    suspend fun submitEdgeAiResult(taskId: String, resultHash: String, attestationNonce: String): SubmitResult =
        submitLock.withLock {
            val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
            if (taskId.isBlank() || resultHash.isBlank()) {
                return@withLock failure("任务 ID 或结果指纹为空")
            }
            submit(TxType.SUBMIT_TASK, account.address, "", 0, "edgeai-result") { accountNumber, sequence ->
                txBuilder.buildSubmitResultTx(taskId, resultHash, attestationNonce, accountNumber, sequence)
            }
        }

    /**
     * 邀请关系绑定（referral.MsgCreateReferral）。
     * 链上要求被邀请人已注册节点、未被他人邀请、不能自邀，且同一邀请人存在冷却区块。
     */
    suspend fun createReferral(inviteeAddress: String, inviteCode: String): SubmitResult =
        submitLock.withLock {
            val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
            val target = inviteeAddress.trim()
            if (!target.startsWith(MCParams.BECH32_PREFIX + "1") ||
                target.startsWith(MCParams.BECH32_PREFIX + "valoper1")
            ) {
                return@withLock failure("被邀请人地址无效，必须是 ${MCParams.BECH32_PREFIX}1 开头的账户地址")
            }
            if (target == account.address) return@withLock failure("不能邀请自己")
            if (inviteCode.length > MAX_INVITE_CODE_LENGTH) {
                return@withLock failure("渠道码过长（最多 $MAX_INVITE_CODE_LENGTH 个字符）")
            }
            ensureFeeBalance(account.balance)?.let { return@withLock failure(it) }

            submit(TxType.REFERRAL, account.address, target, 0, "referral-create") {
                    accountNumber, sequence ->
                txBuilder.buildCreateReferralTx(target, inviteCode.trim(), accountNumber, sequence)
            }
        }

    /** 领取累计返佣（referral.MsgClaimReferralReward）。 */
    suspend fun claimReferralRewards(): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val pending = rpcClient.getReferralPendingRewards(account.address)
            ?: return@withLock failure("无法查询待领返佣，请检查节点连接")
        if (pending <= 0) return@withLock failure("当前没有可领取的返佣")
        ensureFeeBalance(account.balance)?.let { return@withLock failure(it) }

        submit(TxType.CLAIM_REFERRAL, account.address, "", pending, "referral-claim") {
                accountNumber, sequence ->
            txBuilder.buildClaimReferralTx(accountNumber, sequence)
        }
    }

    /**
     * 发布 EdgeAI 任务（edgeai.MsgCreateTask）：奖励由创建者托管到模块账户。
     * 本地先校验余额是否覆盖「奖励 + 手续费」，避免发起注定失败的托管交易。
     */
    suspend fun createEdgeAiTask(description: String, rewardUmc: Long): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val desc = description.trim()
        if (desc.isBlank()) return@withLock failure("任务描述不能为空")
        if (desc.length > MAX_TASK_DESCRIPTION_LENGTH) {
            return@withLock failure("任务描述过长（最多 $MAX_TASK_DESCRIPTION_LENGTH 个字符）")
        }
        if (rewardUmc < 0) return@withLock failure("任务奖励不能为负数")
        if (account.balance > 0) {
            val fee = TxBuilder.DEFAULT_FEE_UMC.toLong()
            if (rewardUmc > account.balance - fee || account.balance < fee) {
                return@withLock failure("余额不足以同时托管任务奖励与支付手续费")
            }
        }

        submit(TxType.EDGEAI_TASK, account.address, "", rewardUmc, "edgeai-create") {
                accountNumber, sequence ->
            txBuilder.buildCreateTaskTx(desc, rewardUmc, accountNumber, sequence)
        }
    }

    /** 对 EdgeAI 任务结果发起争议，由链上仲裁者裁定（edgeai.MsgOpenDispute）。 */
    suspend fun openEdgeAiDispute(taskId: String, reason: String): SubmitResult = submitLock.withLock {
        val account = defaultAccount() ?: return@withLock failure("钱包未初始化，请先创建或导入助记词")
        val id = taskId.trim()
        val text = reason.trim()
        if (id.isBlank()) return@withLock failure("请输入任务 ID")
        if (text.isBlank()) return@withLock failure("请填写争议理由")
        if (text.length > MAX_DISPUTE_REASON_LENGTH) {
            return@withLock failure("争议理由过长（最多 $MAX_DISPUTE_REASON_LENGTH 个字符）")
        }
        ensureFeeBalance(account.balance)?.let { return@withLock failure(it) }

        submit(TxType.EDGEAI_DISPUTE, account.address, id, 0, "edgeai-dispute") {
                accountNumber, sequence ->
            txBuilder.buildOpenDisputeTx(id, text, accountNumber, sequence)
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
        maxAttempts: Int = 3,
        build: suspend (accountNumber: Long, sequence: Long) -> TxBuilder.BuiltTx,
    ): SubmitResult {
        var lastLog = ""
        // 签名前强校验链 ID：节点与 App 配置不一致时直接拒绝，避免签出一笔注定被拒的交易。
        ensureChainMatchesNode()?.let { return failure(it) }
        feeProvider.refresh()
        repeat(maxAttempts) { attempt ->
            val info = rpcClient.getAccount(from)
                ?: return failure("无法获取链上账户信息，请检查网络或节点地址")
            val built = runCatching { build(info.first, info.second) }
                .getOrElse { return failure(it.message ?: "交易构造失败") }

            // 动态手续费可能高于界面展示的最低值；广播前用链上余额做一次硬校验。
            val liquid = rpcClient.getBalanceOrNull(from)
            if (liquid != null && liquid < built.feeUmc) {
                return failure("可用余额不足以支付网络手续费（需 ${built.feeUmc} umc），请先充值")
            }

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
                // sequence 被并发交易抢先消耗，或查询状态落后一个区块：
                // 等一个出块级间隔再重取账户，避免立刻拿到同样的过期值。
                if (isSequenceMismatch(result.code, result.codespace, result.log) && attempt < maxAttempts - 1) {
                    lastLog = result.log
                    kotlinx.coroutines.delay(2_000)
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
        fee = built.feeUmc,
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
     * 节点链 ID 必须与安装包配置一致；节点不可达时返回 null，交由后续账户查询/广播判定，
     * 以保留「先签名、网络恢复后重放」的离线队列行为。
     */
    private suspend fun ensureChainMatchesNode(): String? {
        val status = rpcClient.getNodeStatus() ?: return null
        if (status.chainId.isBlank()) return null
        if (status.chainId != MCParams.CHAIN_ID) {
            return "节点链 ID 为 ${status.chainId}，与 App 配置的 ${MCParams.CHAIN_ID} 不一致，" +
                "已拒绝签名。请切换正确节点，或安装对应网络的安装包"
        }
        return null
    }

    private fun validateValidator(address: String): String? = when {
        address.isEmpty() -> "请输入验证人地址"
        !address.startsWith(MCParams.BECH32_PREFIX + "valoper1") ->
            "验证人地址必须以 ${MCParams.BECH32_PREFIX}valoper1 开头"
        address.length !in 48..62 -> "验证人地址长度异常，请检查是否粘贴完整"
        !com.mcchain.miner.shared.util.Bech32Address.hasValidDataPart(
            address, MCParams.BECH32_PREFIX + "valoper"
        ) -> "验证人地址包含非法字符"
        else -> null
    }

    /**
     * 校验解除/转委托金额不超过该验证人的链上委托数量。
     * 委托列表为空时可能是节点不可达，交由链上最终裁决，避免误伤离线场景。
     */
    private suspend fun ensureDelegationAmount(
        address: String,
        validatorAddress: String,
        amountUmc: Long,
    ): String? {
        val delegations = rpcClient.getDelegations(address)
        if (delegations.isEmpty()) return null
        val delegation = delegations.firstOrNull { it.validatorAddress == validatorAddress }
            ?: return "当前没有向该验证人的委托"
        if (amountUmc > delegation.amount) {
            return "金额超过该验证人的委托数量，请先刷新后重试"
        }
        return null
    }

    private fun ensureFeeBalance(balance: Long): String? =
        if (balance < TxBuilder.DEFAULT_FEE_UMC.toLong()) {
            "可用余额不足以支付网络手续费，请先充值"
        } else {
            null
        }

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
            (codespace == CODESPACE_DEPIN && code == DEPIN_ERR_TASK_EXISTS) ||
                log.contains("contribution task id already exists", ignoreCase = true) ||
                log.contains("task id already exists", ignoreCase = true) ->
                "贡献已记录（幂等收敛）"
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
            codespace == CODESPACE_PHONENODE && code == PHONENODE_ERR_NONCE_REUSED ->
                "认证 nonce 已被使用，请重新发起认证"
            codespace == CODESPACE_PHONENODE && code == PHONENODE_ERR_DEVICE_ALREADY_BOUND ->
                "设备已绑定其他节点地址，拒绝重复绑定"
            (codespace == CODESPACE_PHONENODE && code == PHONENODE_ERR_INVALID_ATTESTATION) ||
                log.contains("attestation signature verification failed", ignoreCase = true) ->
                "设备自签认证验签失败，请重试认证"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_TASK_NOT_FOUND ->
                "任务不存在或已被清理"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_TASK_NOT_OPEN ->
                "任务已不再开放（已被结算或过期）"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_TASK_ALREADY_DONE ->
                "任务已完成（幂等收敛）"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_DUPLICATE_RESULT ->
                "本节点已提交过该任务结果（幂等收敛）"
            codespace == CODESPACE_EDGEAI && code == EDGEAI_ERR_NOT_ASSIGNED ->
                "任务已分配给其他节点"
            (codespace == CODESPACE_SDK && code == SDK_ERR_UNAUTHORIZED) ||
                log.contains("node not attested", ignoreCase = true) ->
                "节点未认证或认证已过期，请先完成设备认证"
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

        /** depin keeper 对 task_id 的长度闸门（x/depin/types/keys.go）。 */
        const val MAX_TASK_ID_LENGTH = 128

        /** 与链端一致的输入上限（超出会在签名前直接拒绝，避免白付手续费）。 */
        const val MAX_INVITE_CODE_LENGTH = 32
        const val MAX_TASK_DESCRIPTION_LENGTH = 500
        const val MAX_DISPUTE_REASON_LENGTH = 200

        const val CODESPACE_SDK = "sdk"
        const val CODESPACE_DEPIN = "depin"
        const val CODESPACE_PHONENODE = "phonenode"
        const val CODESPACE_EDGEAI = "edgeai"

        // Cosmos SDK types/errors/errors.go
        const val SDK_ERR_INVALID_SEQUENCE = 3
        const val SDK_ERR_UNAUTHORIZED = 4
        const val SDK_ERR_INSUFFICIENT_FUNDS = 5
        const val SDK_ERR_INSUFFICIENT_FEE = 13
        const val SDK_ERR_WRONG_SEQUENCE = 32

        // x/depin/types/errors.go
        const val DEPIN_ERR_DEVICE_EXISTS = 1100
        const val DEPIN_ERR_DEVICE_NOT_FOUND = 1101
        const val DEPIN_ERR_TASK_EXISTS = 1102
        const val DEPIN_ERR_DEVICE_NOT_ATTESTED = 1105
        const val DEPIN_ERR_INVALID_ATTESTATION = 1106
        const val DEPIN_ERR_PHONENODE_NOT_REGISTERED = 1107
        const val DEPIN_ERR_ATTESTATION_FAILED = 1108

        // x/phonenode/types/errors.go
        const val PHONENODE_ERR_NODE_EXISTS = 1100
        const val PHONENODE_ERR_NONCE_REUSED = 1104
        const val PHONENODE_ERR_DEVICE_ALREADY_BOUND = 1105
        const val PHONENODE_ERR_INVALID_ATTESTATION = 1106

        // x/edgeai/types/errors.go
        const val EDGEAI_ERR_TASK_NOT_FOUND = 1200
        const val EDGEAI_ERR_TASK_NOT_OPEN = 1201
        const val EDGEAI_ERR_TASK_ALREADY_DONE = 1202
        const val EDGEAI_ERR_DUPLICATE_RESULT = 1203
        const val EDGEAI_ERR_NOT_ASSIGNED = 1206
    }
}
