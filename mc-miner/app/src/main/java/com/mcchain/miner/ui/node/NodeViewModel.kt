package com.mcchain.miner.ui.node

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.db.BlockDao
import com.mcchain.miner.data.db.NodeStatusDao
import com.mcchain.miner.data.db.PeerDao
import com.mcchain.miner.data.db.PhoneNodeDao
import com.mcchain.miner.data.model.NodeStatus
import com.mcchain.miner.data.model.NodeStatusSnapshot
import com.mcchain.miner.data.model.PeerNode
import com.mcchain.miner.data.model.PhoneNodeRecord
import com.mcchain.miner.data.model.TxStatus
import com.mcchain.miner.data.model.TxType
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.domain.node.AndroidKeyAttestation
import com.mcchain.miner.domain.node.AttestationResult
import com.mcchain.miner.domain.node.DeviceAttestation
import com.mcchain.miner.domain.node.HardwareCheck
import com.mcchain.miner.domain.wallet.TransactionService
import com.mcchain.miner.domain.wallet.TxBuilder
import com.mcchain.miner.network.RpcClient
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import javax.inject.Inject

/** 认证流程中的五个真实步骤。 */
enum class AuthStepId { WALLET, DEVICE, NODE, PROOF, SUBMIT }

/** 步骤状态：已完成 / 进行中（当前应执行的一步） / 未开始。 */
enum class StepState { DONE, CURRENT, TODO }

data class AuthStep(
    val id: AuthStepId,
    val index: Int,
    val title: String,
    val description: String,
    val state: StepState,
    /** 该步骤的真实进度描述（来自链上回执 / 安全存储 / Room 记录）。 */
    val detail: String,
)

/**
 * 安全存储中的本机认证信息快照。
 * 读取失败（Keystore 初始化失败）时保持空值，不猜测、不伪造。
 */
data class NodeLocalInfo(
    val deviceId: String = "",
    val attestationExpiry: Long = 0L,
    val lastHeartbeat: Long = 0L,
    val registeredDeviceAddress: String? = null,
    val registeredNodeAddress: String? = null,
    val walletAddress: String? = null,
)

/**
 * 硬件证明能力：在 AndroidKeyStore 中真实生成一次绑定 challenge 的密钥，
 * 读取证书扩展里的 securityLevel（TEE / StrongBox），随后立即删除探测密钥。
 */
data class AttestationCapability(
    val level: String,
    val summary: String,
    val strongBoxSupported: Boolean,
)

/**
 * 节点与设备认证板块。
 *
 * 数据来源约束：
 *  - 节点状态：链上回执写入的 Room 记录（PhoneNodeRecord.status），没有记录即「未注册」；
 *  - 认证有效期 / 心跳：SecurePrefs 中真实写入的时间戳；
 *  - 链上状态快照：RPC `status` 实时查询，失败时回退到 Room 缓存并明确标注。
 * 所有失败路径只展示中文原因，不产生本地「伪成功」。
 */
@HiltViewModel
class NodeViewModel @Inject constructor(
    @ApplicationContext private val context: Context,
    private val nodeStatusDao: NodeStatusDao,
    private val phoneNodeDao: PhoneNodeDao,
    private val peerDao: PeerDao,
    private val blockDao: BlockDao,
    private val deviceAttestation: DeviceAttestation,
    private val keyAttestation: AndroidKeyAttestation,
    private val securePrefs: SecurePrefs,
    private val walletRepository: WalletRepository,
    private val transactionService: TransactionService,
    private val rpcClient: RpcClient,
) : ViewModel() {

    private val _nodeStatus = MutableStateFlow<NodeStatusSnapshot?>(null)
    val nodeStatus: StateFlow<NodeStatusSnapshot?> = _nodeStatus

    private val _phoneNode = MutableStateFlow<PhoneNodeRecord?>(null)
    val phoneNode: StateFlow<PhoneNodeRecord?> = _phoneNode

    private val _peers = MutableStateFlow<List<PeerNode>>(emptyList())
    val peers: StateFlow<List<PeerNode>> = _peers

    private val _hardwareCheck = MutableStateFlow<HardwareCheck?>(null)
    val hardwareCheck: StateFlow<HardwareCheck?> = _hardwareCheck

    private val _capability = MutableStateFlow<AttestationCapability?>(null)
    val capability: StateFlow<AttestationCapability?> = _capability

    private val _localInfo = MutableStateFlow(NodeLocalInfo())
    val localInfo: StateFlow<NodeLocalInfo> = _localInfo

    private val _steps = MutableStateFlow<List<AuthStep>>(emptyList())
    val steps: StateFlow<List<AuthStep>> = _steps

    /** 当前正在执行的步骤；null 表示空闲。 */
    private val _busyStep = MutableStateFlow<AuthStepId?>(null)
    val busyStep: StateFlow<AuthStepId?> = _busyStep

    /** 最近一次操作的提示或失败原因（全部中文）。 */
    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message

    private val _messageIsError = MutableStateFlow(false)
    val messageIsError: StateFlow<Boolean> = _messageIsError

    private val _isRefreshing = MutableStateFlow(false)
    val isRefreshing: StateFlow<Boolean> = _isRefreshing

    /** 链上刷新失败原因；null 表示没有错误。 */
    private val _refreshError = MutableStateFlow<String?>(null)
    val refreshError: StateFlow<String?> = _refreshError

    /** 当前展示的节点状态是否来自实时 RPC（false 表示 Room 缓存）。 */
    private val _isLive = MutableStateFlow(false)
    val isLive: StateFlow<Boolean> = _isLive

    /** 第 ④ 步产出的硬件证明；只保存在内存，应用重启后必须重新证明。 */
    private val _pendingProof = MutableStateFlow<AttestationResult?>(null)

    /** 已成功上链的 phonenode 自签 nonce，避免第 ⑤ 步重试时重复提交同一 nonce。 */
    private var acceptedNodeAttestNonce: String? = null

    private var capabilityProbed = false

    init {
        load()
        refresh()
    }

    fun load() {
        viewModelScope.launch {
            if (_nodeStatus.value == null) _nodeStatus.value = nodeStatusDao.get()
            reloadLocal()
            if (!capabilityProbed) {
                capabilityProbed = true
                _capability.value = probeCapability()
            }
        }
    }

    /**
     * 手动刷新：查询实时链上节点状态；失败时保留 Room 缓存并给出可操作的指引。
     */
    fun refresh() {
        if (_isRefreshing.value) return
        _isRefreshing.value = true
        viewModelScope.launch {
            _refreshError.value = null
            try {
                val live = withContext(Dispatchers.IO) { rpcClient.getNodeStatus() }
                if (live != null) {
                    withContext(Dispatchers.IO) { nodeStatusDao.upsert(live) }
                    _nodeStatus.value = live
                    _isLive.value = true
                } else {
                    _nodeStatus.value = withContext(Dispatchers.IO) { nodeStatusDao.get() }
                    _isLive.value = false
                    _refreshError.value = "无法连接 RPC 节点，请到「设置 → RPC 端点」配置并测试连接"
                }
            } catch (e: Exception) {
                _nodeStatus.value = withContext(Dispatchers.IO) { nodeStatusDao.get() }
                _isLive.value = false
                _refreshError.value = "刷新失败：${e.message ?: e.javaClass.simpleName}"
            } finally {
                reloadLocal()
                _isRefreshing.value = false
            }
        }
    }

    /**
     * 执行指定步骤。前置步骤未完成时直接拒绝，并提示应先完成哪一步。
     */
    fun runStep(step: AuthStepId) {
        if (_busyStep.value != null) return
        viewModelScope.launch {
            setMessage(null)
            val stepsNow = _steps.value
            val index = stepsNow.indexOfFirst { it.id == step }
            if (index < 0) return@launch

            val blocking = stepsNow.take(index).firstOrNull { it.state != StepState.DONE }
            if (blocking != null) {
                setMessage("请先完成第 ${blocking.index} 步「${blocking.title}」", true)
                return@launch
            }
            val stepNow = stepsNow[index]
            val rerunnable = step == AuthStepId.PROOF ||
                (step == AuthStepId.SUBMIT && _pendingProof.value != null)
            if (stepNow.state == StepState.DONE && !rerunnable) {
                setMessage("第 ${stepNow.index} 步「${stepNow.title}」已完成，无需重复操作")
                return@launch
            }

            _busyStep.value = step
            try {
                when (step) {
                    AuthStepId.WALLET -> setMessage("请到「钱包」页创建或导入钱包，完成后返回本页刷新")
                    AuthStepId.DEVICE -> registerDevice()
                    AuthStepId.NODE -> registerNode()
                    AuthStepId.PROOF -> proveHardware()
                    AuthStepId.SUBMIT -> submitAttestation()
                }
            } catch (e: Exception) {
                setMessage("操作失败：${e.message ?: e.javaClass.simpleName}", true)
            } finally {
                _busyStep.value = null
                reloadLocal()
            }
        }
    }

    // ------------------------------------------------------------------
    // 各步骤的真实实现
    // ------------------------------------------------------------------

    /** ② 设备注册（depin 模块的设备注册消息）。 */
    private suspend fun registerDevice() {
        val account = walletRepository.getDefaultAccount()
        if (account == null) {
            setMessage("请先完成第 ① 步：创建钱包", true)
            return
        }
        val registered = runCatching { securePrefs.registeredDeviceAddress }.getOrNull()
        if (registered == account.address) {
            setMessage("设备已上链注册，无需重复操作")
            return
        }
        if (!rpcReachable()) return
        if (account.balance <= TxBuilder.DEFAULT_FEE_UMC.toLong()) {
            setMessage(
                "钱包余额不足，无法支付注册手续费（最低需 ${TxBuilder.DEFAULT_FEE_UMC} umc），请先充值",
                true,
            )
            return
        }

        setMessage("正在提交设备注册交易…")
        val result = withContext(Dispatchers.IO) {
            transactionService.ensureDeviceRegistered(
                model = Build.MODEL,
                os = "Android ${Build.VERSION.RELEASE}",
            )
        }
        if (result == null) {
            setMessage(
                "设备注册未发起：可能有注册交易正在队列中，或余额不足，请稍后刷新重试",
                true,
            )
            return
        }
        when {
            result.success && result.pending -> setMessage("设备注册交易已广播，等待链上确认")
            result.success -> setMessage("设备已上链注册")
            else -> setMessage("设备注册失败：${result.message}", true)
        }
    }

    /** ③ 节点注册（phonenode.MsgRegisterNode）。 */
    private suspend fun registerNode() {
        val account = walletRepository.getDefaultAccount()
        if (account == null) {
            setMessage("请先完成第 ① 步：创建钱包", true)
            return
        }
        val registered = runCatching { securePrefs.registeredNodeAddress }.getOrNull()
        if (registered == account.address) {
            setMessage("节点已上链注册，无需重复操作")
            return
        }
        if (!rpcReachable()) return
        if (account.balance <= TxBuilder.DEFAULT_FEE_UMC.toLong()) {
            setMessage(
                "钱包余额不足，无法支付注册手续费（最低需 ${TxBuilder.DEFAULT_FEE_UMC} umc），请先充值",
                true,
            )
            return
        }

        setMessage("正在提交节点注册交易…")
        val result = withContext(Dispatchers.IO) {
            transactionService.ensureNodeRegistered(
                model = Build.MODEL,
                os = "Android ${Build.VERSION.RELEASE}",
            )
        }
        if (result == null) {
            setMessage("节点注册未发起：请确认网络可用且余额足够支付手续费后重试", true)
            return
        }
        when {
            result.success && result.pending -> setMessage("节点注册交易已广播，等待链上确认")
            result.success -> setMessage("节点已上链注册")
            else -> setMessage("节点注册失败：${result.message}", true)
        }
    }

    /** ④ 硬件证明（AndroidKeyStore 出证 + 预言机签名）。 */
    private suspend fun proveHardware() {
        val account = walletRepository.getDefaultAccount()
        if (account == null) {
            setMessage("请先完成第 ① 步：创建钱包", true)
            return
        }
        val oracle = runCatching { securePrefs.oracleEndpoint }.getOrNull().orEmpty()
        if (oracle.isBlank()) {
            setMessage("未配置认证预言机，请到「设置 → 认证预言机」填写 /sign 地址后再试", true)
            return
        }

        setMessage("正在生成硬件证明并请求预言机签名…")
        val result = withContext(Dispatchers.IO) {
            deviceAttestation.performAttestation(account.address)
        }
        if (!result.success) {
            setMessage(result.error ?: "硬件证明失败，请重试", true)
            return
        }
        _pendingProof.value = result
        acceptedNodeAttestNonce = null
        setMessage("硬件证明已通过（${levelLabel(result.securityLevel)}），请继续第 ⑤ 步提交认证")
    }

    /** ⑤ 提交认证（phonenode 自签 + depin.MsgAttestDevice）。 */
    private suspend fun submitAttestation() {
        val proof = _pendingProof.value
        if (proof == null) {
            setMessage("本地缺少本次认证材料，请重新执行第 ④ 步生成证明后再提交（应用重启后材料不保留）", true)
            return
        }
        val account = walletRepository.getDefaultAccount()
        if (account == null) {
            setMessage("请先完成第 ① 步：创建钱包", true)
            return
        }
        if (!rpcReachable()) return

        val alreadyAccepted = acceptedNodeAttestNonce == proof.selfNonce
        if (!alreadyAccepted) {
            setMessage("正在提交节点自签认证…")
            val nodeAttest = withContext(Dispatchers.IO) {
                transactionService.submitPhonenodeAttestation(
                    rootHash = proof.rootHash,
                    nonce = proof.selfNonce,
                    deviceIdHash = proof.deviceIdHash,
                    devicePubKeyHex = proof.devicePubKeyHex,
                    signatureHex = proof.selfSignatureHex,
                )
            }
            if (!nodeAttest.success) {
                setMessage("节点认证上链失败：${nodeAttest.message}", true)
                return
            }
            acceptedNodeAttestNonce = proof.selfNonce
        }

        setMessage("正在提交设备认证…")
        val submit = withContext(Dispatchers.IO) {
            transactionService.attestDevice(proof.challenge, proof.signatureBase64)
        }
        if (!submit.success) {
            setMessage("设备认证上链失败：${submit.message}", true)
            return
        }

        val previous = _phoneNode.value
        val now = System.currentTimeMillis()
        val pending = submit.pending
        phoneNodeDao.upsert(
            PhoneNodeRecord(
                deviceId = proof.deviceId.ifBlank { _localInfo.value.deviceId },
                nodeAddress = account.address,
                attestationToken = proof.token,
                attestationTime = now,
                attestationExpiry = now + MCParams.PHONENODE_ATTESTATION_VALIDITY_SECONDS * 1000,
                // 心跳时间只来自真实发送记录，不在这里预写。
                lastHeartbeat = previous?.lastHeartbeat ?: 0L,
                status = if (pending) NodeStatus.PENDING else NodeStatus.ACTIVE,
                // 质押数据只保留链上已有记录，未查到即 0，不写死最小质押。
                stakedAmount = previous?.stakedAmount ?: 0L,
                slashCount = previous?.slashCount ?: 0,
                reputation = previous?.reputation ?: 100,
            )
        )
        _pendingProof.value = null
        setMessage(
            if (pending) {
                "认证交易已广播，等待链上确认（当前状态：认证中）"
            } else {
                "设备已通过 ${levelLabel(proof.securityLevel)} 认证并在链上登记"
            }
        )
    }

    // ------------------------------------------------------------------
    // 本地数据装载与步骤推导
    // ------------------------------------------------------------------

    /**
     * 「认证中」→「在线」只依据链上回执推进：
     * 仅当交易队列中不再有未确认的认证交易、且存在本次提交之后确认成功的认证回执时才升级状态。
     * @return 是否发生了状态升级。
     */
    private suspend fun promotePendingNodeIfConfirmed(
        record: PhoneNodeRecord?,
        walletAddress: String?,
    ): Boolean {
        if (record == null || record.status != NodeStatus.PENDING) return false
        val address = walletAddress ?: return false
        val attestTxs = runCatching {
            walletRepository.getTransactionsForAddress(address, 50)
        }.getOrNull().orEmpty().filter { it.type == TxType.ATTEST_NODE }
        if (attestTxs.isEmpty()) return false

        val stillPending = attestTxs.any { it.status == TxStatus.PENDING }
        val confirmedAfterSubmit = attestTxs.any {
            it.status == TxStatus.SUCCESS && it.timestamp >= record.attestationTime - 15 * 60_000L
        }
        if (!stillPending && confirmedAfterSubmit) {
            phoneNodeDao.updateStatus(record.deviceId, NodeStatus.ACTIVE, System.currentTimeMillis())
            return true
        }
        return false
    }

    private suspend fun reloadLocal() {
        val account = runCatching { walletRepository.getDefaultAccount() }.getOrNull()
        val local = runCatching {
            NodeLocalInfo(
                deviceId = securePrefs.deviceId.orEmpty(),
                attestationExpiry = securePrefs.attestationExpiry,
                lastHeartbeat = securePrefs.lastHeartbeatTime,
                registeredDeviceAddress = securePrefs.registeredDeviceAddress,
                registeredNodeAddress = securePrefs.registeredNodeAddress,
                walletAddress = account?.address,
            )
        }.getOrElse {
            // 安全存储不可用：只展示钱包信息，其余留空并让 UI 提示失败原因。
            NodeLocalInfo(walletAddress = account?.address)
        }

        val rawRecord = local.deviceId
            .takeIf { it.isNotBlank() }
            ?.let { runCatching { phoneNodeDao.getByDeviceId(it) }.getOrNull() }
        val record = if (promotePendingNodeIfConfirmed(rawRecord, local.walletAddress)) {
            runCatching { phoneNodeDao.getByDeviceId(local.deviceId) }.getOrNull()
        } else {
            rawRecord
        }

        _localInfo.value = local
        _phoneNode.value = record
        _peers.value = runCatching { peerDao.getAllPeers() }.getOrDefault(emptyList())
        _hardwareCheck.value = withContext(Dispatchers.IO) {
            runCatching { deviceAttestation.checkHardwareRequirements() }.getOrNull()
        }
        _steps.value = buildSteps(local, record)
    }

    private fun buildSteps(local: NodeLocalInfo, record: PhoneNodeRecord?): List<AuthStep> {
        val now = System.currentTimeMillis()
        val walletDone = !local.walletAddress.isNullOrBlank()
        val deviceDone = walletDone && local.registeredDeviceAddress == local.walletAddress
        val nodeDone = walletDone && local.registeredNodeAddress == local.walletAddress
        val proofDone = _pendingProof.value != null ||
            (local.deviceId.isNotBlank() && local.attestationExpiry > now)
        val submitDone = record != null

        val doneFlags = listOf(walletDone, deviceDone, nodeDone, proofDone, submitDone)
        val currentIndex = doneFlags.indexOfFirst { !it }

        val base = listOf(
            AuthStep(
                id = AuthStepId.WALLET,
                index = 1,
                title = "创建钱包",
                description = "在「钱包」页创建或导入钱包，助记词由您本人离线备份",
                state = StepState.TODO,
                detail = if (walletDone) {
                    "钱包地址 ${shortAddress(local.walletAddress.orEmpty())}"
                } else {
                    "尚未创建钱包"
                },
            ),
            AuthStep(
                id = AuthStepId.DEVICE,
                index = 2,
                title = "注册设备",
                description = "把本机设备指纹登记到链上（depin 模块注册设备）",
                state = StepState.TODO,
                detail = if (deviceDone) "设备已上链注册" else "未注册",
            ),
            AuthStep(
                id = AuthStepId.NODE,
                index = 3,
                title = "注册节点",
                description = "登记移动节点身份（phonenode 模块注册节点，贡献发币的前置条件）",
                state = StepState.TODO,
                detail = if (nodeDone) "节点已上链注册" else "未注册",
            ),
            AuthStep(
                id = AuthStepId.PROOF,
                index = 4,
                title = "硬件证明",
                description = "在 TEE（可信执行环境）或 StrongBox（独立安全芯片）中生成绑定一次性随机数（challenge）的密钥，并取得认证预言机签名",
                state = StepState.TODO,
                detail = when {
                    _pendingProof.value != null -> "本次证明材料已就绪，等待提交"
                    proofDone -> "证明材料有效，到期时间见上方认证详情"
                    else -> "未完成"
                },
            ),
            AuthStep(
                id = AuthStepId.SUBMIT,
                index = 5,
                title = "提交认证",
                description = "提交节点自签认证与 depin 设备认证，等待链上确认",
                state = StepState.TODO,
                detail = if (submitDone) "已提交（${statusLabel(record?.status)}）" else "未提交",
            ),
        )

        return base.mapIndexed { index, step ->
            val state = when {
                doneFlags[index] -> StepState.DONE
                index == currentIndex -> StepState.CURRENT
                else -> StepState.TODO
            }
            step.copy(state = state)
        }
    }

    // ------------------------------------------------------------------
    // 能力探测与工具
    // ------------------------------------------------------------------

    private suspend fun probeCapability(): AttestationCapability = withContext(Dispatchers.IO) {
        val strongBox = Build.VERSION.SDK_INT >= Build.VERSION_CODES.P &&
            runCatching {
                context.packageManager.hasSystemFeature(PackageManager.FEATURE_STRONGBOX_KEYSTORE)
            }.getOrDefault(false)

        val alias = "mc-capability-probe"
        keyAttestation.deleteKey(alias)
        val probe = runCatching {
            keyAttestation.generate(alias, "capability|$alias".toByteArray(Charsets.UTF_8))
        }
        // 探测密钥只用于读取证书中的安全等级，立即删除，不留下长期密钥。
        keyAttestation.deleteKey(alias)

        probe.fold(
            onSuccess = { attested ->
                val level = attested.securityLevel.name
                AttestationCapability(
                    level = level,
                    summary = when (level) {
                        "STRONGBOX" -> "StrongBox（独立安全芯片）可用"
                        "TEE" -> "TEE（可信执行环境）可用"
                        else -> "仅软件级密钥，无法满足上链认证要求"
                    },
                    strongBoxSupported = strongBox,
                )
            },
            onFailure = { e ->
                AttestationCapability(
                    level = "UNKNOWN",
                    summary = "硬件证明能力检测失败：${e.message ?: e.javaClass.simpleName}",
                    strongBoxSupported = strongBox,
                )
            },
        )
    }

    private suspend fun rpcReachable(): Boolean {
        val ok = withContext(Dispatchers.IO) { rpcClient.ping() }
        if (!ok) {
            setMessage("无法连接 RPC 节点，请到「设置 → RPC 端点」配置并测试连接", true)
        }
        return ok
    }

    private fun setMessage(text: String?, error: Boolean = false) {
        _message.value = text
        _messageIsError.value = text != null && error
    }

    private fun levelLabel(level: String): String = when (level.uppercase()) {
        "STRONGBOX" -> "StrongBox 硬件安全芯片"
        "TEE" -> "TEE 可信执行环境"
        "SOFTWARE" -> "软件密钥"
        else -> "未知等级"
    }

    private fun statusLabel(status: NodeStatus?): String = when (status) {
        NodeStatus.ACTIVE -> "在线"
        NodeStatus.PENDING -> "认证中"
        NodeStatus.GRACE -> "宽限期"
        NodeStatus.JAILED -> "已关押"
        NodeStatus.SLASHED -> "已罚没"
        NodeStatus.UNREGISTERED, null -> "未注册"
    }

    private fun shortAddress(address: String): String =
        if (address.length <= 16) address else address.take(8) + "…" + address.takeLast(6)
}
