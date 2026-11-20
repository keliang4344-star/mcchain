package com.mcchain.miner.ui.ecosystem

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.domain.wallet.TransactionService
import com.mcchain.miner.network.DexPool
import com.mcchain.miner.network.DexPrice
import com.mcchain.miner.network.DexSwapQuote
import com.mcchain.miner.network.EdgeAiDispute
import com.mcchain.miner.network.EdgeAiResult
import com.mcchain.miner.network.EdgeAiTaskDetail
import com.mcchain.miner.network.IndexerClient
import com.mcchain.miner.network.ReferralRecord
import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.network.TokenomicsSnapshot
import com.mcchain.miner.shared.util.McAmount
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import javax.inject.Inject

/** 推荐 / EdgeAI 页面共用的操作反馈。 */
data class EcosystemMessage(val text: String, val error: Boolean)

/**
 * 阶段 2 生态能力：推荐返佣、EdgeAI 任务、代币经济。
 *
 * 所有金额单位均为 umc；只展示链上查询结果与交易回执，不构造任何估算收益。
 */
@HiltViewModel
class EcosystemViewModel @Inject constructor(
    private val walletRepository: WalletRepository,
    private val transactionService: TransactionService,
    private val rpcClient: RpcClient,
    private val indexerClient: IndexerClient,
) : ViewModel() {

    private val _accountAddress = MutableStateFlow<String?>(null)
    val accountAddress: StateFlow<String?> = _accountAddress

    // === 推荐返佣 ===
    private val _referralLoading = MutableStateFlow(false)
    val referralLoading: StateFlow<Boolean> = _referralLoading

    private val _referrals = MutableStateFlow<List<ReferralRecord>>(emptyList())
    val referrals: StateFlow<List<ReferralRecord>> = _referrals

    private val _referralPendingUmc = MutableStateFlow<Long?>(null)
    val referralPendingUmc: StateFlow<Long?> = _referralPendingUmc

    private val _referralError = MutableStateFlow<String?>(null)
    val referralError: StateFlow<String?> = _referralError

    private val _referralMessage = MutableStateFlow<EcosystemMessage?>(null)
    val referralMessage: StateFlow<EcosystemMessage?> = _referralMessage

    private val _referralBusy = MutableStateFlow(false)
    val referralBusy: StateFlow<Boolean> = _referralBusy

    // === EdgeAI ===
    private val _edgeAiLoading = MutableStateFlow(false)
    val edgeAiLoading: StateFlow<Boolean> = _edgeAiLoading

    private val _edgeAiTasks = MutableStateFlow<List<EdgeAiTaskDetail>>(emptyList())
    val edgeAiTasks: StateFlow<List<EdgeAiTaskDetail>> = _edgeAiTasks

    private val _edgeAiResults = MutableStateFlow<List<EdgeAiResult>>(emptyList())
    val edgeAiResults: StateFlow<List<EdgeAiResult>> = _edgeAiResults

    private val _edgeAiDisputes = MutableStateFlow<List<EdgeAiDispute>>(emptyList())
    val edgeAiDisputes: StateFlow<List<EdgeAiDispute>> = _edgeAiDisputes

    private val _edgeAiError = MutableStateFlow<String?>(null)
    val edgeAiError: StateFlow<String?> = _edgeAiError

    private val _edgeAiMessage = MutableStateFlow<EcosystemMessage?>(null)
    val edgeAiMessage: StateFlow<EcosystemMessage?> = _edgeAiMessage

    private val _edgeAiBusy = MutableStateFlow(false)
    val edgeAiBusy: StateFlow<Boolean> = _edgeAiBusy

    // === 代币经济 ===
    private val _tokenomics = MutableStateFlow<TokenomicsSnapshot?>(null)
    val tokenomics: StateFlow<TokenomicsSnapshot?> = _tokenomics

    private val _tokenomicsLoading = MutableStateFlow(false)
    val tokenomicsLoading: StateFlow<Boolean> = _tokenomicsLoading

    private val _tokenomicsError = MutableStateFlow<String?>(null)
    val tokenomicsError: StateFlow<String?> = _tokenomicsError

    fun loadReferral() {
        viewModelScope.launch {
            _referralLoading.value = true
            _referralError.value = null
            val account = walletRepository.getDefaultAccount()
            _accountAddress.value = account?.address
            if (account == null) {
                _referrals.value = emptyList()
                _referralPendingUmc.value = null
                _referralError.value = "请先在「钱包」创建或导入账户"
                _referralLoading.value = false
                return@launch
            }
            val snapshot = withContext(Dispatchers.IO) {
                rpcClient.getReferralsByInviter(account.address) to
                    rpcClient.getReferralPendingRewards(account.address)
            }
            val list = snapshot.first
            val pending = snapshot.second
            if (list == null || pending == null) {
                _referralError.value = "无法读取推荐数据：节点未开启 referral 查询或 RPC 不可达"
            } else {
                _referrals.value = list.sortedByDescending { it.referralId }
                _referralPendingUmc.value = pending
            }
            _referralLoading.value = false
        }
    }

    /** 邀请绑定：当前钱包是邀请人，被邀请人必须是已注册节点。 */
    fun createReferral(invitee: String, inviteCode: String) {
        if (_referralBusy.value) return
        viewModelScope.launch {
            _referralBusy.value = true
            _referralMessage.value = null
            val result = withContext(Dispatchers.IO) {
                transactionService.createReferral(invitee, inviteCode)
            }
            _referralMessage.value = EcosystemMessage(result.message, !result.success)
            _referralBusy.value = false
            if (result.success) loadReferral()
        }
    }

    fun claimReferralRewards() {
        if (_referralBusy.value) return
        viewModelScope.launch {
            _referralBusy.value = true
            _referralMessage.value = null
            val result = withContext(Dispatchers.IO) { transactionService.claimReferralRewards() }
            _referralMessage.value = EcosystemMessage(result.message, !result.success)
            _referralBusy.value = false
            if (result.success) loadReferral()
        }
    }

    fun clearReferralMessage() {
        _referralMessage.value = null
    }

    // === EdgeAI ===

    fun loadEdgeAi() {
        viewModelScope.launch {
            _edgeAiLoading.value = true
            _edgeAiError.value = null
            val account = walletRepository.getDefaultAccount()
            _accountAddress.value = account?.address
            val snapshot = withContext(Dispatchers.IO) {
                Triple(
                    rpcClient.getEdgeAiTaskDetails(limit = EDGEAI_TASK_LIMIT),
                    rpcClient.getEdgeAiResults(),
                    rpcClient.getEdgeAiDisputes(),
                )
            }
            val tasks = snapshot.first
            if (tasks == null) {
                _edgeAiError.value = "无法读取 EdgeAI 任务：节点未开启 edgeai 查询或 RPC 不可达"
            } else {
                _edgeAiTasks.value = tasks.sortedByDescending { it.taskId.toLongOrNull() ?: 0L }
            }
            _edgeAiResults.value = snapshot.second.orEmpty()
            _edgeAiDisputes.value = snapshot.third.orEmpty()
            _edgeAiLoading.value = false
        }
    }

    fun createTask(description: String, rewardUmc: Long) {
        if (_edgeAiBusy.value) return
        viewModelScope.launch {
            _edgeAiBusy.value = true
            _edgeAiMessage.value = null
            val result = withContext(Dispatchers.IO) {
                transactionService.createEdgeAiTask(description, rewardUmc)
            }
            _edgeAiMessage.value = EcosystemMessage(result.message, !result.success)
            _edgeAiBusy.value = false
            if (result.success) loadEdgeAi()
        }
    }

    fun openDispute(taskId: String, reason: String) {
        if (_edgeAiBusy.value) return
        viewModelScope.launch {
            _edgeAiBusy.value = true
            _edgeAiMessage.value = null
            val result = withContext(Dispatchers.IO) {
                transactionService.openEdgeAiDispute(taskId, reason)
            }
            _edgeAiMessage.value = EcosystemMessage(result.message, !result.success)
            _edgeAiBusy.value = false
            if (result.success) loadEdgeAi()
        }
    }

    fun clearEdgeAiMessage() {
        _edgeAiMessage.value = null
    }

    // === 代币经济 ===

    fun loadTokenomics() {
        if (_tokenomicsLoading.value) return
        viewModelScope.launch {
            _tokenomicsLoading.value = true
            _tokenomicsError.value = null
            val snapshot = withContext(Dispatchers.IO) { rpcClient.getTokenomics() }
            if (snapshot == null) {
                _tokenomicsError.value = "无法读取链上代币数据，请检查节点连接"
            } else {
                _tokenomics.value = snapshot
            }
            _tokenomicsLoading.value = false
        }
    }

    // === 贡献历史（索引器） ===
    private val _contributions = MutableStateFlow<List<IndexerClient.IndexerEvent>>(emptyList())
    val contributions: StateFlow<List<IndexerClient.IndexerEvent>> = _contributions

    private val _contributionTotal = MutableStateFlow(0)
    val contributionTotal: StateFlow<Int> = _contributionTotal

    private val _contributionLoading = MutableStateFlow(false)
    val contributionLoading: StateFlow<Boolean> = _contributionLoading

    private val _contributionError = MutableStateFlow<String?>(null)
    val contributionError: StateFlow<String?> = _contributionError

    private val _contributionNotConfigured = MutableStateFlow(false)
    val contributionNotConfigured: StateFlow<Boolean> = _contributionNotConfigured

    /**
     * 从本地索引器按地址分页读取贡献事件。
     * 索引器未配置/不可达时保留空列表并给出明确状态，不回落成本地推测数据。
     */
    fun loadContributions(loadMore: Boolean = false) {
        if (_contributionLoading.value) return
        viewModelScope.launch {
            _contributionLoading.value = true
            if (!loadMore) {
                _contributionError.value = null
                _contributionNotConfigured.value = false
            }
            val account = walletRepository.getDefaultAccount()
            _accountAddress.value = account?.address
            if (account == null) {
                _contributionError.value = "请先在「钱包」创建或导入账户"
                _contributionLoading.value = false
                return@launch
            }
            val offset = if (loadMore) _contributions.value.size else 0
            when (val result = indexerClient.getContributions(account.address, CONTRIBUTION_PAGE_SIZE, offset)) {
                is IndexerClient.ContributionsResult.NotConfigured -> {
                    _contributionNotConfigured.value = true
                    if (!loadMore) _contributions.value = emptyList()
                }
                is IndexerClient.ContributionsResult.Failure -> {
                    _contributionError.value = result.message
                }
                is IndexerClient.ContributionsResult.Success -> {
                    _contributionNotConfigured.value = false
                    _contributions.value = if (loadMore) {
                        _contributions.value + result.events
                    } else {
                        result.events
                    }
                    _contributionTotal.value = result.total
                }
            }
            _contributionLoading.value = false
        }
    }

    // === DEX 只读行情 ===
    private val _dexPools = MutableStateFlow<List<DexPool>>(emptyList())
    val dexPools: StateFlow<List<DexPool>> = _dexPools

    private val _dexPrices = MutableStateFlow<Map<Long, DexPrice>>(emptyMap())
    val dexPrices: StateFlow<Map<Long, DexPrice>> = _dexPrices

    private val _dexLoading = MutableStateFlow(false)
    val dexLoading: StateFlow<Boolean> = _dexLoading

    private val _dexError = MutableStateFlow<String?>(null)
    val dexError: StateFlow<String?> = _dexError

    private val _dexQuote = MutableStateFlow<DexQuoteState?>(null)
    val dexQuote: StateFlow<DexQuoteState?> = _dexQuote

    private val _dexQuoteBusy = MutableStateFlow(false)
    val dexQuoteBusy: StateFlow<Boolean> = _dexQuoteBusy

    private val _dexQuoteError = MutableStateFlow<String?>(null)
    val dexQuoteError: StateFlow<String?> = _dexQuoteError

    /** 只读加载池列表与各池价格；不提供任何兑换交易入口。 */
    fun loadDex() {
        if (_dexLoading.value) return
        viewModelScope.launch {
            _dexLoading.value = true
            _dexError.value = null
            val pools = withContext(Dispatchers.IO) { rpcClient.getDexPools() }
            if (pools == null) {
                _dexError.value = "无法读取 DEX 池：节点不可达或节点版本不支持该查询"
                _dexLoading.value = false
                return@launch
            }
            _dexPools.value = pools.sortedBy { it.id }
            val prices = withContext(Dispatchers.IO) {
                pools.take(DEX_POOL_LIMIT).mapNotNull { pool ->
                    rpcClient.getDexPrice(pool.id, pool.denomA)?.let { pool.id to it }
                }.toMap()
            }
            _dexPrices.value = prices
            _dexLoading.value = false
        }
    }

    /** 试算指定池的 A→B 兑换，仅展示链上计算结果，不签名、不广播。 */
    fun estimateDexSwap(pool: DexPool, amountMc: String) {
        if (_dexQuoteBusy.value) return
        val amountUmc = McAmount.parseToUmc(amountMc)
        if (amountUmc == null || amountUmc <= 0L) {
            _dexQuoteError.value = "请输入大于 0 的金额（最多 6 位小数）"
            return
        }
        viewModelScope.launch {
            _dexQuoteBusy.value = true
            _dexQuoteError.value = null
            _dexQuote.value = null
            val quote = withContext(Dispatchers.IO) {
                rpcClient.estimateDexSwap(pool.id, pool.denomA, amountUmc, pool.denomB)
            }
            if (quote == null) {
                _dexQuoteError.value = "试算失败：请检查金额、币种与池状态"
            } else {
                _dexQuote.value = DexQuoteState(pool.id, quote)
            }
            _dexQuoteBusy.value = false
        }
    }

    fun clearDexQuote() {
        _dexQuote.value = null
        _dexQuoteError.value = null
    }

    private companion object {
        const val EDGEAI_TASK_LIMIT = 50
        const val CONTRIBUTION_PAGE_SIZE = 20
        const val DEX_POOL_LIMIT = 20
    }
}

/** 某个池的只读试算结果。 */
data class DexQuoteState(val poolId: Long, val quote: DexSwapQuote)
