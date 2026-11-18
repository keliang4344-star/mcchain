package com.mcchain.miner.ui.wallet

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.data.repository.WalletRepository
import com.mcchain.miner.domain.wallet.TransactionService
import com.mcchain.miner.domain.wallet.WalletManager
import com.mcchain.miner.shared.util.Hex
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.math.BigDecimal
import javax.inject.Inject

@HiltViewModel
class WalletViewModel @Inject constructor(
    private val walletRepository: WalletRepository,
    private val walletManager: WalletManager,
    private val transactionService: TransactionService,
) : ViewModel() {

    private val _accounts = MutableStateFlow<List<WalletAccount>>(emptyList())
    val accounts: StateFlow<List<WalletAccount>> = _accounts

    private val _transactions = MutableStateFlow<List<TxRecord>>(emptyList())
    val transactions: StateFlow<List<TxRecord>> = _transactions

    /** 创建钱包时待用户备份的助记词；确认前不落盘。 */
    private val _backupMnemonic = MutableStateFlow<List<String>?>(null)
    val backupMnemonic: StateFlow<List<String>?> = _backupMnemonic

    private val _busy = MutableStateFlow(false)
    val busy: StateFlow<Boolean> = _busy

    private val _message = MutableStateFlow<String?>(null)
    val message: StateFlow<String?> = _message

    init {
        refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            _accounts.value = walletRepository.getAllAccounts()
            val default = _accounts.value.firstOrNull { it.isDefault } ?: return@launch

            _transactions.value = walletRepository.getTransactionsForAddress(default.address, 50)
            withContext(Dispatchers.IO) {
                // 先重放离线队列，再刷新余额：队列里的交易可能已改变余额与 sequence。
                transactionService.replayPending()
                transactionService.refreshAccount(default.address)
                // 首次有余额时自动完成 depin.MsgRegisterDevice（未注册才发起）。
                transactionService.ensureDeviceRegistered(
                    model = android.os.Build.MODEL,
                    os = "Android ${android.os.Build.VERSION.RELEASE}",
                )?.takeUnless { it.success }?.let { _message.value = it.message }
            }
            _accounts.value = walletRepository.getAllAccounts()
        }
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

    fun importWallet(words: String) {
        val mnemonic = words.trim().split(Regex("\\s+")).filter { it.isNotBlank() }
        if (!walletManager.isValidMnemonic(mnemonic)) {
            _message.value = "助记词无效：请检查单词、顺序与校验和"
            return
        }
        persistWallet(mnemonic)
    }

    fun send(toAddress: String, amountMc: String) {
        val amount = parseAmountMc(amountMc)
        if (amount == null) {
            _message.value = "金额格式无效（最多 6 位小数）"
            return
        }
        viewModelScope.launch {
            _busy.value = true
            val result = withContext(Dispatchers.IO) {
                transactionService.send(toAddress.trim(), amount)
            }
            _message.value = result.message
            _busy.value = false
            refresh()
        }
    }

    fun claimRewards() {
        viewModelScope.launch {
            _busy.value = true
            val result = withContext(Dispatchers.IO) {
                transactionService.claimAllRewards()
            }
            _message.value = result.message
            _busy.value = false
            refresh()
        }
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

    private fun parseAmountMc(input: String): Long? {
        val text = input.trim()
        if (text.isEmpty()) return null
        return try {
            val value = BigDecimal(text)
            if (value.signum() <= 0 || value.scale() > 6) return null
            value.movePointRight(6).longValueExact()
        } catch (e: Exception) {
            null
        }
    }
}
