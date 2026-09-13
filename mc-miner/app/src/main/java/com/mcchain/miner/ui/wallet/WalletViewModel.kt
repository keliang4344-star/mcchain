package com.mcchain.miner.ui.wallet

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mcchain.miner.data.db.AccountDao
import com.mcchain.miner.data.db.TxDao
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.WalletAccount
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@HiltViewModel
class WalletViewModel @Inject constructor(
    private val accountDao: AccountDao,
    private val txDao: TxDao
) : ViewModel() {

    private val _accounts = MutableStateFlow<List<WalletAccount>>(emptyList())
    val accounts: StateFlow<List<WalletAccount>> = _accounts

    private val _transactions = MutableStateFlow<List<TxRecord>>(emptyList())
    val transactions: StateFlow<List<TxRecord>> = _transactions

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _accounts.value = accountDao.getAllAccounts()
            val defaultAddr = _accounts.value.firstOrNull { it.isDefault }?.address
            if (defaultAddr != null) {
                _transactions.value = txDao.getTransactionsForAddress(defaultAddr, 50)
            }
        }
    }
}
