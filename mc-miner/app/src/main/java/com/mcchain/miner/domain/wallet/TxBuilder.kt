package com.mcchain.miner.domain.wallet

import com.mcchain.miner.MCParams
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosMsgs
import com.mcchain.miner.shared.tx.DirectTxBuilder
import com.mcchain.miner.shared.tx.DirectTxInput
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Cosmos SDK v0.47 交易构造器（SIGN_MODE_DIRECT / TxRaw）。
 *
 * 输出为 protobuf TxRaw 字节，可直接 base64 后提交到 REST `/cosmos/tx/v1beta1/txs`。
 * 旧版 Amino JSON + DER 签名与 v0.47 链不兼容，已全部移除。
 */
@Singleton
class TxBuilder @Inject constructor(
    private val walletManager: WalletManager
) {
    data class BuiltTx(
        val txRaw: ByteArray,
        val txHashHex: String,
        val fromAddress: String,
    )

    fun buildSendTx(
        toAddress: String,
        amountUmc: Long,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
        gas: Long = DEFAULT_GAS_SEND,
        feeAmount: String = DEFAULT_FEE_UMC,
    ): BuiltTx {
        val wallet = requireWallet()
        val message = CosmosMsgs.send(
            fromAddress = wallet.address,
            toAddress = toAddress,
            amount = Coin(MCParams.DENOM, amountUmc.toString()),
        )
        return build(wallet, listOf(message), accountNumber, sequence, memo, gas, feeAmount)
    }

    fun buildDelegateTx(
        validatorAddress: String,
        amountUmc: Long,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
        gas: Long = DEFAULT_GAS_DELEGATE,
        feeAmount: String = DEFAULT_FEE_UMC,
    ): BuiltTx {
        val wallet = requireWallet()
        val message = CosmosMsgs.delegate(
            delegatorAddress = wallet.address,
            validatorAddress = validatorAddress,
            amount = Coin(MCParams.DENOM, amountUmc.toString()),
        )
        return build(wallet, listOf(message), accountNumber, sequence, memo, gas, feeAmount)
    }

    /** 一次交易领取多个验证人的待领奖励。 */
    fun buildClaimRewardsTx(
        validatorAddresses: List<String>,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
        gas: Long = DEFAULT_GAS_CLAIM,
        feeAmount: String = DEFAULT_FEE_UMC,
    ): BuiltTx {
        require(validatorAddresses.isNotEmpty()) { "至少需要一个验证人地址" }
        val wallet = requireWallet()
        val messages = validatorAddresses.map {
            CosmosMsgs.withdrawDelegatorReward(wallet.address, it)
        }
        return build(wallet, messages, accountNumber, sequence, memo, gas, feeAmount)
    }

    private fun requireWallet(): DerivedWallet =
        walletManager.getStoredWallet() ?: error("钱包未初始化：请先创建或导入助记词")

    private fun build(
        wallet: DerivedWallet,
        messages: List<ByteArray>,
        accountNumber: Long,
        sequence: Long,
        memo: String,
        gas: Long,
        feeAmount: String,
    ): BuiltTx {
        val signed = DirectTxBuilder.build(
            input = DirectTxInput(
                messages = messages,
                chainId = MCParams.CHAIN_ID,
                accountNumber = accountNumber,
                sequence = sequence,
                fee = listOf(Coin(MCParams.DENOM, feeAmount)),
                gasLimit = gas,
                memo = memo,
            ),
            signer = Secp256k1Signer(wallet.privateKey),
        )
        return BuiltTx(
            txRaw = signed.txRawBytes,
            txHashHex = signed.txHashHex,
            fromAddress = wallet.address,
        )
    }

    companion object {
        const val DEFAULT_FEE_UMC = "5000"
        const val DEFAULT_GAS_SEND = 200_000L
        const val DEFAULT_GAS_DELEGATE = 250_000L
        const val DEFAULT_GAS_CLAIM = 300_000L
    }
}
