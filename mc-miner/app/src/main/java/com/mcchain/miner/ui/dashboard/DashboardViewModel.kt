package com.mcchain.miner.ui.dashboard

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.data.db.BlockDao
import com.mcchain.miner.data.db.ContributionDao
import com.mcchain.miner.data.db.NodeStatusDao
import com.mcchain.miner.data.db.PhoneNodeDao
import com.mcchain.miner.data.db.AccountDao
import com.mcchain.miner.data.model.BlockInfo
import com.mcchain.miner.data.model.NodeStatusSnapshot
import com.mcchain.miner.data.model.PhoneNodeRecord
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.data.pref.SecurePrefs
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class DashboardViewModel @Inject constructor(
    private val nodeStatusDao: NodeStatusDao,
    private val phoneNodeDao: PhoneNodeDao,
    private val accountDao: AccountDao,
    private val blockDao: BlockDao,
    private val contributionDao: ContributionDao,
    private val securePrefs: SecurePrefs
) : ViewModel() {

    private val _nodeStatus = MutableStateFlow<NodeStatusSnapshot?>(null)
    val nodeStatus: StateFlow<NodeStatusSnapshot?> = _nodeStatus

    private val _phoneNode = MutableStateFlow<PhoneNodeRecord?>(null)
    val phoneNode: StateFlow<PhoneNodeRecord?> = _phoneNode

    private val _account = MutableStateFlow<WalletAccount?>(null)
    val account: StateFlow<WalletAccount?> = _account

    private val _recentBlocks = MutableStateFlow<List<BlockInfo>>(emptyList())
    val recentBlocks: StateFlow<List<BlockInfo>> = _recentBlocks

    private val _totalReward = MutableStateFlow(0L)
    val totalReward: StateFlow<Long> = _totalReward

    init {
        loadData()
    }

    fun loadData() {
        viewModelScope.launch {
            val deviceId = securePrefs.deviceId ?: ""
            _nodeStatus.value = nodeStatusDao.get()
            _phoneNode.value = phoneNodeDao.getByDeviceId(deviceId)
            _account.value = accountDao.getDefaultAccount()
            _recentBlocks.value = blockDao.getRecentBlocks(10)
            _totalReward.value = phoneNodeDao.getByDeviceId(deviceId)?.let {
                contributionDao.getTotalReward(deviceId)
            } ?: 0
        }
    }
}
