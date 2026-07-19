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
import com.mcchain.miner.domain.node.HardwareCheck
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.domain.node.DeviceAttestation
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class NodeViewModel @Inject constructor(
    private val nodeStatusDao: NodeStatusDao,
    private val phoneNodeDao: PhoneNodeDao,
    private val peerDao: PeerDao,
    private val blockDao: BlockDao,
    private val deviceAttestation: DeviceAttestation,
    private val securePrefs: SecurePrefs
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

    fun performAttestation() {
        viewModelScope.launch {
            val deviceId = securePrefs.deviceId ?: ""
            _isAttesting.value = true
            try {
                val result = deviceAttestation.performAttestation()
                if (result.success) {
                    phoneNodeDao.upsert(
                        PhoneNodeRecord(
                            deviceId = deviceId,
                            nodeAddress = "",
                            attestationToken = result.token,
                            attestationTime = System.currentTimeMillis(),
                            attestationExpiry = System.currentTimeMillis() + MCParams.PHONENODE_ATTESTATION_VALIDITY_SECONDS * 1000,
                            lastHeartbeat = System.currentTimeMillis(),
                            status = NodeStatus.ACTIVE,
                            stakedAmount = MCParams.PHONENODE_MIN_STAKE_UMC
                        )
                    )
                }
            } finally {
                _isAttesting.value = false
            }
            load()
        }
    }
}
