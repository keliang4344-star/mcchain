package com.mcchain.miner.ui.wallet

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.AddressBookEntry
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.domain.wallet.TransactionService
import com.mcchain.miner.domain.wallet.TxBuilder
import com.mcchain.miner.domain.wallet.WalletManager
import com.mcchain.miner.network.ChainTxRecord
import com.mcchain.miner.network.Delegation
import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.network.TxResult
import com.mcchain.miner.network.ValidatorInfo
import com.mcchain.miner.shared.util.AddressRules
import com.mcchain.miner.shared.util.Hex
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.math.BigDecimal
import java.text.DecimalFormat
import javax.inject.Inject

/** 一次链上操作的结果，用于统一展示成功/排队/失败与交易哈希。 */
data class TxOutcome(
    val title: String,
    val success: Boolean,
    val pending: Boolean = false,
    val txHash: String = "",
    val message: String = "",
)

/** 交易详情：本地记录 + 链上回执（失败原因来自回执日志）。 */
data class TxDetailState(
    val record: TxRecord,
    val loading: Boolean,
    val receipt: TxResult? = null,
)

@HiltViewModel
class WalletViewModel @Inject constructor(
    private val walletRepository: WalletRepository,
    private val walletManager: WalletManager,
    private val transactionService: TransactionService,
    private val rpcClient: RpcClient,
) : ViewModel() {

    private val _accounts = MutableStateFlow<List<WalletAccount>>(emptyList())
    val accounts: StateFlow<List<WalletAccount>> = _accounts

    /** 本地地址簿（仅本机，不上传）。 */
    private val _addressBook = MutableStateFlow<List<AddressBookEntry>>(emptyList())
    val addressBook: StateFlow<List<AddressBookEntry>> = _addressBook

    private val _addressBookMessage = MutableStateFlow<String?>(null)
    val addressBookMessage: StateFlow<String?> = _addressBookMessage

    private val _transactions = MutableStateFlow<List<TxRecord>>(emptyList())
    val transactions: StateFlow<List<TxRecord>> = _transactions

    /** 链上交易历史（LCD 查询，权威来源；与本地记录并列展示）。 */
    private val _chainTxs = MutableStateFlow<List<ChainTxRecord>>(emptyList())
    val chainTxs: StateFlow<List<ChainTxRecord>> = _chainTxs

    /** 链上交易历史查询失败原因；null 表示成功或尚未查询。 */
    private val _chainTxError = MutableStateFlow<String?>(null)
    val chainTxError: StateFlow<String?> = _chainTxError

    /** 当前默认地址在链上的委托列表（getDelegations 实时查询）。 */
    private val _delegations = MutableStateFlow<List<Delegation>>(emptyList())
    val delegations: StateFlow<List<Delegation>> = _delegations

    /** 链上已绑定验证人列表，用于委托选择。 */
    private val _validators = MutableStateFlow<List<ValidatorInfo>>(emptyList())
    val validators: StateFlow<List<ValidatorInfo>> = _validators

    private val _validatorsLoading = MutableStateFlow(false)
    val validatorsLoading: StateFlow<Boolean> = _validatorsLoading

    private val _validatorError = MutableStateFlow<String?>(null)
    val validatorError: StateFlow<String?> = _validatorError

    /** 创建钱包时待用户备份的助记词；确认前不落盘。 */
    private val _backupMnemonic = MutableStateFlow<List<String>?>(null)
    val backupMnemonic: StateFlow<List<String>?> = _backupMnemonic

    /** 二次确认后从安全存储读出的助记词；关闭即清空内存引用。 */
    private val _revealedMnemonic = MutableStateFlow<List<String>?>(null)
    val revealedMnemonic: StateFlow<List<String>?> = _revealedMnemonic

    private val _busy = MutableStateFlow(false)
    val busy: StateFlow<Boolean> = _busy

    private val _refreshing = MutableStateFlow(false)
    val refreshing: StateFlow<Boolean> = _refreshing

    /** 链上刷新失败原因（网络不可达/异常），UI 需展示并提供重试。 */
    private val _refreshError = MutableStateFlow<String?>(null)
    val refreshError: StateFlow<String?> = _refreshError

    /** 非失败的链上提示（例如地址尚未被链上收录）。 */
    private val _chainHint = MutableStateFlow<String?>(null)
    val chainHint: StateFlow<String?> = _chainHint

    private val _lastUpdated = MutableStateFlow<Long?>(null)
    val lastUpdated: StateFlow<Long?> = _lastUpdated

    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message

    private val _outcome = MutableStateFlow<TxOutcome?>(null)
    val outcome: StateFlow<TxOutcome?> = _outcome

    private val _txDetail = MutableStateFlow<TxDetailState?>(null)
    val txDetail: StateFlow<TxDetailState?> = _txDetail

    init {
        refresh()
    }

    fun refresh() {
        if (!_refreshing.compareAndSet(false, true)) return
        viewModelScope.launch {
            _refreshError.value = null
            try {
                val accounts = walletRepository.getAllAccounts()
                // 签名私钥由 SecurePrefs.activeAccountIndex 决定，必须与 Room 默认账户一致。
                // 切换中断、清数据或恢复备份后若两者不一致，以签名索引为准校正，避免
                // “界面显示 A 账户、实际用 B 账户私钥签名”的资金风险。
                val activeAddress = withContext(Dispatchers.IO) { walletManager.getStoredWallet()?.address }
                val currentDefault = accounts.firstOrNull { it.isDefault }
                if (activeAddress != null && currentDefault?.address != activeAddress) {
                    if (accounts.any { it.address == activeAddress }) {
                        walletRepository.clearDefaultFlags()
                        walletRepository.setDefault(activeAddress)
                    } else {
                        walletManager.activeAccountIndex = 0
                    }
                }
                _accounts.value = walletRepository.getAllAccounts()
                _addressBook.value = runCatching { walletRepository.getAddressBook() }
                    .getOrDefault(emptyList())
                val default = _accounts.value.firstOrNull { it.isDefault }
                if (default == null) {
                    _transactions.value = emptyList()
                    _chainTxs.value = emptyList()
                    _chainTxError.value = null
                    _delegations.value = emptyList()
                    _chainHint.value = null
                    return@launch
                }

                val snapshot = withContext(Dispatchers.IO) {
                    // 先重放离线队列，再刷新余额：队列里的交易可能已改变余额与 sequence。
                    transactionService.replayPending()
                    val accountFound = transactionService.refreshAccount(default.address)
                    val online = rpcClient.ping()
                    val delegations = if (online) rpcClient.getDelegations(default.address) else emptyList()
                    val chainTxs = if (online) rpcClient.getTxHistoryOrNull(default.address, CHAIN_TX_LIMIT) else null
                    ensureDeviceRegistered()
                    RefreshSnapshot(accountFound, online, delegations, chainTxs)
                }

                _accounts.value = walletRepository.getAllAccounts()
                _transactions.value = walletRepository
                    .getTransactionsForAddress(default.address, 50)
                    .sortedByDescending { it.timestamp }
                _delegations.value = snapshot.delegations
                _chainTxs.value = snapshot.chainTxs.orEmpty()
                _chainTxError.value = when {
                    !snapshot.online -> "无法连接节点，链上交易历史暂不可用"
                    snapshot.chainTxs == null -> "节点未返回交易历史：REST 交易索引可能未开启，或该接口暂不支持按发送方过滤"
                    else -> null
                }

                if (!snapshot.online) {
                    _refreshError.value = "无法连接链上节点：请检查网络连接或设置中的 RPC 节点地址后重试"
                    _chainHint.value = null
                } else {
                    _lastUpdated.value = System.currentTimeMillis()
                    _chainHint.value = if (snapshot.accountFound) {
                        null
                    } else {
                        "未能获取链上账户信息：可能该地址尚无交易，或节点 REST 接口暂不可用；余额显示为 0"
                    }
                }
            } catch (e: Exception) {
                _refreshError.value = "刷新失败：${e.message ?: e.javaClass.simpleName}"
            } finally {
                _refreshing.value = false
            }
        }
    }

    private suspend fun ensureDeviceRegistered() {
        val result = runCatching {
            transactionService.ensureDeviceRegistered(
                model = android.os.Build.MODEL,
                os = "Android ${android.os.Build.VERSION.RELEASE}",
            )
        }.getOrNull() ?: return
        if (!result.success) _message.value = result.message
    }

    fun createWallet() {
        if (!walletManager.isSecureStorageAvailable) {
            _message.value = "安全存储不可用（Keystore 初始化失败），无法创建钱包"
            return
        }
        val mnemonic = walletManager.generateMnemonic()
        if (mnemonic.isEmpty()) {
            _message.value = "助记词生成失败"
            return
        }
        _backupMnemonic.value = mnemonic
    }

    fun confirmCreateWallet() {
        val mnemonic = _backupMnemonic.value ?: return
        persistWallet(mnemonic)
        _backupMnemonic.value = null
    }

    fun discardBackup() {
        _backupMnemonic.value = null
    }

    fun revealMnemonic() {
        if (!walletManager.isSecureStorageAvailable) {
            _message.value = "安全存储不可用（Keystore 初始化失败），无法读取助记词"
            return
        }
        val words = walletManager.getStoredMnemonic()
        if (words.isNullOrEmpty()) {
            _message.value = "本机未保存助记词，请通过助记词导入钱包"
            return
        }
        _revealedMnemonic.value = words
    }

    fun hideMnemonic() {
        _revealedMnemonic.value = null
    }

    fun importWallet(words: String) {
        val mnemonic = words.trim().split(Regex("\\s+")).filter { it.isNotBlank() }
        if (!walletManager.isValidMnemonic(mnemonic)) {
            _message.value = "助记词无效：请检查单词、顺序与校验和"
            return
        }
        persistWallet(mnemonic)
    }

    /** 加载链上验证人列表；失败时保留旧列表并给出可读原因。 */
    fun loadValidators(force: Boolean = false) {
        if (_validatorsLoading.value) return
        if (!force && _validators.value.isNotEmpty()) return
        _validatorsLoading.value = true
        _validatorError.value = null
        viewModelScope.launch {
            try {
                val list = withContext(Dispatchers.IO) { rpcClient.getValidators() }
                    .filter { !it.jailed && it.operatorAddress.isNotBlank() }
                    .sortedByDescending { it.tokensUmc }
                _validators.value = list
                if (list.isEmpty()) {
                    _validatorError.value = "链上暂未查询到可用验证人，请检查节点地址，或手动填写验证人地址"
                }
            } catch (e: Exception) {
                _validatorError.value = "验证人列表加载失败：${e.message ?: e.javaClass.simpleName}"
            } finally {
                _validatorsLoading.value = false
            }
        }
    }

    /** 转账表单校验；返回 null 表示可以提交，否则为中文错误原因。 */
    fun validateTransfer(toAddress: String, amountMc: String): String? {
        val address = toAddress.trim()
        val account = _accounts.value.firstOrNull { it.isDefault }
            ?: return "钱包未初始化，请先创建或导入助记词"
        validateUserAddress(address)?.let { return it }
        if (address == account.address) return "不能向自己的地址转账"
        val amount = parseAmountMc(amountMc)
            ?: return "金额格式无效：请输入大于 0、最多 6 位小数的 MC 数量"
        if (amount <= 0) return "转账金额必须大于 0"
        return checkBalance(account, amount)
    }

    /** 委托表单校验；返回 null 表示可以提交，否则为中文错误原因。 */
    fun validateDelegation(validatorAddress: String, amountMc: String): String? {
        val validator = validatorAddress.trim()
        val account = _accounts.value.firstOrNull { it.isDefault }
            ?: return "钱包未初始化，请先创建或导入助记词"
        validatorFormatError(validator)?.let { return it }
        val amount = parseAmountMc(amountMc)
            ?: return "金额格式无效：请输入大于 0、最多 6 位小数的 MC 数量"
        if (amount <= 0) return "委托金额必须大于 0"
        return checkBalance(account, amount)
    }

    /** 解除委托表单校验；金额不得超过该验证人的链上委托数量。 */
    fun validateUndelegate(validatorAddress: String, amountMc: String): String? {
        val account = _accounts.value.firstOrNull { it.isDefault }
            ?: return "钱包未初始化，请先创建或导入助记词"
        val validator = validatorAddress.trim()
        validatorFormatError(validator)?.let { return it }
        val amount = parseAmountMc(amountMc)
            ?: return "金额格式无效：请输入大于 0、最多 6 位小数的 MC 数量"
        if (amount <= 0) return "解除金额必须大于 0"
        val delegation = _delegations.value.firstOrNull { it.validatorAddress == validator }
            ?: return "当前没有向该验证人的委托，请刷新后重试"
        if (amount > delegation.amount) {
            return "解除金额超过该验证人的委托数量（${formatMc(delegation.amount)}）"
        }
        return checkFeeBalance(account)
    }

    /** 转委托表单校验；源与目标不能相同，金额不得超过源委托数量。 */
    fun validateRedelegate(source: String, destination: String, amountMc: String): String? {
        val account = _accounts.value.firstOrNull { it.isDefault }
            ?: return "钱包未初始化，请先创建或导入助记词"
        val src = source.trim()
        val dst = destination.trim()
        validatorFormatError(src)?.let { return "源验证人地址无效：$it" }
        validatorFormatError(dst)?.let { return "目标验证人地址无效：$it" }
        if (src == dst) return "转委托的源与目标验证人不能相同"
        val amount = parseAmountMc(amountMc)
            ?: return "金额格式无效：请输入大于 0、最多 6 位小数的 MC 数量"
        if (amount <= 0) return "转委托金额必须大于 0"
        val delegation = _delegations.value.firstOrNull { it.validatorAddress == src }
            ?: return "当前没有向源验证人的委托，请刷新后重试"
        if (amount > delegation.amount) {
            return "转委托金额超过源验证人的委托数量（${formatMc(delegation.amount)}）"
        }
        return checkFeeBalance(account)
    }

    fun send(toAddress: String, amountMc: String) {
        validateTransfer(toAddress, amountMc)?.let { reason ->
            _outcome.value = TxOutcome("转账失败", success = false, message = reason)
            return
        }
        val amount = parseAmountMc(amountMc) ?: return
        submit("转账") { transactionService.send(toAddress.trim(), amount) }
    }

    fun delegate(validatorAddress: String, amountMc: String) {
        validateDelegation(validatorAddress, amountMc)?.let { reason ->
            _outcome.value = TxOutcome("质押失败", success = false, message = reason)
            return
        }
        val amount = parseAmountMc(amountMc) ?: return
        submit("质押委托") { transactionService.delegate(validatorAddress.trim(), amount) }
    }

    fun undelegate(validatorAddress: String, amountMc: String) {
        validateUndelegate(validatorAddress, amountMc)?.let { reason ->
            _outcome.value = TxOutcome("解除委托失败", success = false, message = reason)
            return
        }
        val amount = parseAmountMc(amountMc) ?: return
        submit("解除委托") { transactionService.undelegate(validatorAddress.trim(), amount) }
    }

    fun redelegate(source: String, destination: String, amountMc: String) {
        validateRedelegate(source, destination, amountMc)?.let { reason ->
            _outcome.value = TxOutcome("转委托失败", success = false, message = reason)
            return
        }
        val amount = parseAmountMc(amountMc) ?: return
        submit("转委托") {
            transactionService.redelegate(source.trim(), destination.trim(), amount)
        }
    }

    fun claimRewards() {
        val account = _accounts.value.firstOrNull { it.isDefault }
        if (account == null) {
            _outcome.value = TxOutcome("领取失败", success = false, message = "钱包未初始化，请先创建或导入助记词")
            return
        }
        if (account.rewardBalance <= 0) {
            _outcome.value = TxOutcome("领取失败", success = false, message = "当前没有可领取的委托奖励")
            return
        }
        submit("领取委托奖励") { transactionService.claimAllRewards() }
    }

    private fun submit(title: String, block: suspend () -> TransactionService.SubmitResult) {
        if (!_busy.compareAndSet(false, true)) return
        viewModelScope.launch {
            try {
                val result = withContext(Dispatchers.IO) { block() }
                _outcome.value = TxOutcome(
                    title = if (result.success) title else "${title}失败",
                    success = result.success,
                    pending = result.pending,
                    txHash = result.txHash,
                    message = result.message.ifBlank {
                        if (result.success) "操作已完成" else "操作失败，请稍后重试"
                    },
                )
            } catch (e: Exception) {
                _outcome.value = TxOutcome(
                    title = "${title}失败",
                    success = false,
                    message = "提交异常：${e.message ?: e.javaClass.simpleName}",
                )
            } finally {
                _busy.value = false
            }
            refresh()
        }
    }

    fun openTxDetail(tx: TxRecord) {
        _txDetail.value = TxDetailState(record = tx, loading = true)
        viewModelScope.launch {
            val receipt = withContext(Dispatchers.IO) {
                runCatching { rpcClient.getTxResult(tx.hash) }.getOrNull()
            }
            val current = _txDetail.value
            if (current != null && current.record.hash == tx.hash) {
                _txDetail.value = current.copy(loading = false, receipt = receipt)
            }
        }
    }

    fun closeTxDetail() {
        _txDetail.value = null
    }

    fun clearOutcome() {
        _outcome.value = null
    }

    fun clearMessage() {
        _message.value = null
    }

    /** 切换活动账户：同步 SecurePrefs 派生索引与 Room 默认标记，并刷新该账户链上数据。 */
    fun switchAccount(address: String) {
        if (_busy.value) return
        viewModelScope.launch {
            _busy.value = true
            try {
                val account = walletRepository.getAccount(address)
                if (account == null) {
                    _message.value = "账户不存在，请刷新后重试"
                    return@launch
                }
                walletManager.activeAccountIndex = account.accountIndex
                walletRepository.clearDefaultFlags()
                walletRepository.setDefault(address)
                // 即使 refresh 因并发门禁被跳过，UI 也必须立刻显示新默认账户，
                // 不能出现“界面旧账户、签名新账户”的窗口。
                _accounts.value = walletRepository.getAllAccounts()
                // 清掉上一个账户的视图数据，避免切换瞬间看到旧余额/委托。
                _transactions.value = emptyList()
                _chainTxs.value = emptyList()
                _chainTxError.value = null
                _delegations.value = emptyList()
                _message.value = "已切换到账户 ${account.accountIndex + 1}"
            } catch (e: Exception) {
                _message.value = "切换账户失败：${e.message ?: e.javaClass.simpleName}"
            } finally {
                _busy.value = false
            }
            refresh()
        }
    }

    /** 从当前助记词派生下一个账户并切换（路径 m/44'/118'/0'/0/{index}，无需新助记词）。 */
    fun createAccount() {
        if (!walletManager.isSecureStorageAvailable) {
            _message.value = "安全存储不可用（Keystore 初始化失败），无法创建账户"
            return
        }
        if (_busy.value) return
        viewModelScope.launch {
            _busy.value = true
            try {
                val existing = walletRepository.getAllAccounts()
                val nextIndex = (existing.maxOfOrNull { it.accountIndex } ?: -1) + 1
                if (nextIndex > WalletManager.MAX_ACCOUNT_INDEX) {
                    _message.value = "最多支持 ${WalletManager.MAX_ACCOUNT_INDEX + 1} 个账户"
                    return@launch
                }
                val derived = withContext(Dispatchers.IO) {
                    walletManager.deriveWallet(index = nextIndex)
                }
                if (existing.any { it.address == derived.address }) {
                    _message.value = "该账户已存在"
                    return@launch
                }
                walletManager.activeAccountIndex = nextIndex
                walletRepository.clearDefaultFlags()
                walletRepository.insertAccount(
                    WalletAccount(
                        address = derived.address,
                        accountIndex = nextIndex,
                        publicKeyHex = Hex.encode(derived.publicKey),
                        label = "账户 ${nextIndex + 1}",
                        isDefault = true,
                    )
                )
                _accounts.value = walletRepository.getAllAccounts()
                _message.value = "已创建账户 ${nextIndex + 1}"
            } catch (e: Exception) {
                _message.value = "创建账户失败：${e.message ?: e.javaClass.simpleName}"
            } finally {
                _busy.value = false
            }
            refresh()
        }
    }

    /** 重命名账户（仅本地标签）；留空恢复「账户 N」。 */
    fun renameAccount(address: String, label: String) {
        if (_busy.value) return
        val trimmed = label.trim()
        if (trimmed.length > MAX_ADDRESS_LABEL_LENGTH) {
            _message.value = "账户名称过长（最多 $MAX_ADDRESS_LABEL_LENGTH 个字符）"
            return
        }
        viewModelScope.launch {
            try {
                val account = walletRepository.getAccount(address) ?: run {
                    _message.value = "账户不存在，请刷新后重试"
                    return@launch
                }
                walletRepository.renameAccount(address, trimmed)
                _accounts.value = walletRepository.getAllAccounts()
                _message.value = if (trimmed.isEmpty()) {
                    "已恢复默认名称：账户 ${account.accountIndex + 1}"
                } else {
                    "已重命名为 $trimmed"
                }
            } catch (e: Exception) {
                _message.value = "重命名失败：${e.message ?: e.javaClass.simpleName}"
            }
        }
    }

    /**
     * 按派生索引添加账户（0..19）。
     * 同一助记词可按索引重新派生，用于恢复本机误删或从其他设备对齐的账户。
     */
    fun addAccountByIndex(index: Int) {
        if (!walletManager.isSecureStorageAvailable) {
            _message.value = "安全存储不可用（Keystore 初始化失败），无法添加账户"
            return
        }
        if (_busy.value) return
        if (index !in 0..WalletManager.MAX_ACCOUNT_INDEX) {
            _message.value = "账户索引必须在 0..${WalletManager.MAX_ACCOUNT_INDEX} 之间"
            return
        }
        viewModelScope.launch {
            _busy.value = true
            try {
                val existing = walletRepository.getAllAccounts()
                val derived = withContext(Dispatchers.IO) {
                    walletManager.deriveWallet(index = index)
                }
                if (existing.any { it.address == derived.address }) {
                    _message.value = "索引 $index 对应的账户已在列表中"
                    return@launch
                }
                walletRepository.insertAccount(
                    WalletAccount(
                        address = derived.address,
                        accountIndex = index,
                        publicKeyHex = Hex.encode(derived.publicKey),
                        label = "账户 ${index + 1}",
                        isDefault = false,
                    )
                )
                _accounts.value = walletRepository.getAllAccounts()
                _message.value = "已添加账户 ${index + 1}（索引 $index），可在切换列表中选择"
            } catch (e: Exception) {
                _message.value = "添加账户失败：${e.message ?: e.javaClass.simpleName}"
            } finally {
                _busy.value = false
            }
        }
    }

    private fun persistWallet(mnemonic: List<String>) {
        viewModelScope.launch {
            _busy.value = true
            try {
                val derived = withContext(Dispatchers.IO) {
                    walletManager.storeMnemonic(mnemonic)
                    // 新助记词从索引 0 开始；旧账户全部清空，避免残留无法签名的地址。
                    walletManager.activeAccountIndex = 0
                    walletManager.deriveWallet(mnemonic, 0)
                }
                walletRepository.deleteAllAccounts()
                walletRepository.insertAccount(
                    WalletAccount(
                        address = derived.address,
                        accountIndex = 0,
                        publicKeyHex = Hex.encode(derived.publicKey),
                        isDefault = true,
                    )
                )
                _message.value = "钱包已就绪"
                refresh()
            } catch (e: Exception) {
                _message.value = "钱包创建失败：${e.message ?: e.javaClass.simpleName}"
            } finally {
                _busy.value = false
            }
        }
    }

    private fun checkBalance(account: WalletAccount, amountUmc: Long): String? {
        val fee = TxBuilder.DEFAULT_FEE_UMC.toLong()
        if (account.balance <= 0) {
            return "可用余额为 0，请先收款或点击刷新获取链上余额"
        }
        if (amountUmc > account.balance - fee) {
            return "余额不足：金额加网络手续费 ${formatMc(fee)} 超过可用余额 ${formatMc(account.balance)}"
        }
        return null
    }

    /** 只校验手续费是否够付（解除/转委托不占用可用余额，只扣手续费）。 */
    private fun checkFeeBalance(account: WalletAccount): String? {
        val fee = TxBuilder.DEFAULT_FEE_UMC.toLong()
        if (account.balance < fee) {
            return "可用余额不足以支付网络手续费 ${formatMc(fee)}，请先充值"
        }
        return null
    }

    private fun validatorFormatError(validator: String): String? =
        AddressRules.validatorError(validator, MCParams.BECH32_PREFIX)

    private fun validateUserAddress(address: String): String? =
        AddressRules.accountError(address, MCParams.BECH32_PREFIX)

    /** 地址簿输入校验（对话框保存前调用，避免关闭后才看到错误）。 */
    fun validateAddressBookInput(address: String, label: String): String? {
        validateUserAddress(address.trim())?.let { return it }
        if (address.trim() == "") return "请输入地址"
        if (label.trim().length > MAX_ADDRESS_LABEL_LENGTH) {
            return "备注过长（最多 $MAX_ADDRESS_LABEL_LENGTH 个字符）"
        }
        return null
    }

    /** 保存地址簿条目；同一地址仅更新备注，不重复插入。 */
    fun saveAddressBookEntry(address: String, label: String) {
        val trimmedAddress = address.trim()
        validateUserAddress(trimmedAddress)?.let {
            _addressBookMessage.value = it
            return
        }
        val name = label.trim().ifBlank { "未命名" }
        if (name.length > MAX_ADDRESS_LABEL_LENGTH) {
            _addressBookMessage.value = "备注过长（最多 $MAX_ADDRESS_LABEL_LENGTH 个字符）"
            return
        }
        viewModelScope.launch {
            val existed = _addressBook.value.any { it.address == trimmedAddress }
            runCatching { walletRepository.saveAddressBookEntry(trimmedAddress, name) }
                .onSuccess {
                    _addressBook.value = walletRepository.getAddressBook()
                    _addressBookMessage.value = if (existed) "已更新该地址的备注" else "已添加到地址簿"
                }
                .onFailure {
                    _addressBookMessage.value = "保存失败：${it.message ?: it.javaClass.simpleName}"
                }
        }
    }

    fun deleteAddressBookEntry(id: Long) {
        viewModelScope.launch {
            runCatching { walletRepository.deleteAddressBookEntry(id) }
                .onSuccess {
                    _addressBook.value = walletRepository.getAddressBook()
                    _addressBookMessage.value = "已从地址簿删除"
                }
                .onFailure {
                    _addressBookMessage.value = "删除失败：${it.message ?: it.javaClass.simpleName}"
                }
        }
    }

    fun clearAddressBookMessage() {
        _addressBookMessage.value = null
    }

    private fun parseAmountMc(input: String): Long? {
        val text = input.trim()
        if (!AMOUNT_PATTERN.matches(text)) return null
        return try {
            val value = BigDecimal(text)
            if (value.scale() > 6) return null
            value.movePointRight(6).longValueExact()
        } catch (e: Exception) {
            null
        }
    }

    private data class RefreshSnapshot(
        val accountFound: Boolean,
        val online: Boolean,
        val delegations: List<Delegation>,
        val chainTxs: List<ChainTxRecord>?,
    )

    companion object {
        /** 链上交易历史一次最多展示的条数。 */
        const val CHAIN_TX_LIMIT = 20

        /** 地址簿备注长度上限。 */
        const val MAX_ADDRESS_LABEL_LENGTH = 24

        /** 只接受普通十进制、最多 6 位小数，拒绝科学计数法与负号。 */
        val AMOUNT_PATTERN = Regex("^\\d{1,18}(\\.\\d{1,6})?$")
    }
}

private fun formatMc(umc: Long): String =
    DecimalFormat("#,##0.######").format(BigDecimal.valueOf(umc, 6)) + " MC"
