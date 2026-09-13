package com.mcchain.miner.data.repository

import com.mcchain.miner.data.db.AccountDao
import com.mcchain.miner.data.db.TxDao
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.TxStatus
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.network.RpcClient
import com.mcchain.miner.domain.wallet.WalletManager
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class WalletRepository @Inject constructor(
    private val accountDao: AccountDao,
    private val txDao: TxDao,
    private val walletManager: WalletManager,
    private val rpcClient: RpcClient
) {
    suspend fun getAllAccounts(): List<WalletAccount> =
        accountDao.getAllAccounts()

    suspend fun getAccount(address: String): WalletAccount? =
        accountDao.getAccount(address)

    suspend fun getDefaultAccount(): WalletAccount? =
        accountDao.getDefaultAccount()

    suspend fun insertAccount(account: WalletAccount) =
        accountDao.insertAccount(account)

    suspend fun updateBalances(
        address: String, balance: Long, stake: Long, reward: Long,
        sequence: Long, accountNumber: Long
    ) = accountDao.updateBalances(address, balance, stake, reward, sequence, accountNumber)

    suspend fun clearDefaultFlags() =
        accountDao.clearDefaultFlags()

    suspend fun setDefault(address: String) =
        accountDao.setDefault(address)

    suspend fun getTransactions(from: Long, to: Long): List<TxRecord> =
        txDao.getTransactions(from, to)

    suspend fun getTransactionsForAddress(address: String, limit: Int = 100): List<TxRecord> =
        txDao.getTransactionsForAddress(address, limit)

    suspend fun insertTx(tx: TxRecord) =
        txDao.insertTx(tx)

    suspend fun insertTxs(txs: List<TxRecord>) =
        txDao.insertTxs(txs)

    suspend fun getPendingTxs(): List<TxRecord> =
        txDao.getPendingTxs()

    suspend fun updateTxStatus(hash: String, status: TxStatus) =
        txDao.updateTxStatus(hash, status)

    suspend fun pruneTxs(beforeHeight: Long): Int =
        txDao.pruneTxs(beforeHeight)
}
