package com.mcchain.miner.ui.node

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
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.domain.node.DeviceAttestation
import com.mcchain.miner.domain.node.HardwareCheck
import com.mcchain.miner.domain.wallet.TransactionService
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import javax.inject.Inject

@HiltViewModel
class NodeViewModel @Inject constructor(
    private val nodeStatusDao: NodeStatusDao,
    private val phoneNodeDao: PhoneNodeDao,
    private val peerDao: PeerDao,
    private val blockDao: BlockDao,
    private val deviceAttestation: DeviceAttestation,
    private val securePrefs: SecurePrefs,
    private val walletRepository: WalletRepository,
    private val transactionService: TransactionService,
) : ViewModel() {

    private val _nodeStatus = MutableStateFlow<NodeStatusSnapshot?>(null)
    val nodeStatus: StateFlow<NodeStatusSnapshot?> = _nodeStatus

    private val _phoneNode = MutableStateFlow<PhoneNodeRecord?>(null)
    val phoneNode: StateFlow<PhoneNodeRecord?> = _phoneNode

    private val _peers = MutableStateFlow<List<PeerNode>>(emptyList())
    val peers: StateFlow<List<PeerNode>> = _peers

    private val _hardwareCheck = MutableStateFlow<HardwareCheck?>(null)
    val hardwareCheck: StateFlow<HardwareCheck?> = _hardwareCheck

    private val _isAttesting = MutableStateFlow(false)
    val isAttesting: StateFlow<Boolean> = _isAttesting

    private val _attestationMessage = MutableStateFlow<String?>(null)
    val attestationMessage: StateFlow<String?> = _attestationMessage

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            val deviceId = securePrefs.deviceId ?: ""
            _nodeStatus.value = nodeStatusDao.get()
            _phoneNode.value = phoneNodeDao.getByDeviceId(deviceId)
            _peers.value = peerDao.getAllPeers()
            _hardwareCheck.value = deviceAttestation.checkHardwareRequirements()
        }
    }

    /**
     * 硬件认证 + 上链：
     * AndroidKeyStore 出证 → 预言机 /sign 背书 → depin.MsgAttestDevice → 回执。
     * 任何一步失败都只展示原因，不写「已认证」状态。
     */
    fun performAttestation() {
        viewModelScope.launch {
            _isAttesting.value = true
            try {
                val address = withContext(Dispatchers.IO) {
                    walletRepository.getDefaultAccount()?.address
                }
                if (address == null) {
                    _attestationMessage.value = "请先创建或导入钱包，再认证设备"
                    return@launch
                }

                // 认证上链的前置：depin 设备注册 + phonenode 节点注册（发币闸口要求 HasNode）。
                withContext(Dispatchers.IO) {
                    transactionService.ensureDeviceRegistered(
                        model = android.os.Build.MODEL,
                        os = "Android ${android.os.Build.VERSION.RELEASE}",
                    )
                    transactionService.ensureNodeRegistered(
                        model = android.os.Build.MODEL,
                        os = "Android ${android.os.Build.VERSION.RELEASE}",
                    )
                }

                val result = withContext(Dispatchers.IO) {
                    deviceAttestation.performAttestation(address)
                }
                if (!result.success) {
                    _attestationMessage.value = result.error ?: "认证失败"
                    return@launch
                }

                _attestationMessage.value =
                    "硬件证明已通过（${result.securityLevel}），正在提交上链…"

                // 先 phonenode 自签认证（Layer2 闸门），再 depin 预言机认证（刷新有效期/背书层级）。
                val nodeAttest = withContext(Dispatchers.IO) {
                    transactionService.submitPhonenodeAttestation(
                        rootHash = result.rootHash,
                        nonce = result.selfNonce,
                        deviceIdHash = result.deviceIdHash,
                        devicePubKeyHex = result.devicePubKeyHex,
                        signatureHex = result.selfSignatureHex,
                    )
                }
                if (!nodeAttest.success) {
                    _attestationMessage.value = "节点认证上链失败：${nodeAttest.message}"
                    return@launch
                }

                val submit = withContext(Dispatchers.IO) {
                    transactionService.attestDevice(result.challenge, result.signatureBase64)
                }
                if (submit.success) {
                    phoneNodeDao.upsert(
                        PhoneNodeRecord(
                            deviceId = result.deviceId,
                            nodeAddress = address,
                            attestationToken = result.token,
                            attestationTime = System.currentTimeMillis(),
                            attestationExpiry = System.currentTimeMillis() +
                                MCParams.PHONENODE_ATTESTATION_VALIDITY_SECONDS * 1000,
                            lastHeartbeat = System.currentTimeMillis(),
                            status = if (submit.pending) NodeStatus.PENDING else NodeStatus.ACTIVE,
                            stakedAmount = MCParams.PHONENODE_MIN_STAKE_UMC,
                        )
                    )
                    _attestationMessage.value = if (submit.pending) {
                        "认证交易已广播，等待链上确认"
                    } else {
                        "设备已通过 ${result.securityLevel} 认证并在链上登记"
                    }
                } else {
                    _attestationMessage.value = "硬件认证已通过，但上链失败：${submit.message}"
                }
            } catch (e: Exception) {
                _attestationMessage.value = "认证失败：${e.message ?: e.javaClass.simpleName}"
            } finally {
                _isAttesting.value = false
            }
            load()
        }
    }
}
