package com.mcchain.miner.ui.mining

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.data.db.ContributionDao
import com.mcchain.miner.data.db.EdgeAiTaskDao
import com.mcchain.miner.data.db.PhoneNodeDao
import com.mcchain.miner.data.model.Contribution
import com.mcchain.miner.data.model.EdgeAiTask
import com.mcchain.miner.data.pref.SecurePrefs
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class MiningViewModel @Inject constructor(
    private val contributionDao: ContributionDao,
    private val edgeAiTaskDao: EdgeAiTaskDao,
    private val phoneNodeDao: PhoneNodeDao,
    private val securePrefs: SecurePrefs
) : ViewModel() {

    private val _contributions = MutableStateFlow<List<Contribution>>(emptyList())
    val contributions: StateFlow<List<Contribution>> = _contributions

    private val _availableTasks = MutableStateFlow<List<EdgeAiTask>>(emptyList())
    val availableTasks: StateFlow<List<EdgeAiTask>> = _availableTasks

    private val _totalReward = MutableStateFlow(0L)
    val totalReward: StateFlow<Long> = _totalReward

    private val _contributionCount = MutableStateFlow(0)
    val contributionCount: StateFlow<Int> = _contributionCount

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            val deviceId = securePrefs.deviceId ?: ""
            _contributions.value = contributionDao.getByDevice(deviceId, 50)
            _availableTasks.value = edgeAiTaskDao.getAvailableTasks(20)
            _totalReward.value = contributionDao.getTotalReward(deviceId)
            _contributionCount.value = contributionDao.getCountByDevice(deviceId)
        }
    }
}
