package com.mcchain.miner.ui.wallet

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.domain.wallet.TransactionService
import com.mcchain.miner.domain.wallet.TxBuilder
import com.mcchain.miner.domain.wallet.WalletManager
import com.mcchain.miner.network.Delegation
import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.network.TxResult
import com.mcchain.miner.network.ValidatorInfo
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

    private val _transactions = MutableStateFlow<List<TxRecord>>(emptyList())
    val transactions: StateFlow<List<TxRecord>> = _transactions

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
                _accounts.value = walletRepository.getAllAccounts()
                val default = _accounts.value.firstOrNull { it.isDefault }
                if (default == null) {
                    _transactions.value = emptyList()
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
                    ensureDeviceRegistered()
                    RefreshSnapshot(accountFound, online, delegations)
                }

                _accounts.value = walletRepository.getAllAccounts()
                _transactions.value = walletRepository
                    .getTransactionsForAddress(default.address, 50)
                    .sortedByDescending { it.timestamp }
                _delegations.value = snapshot.delegations

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

    private fun persistWallet(mnemonic: List<String>) {
        viewModelScope.launch {
            _busy.value = true
            try {
                val derived = withContext(Dispatchers.IO) {
                    walletManager.storeMnemonic(mnemonic)
                    walletManager.deriveWallet(mnemonic)
                }
                walletRepository.clearDefaultFlags()
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

    private fun validatorFormatError(validator: String): String? {
        if (validator.isEmpty()) return "请输入验证人地址"
        if (!validator.startsWith(MCParams.BECH32_PREFIX + "valoper1")) {
            return "验证人地址必须以 ${MCParams.BECH32_PREFIX}valoper1 开头"
        }
        if (validator.length !in 48..62) return "验证人地址长度异常，请检查是否粘贴完整"
        if (validator.any { it !in BECH32_CHARSET }) return "验证人地址包含非法字符"
        return null
    }

    private fun validateUserAddress(address: String): String? {
        if (address.isEmpty()) return "请输入收款地址"
        val prefix = MCParams.BECH32_PREFIX + "1"
        if (!address.startsWith(prefix)) return "收款地址必须以 $prefix 开头"
        if (address.length !in 39..59) return "收款地址长度异常，请检查是否粘贴完整"
        if (address.any { it !in BECH32_CHARSET }) return "收款地址包含非法字符，请检查是否复制正确"
        return null
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
    )

    private companion object {
        /** bech32 数据字符集（不含分隔符 1）。 */
        const val BECH32_CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

        /** 只接受普通十进制、最多 6 位小数，拒绝科学计数法与负号。 */
        val AMOUNT_PATTERN = Regex("^\\d{1,18}(\\.\\d{1,6})?$")
    }
}

private fun formatMc(umc: Long): String =
    DecimalFormat("#,##0.######").format(BigDecimal.valueOf(umc, 6)) + " MC"
