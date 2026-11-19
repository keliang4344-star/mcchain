package com.mcchain.miner.domain.wallet

import com.mcchain.miner.MCParams
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosMsgs
import com.mcchain.miner.shared.protocol.DepinTx
import com.mcchain.miner.shared.protocol.PhonenodeTx
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

    /**
     * 设备注册（depin.MsgRegisterDevice）。
     * 链上要求 address 必须等于签名者，因此 creator 与 address 都填当前钱包地址。
     */
    fun buildRegisterDeviceTx(
        model: String,
        os: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "register-device",
        gas: Long = DEFAULT_GAS_REGISTER,
        feeAmount: String = DEFAULT_FEE_UMC,
    ): BuiltTx {
        val wallet = requireWallet()
        val message = DepinTx.registerDevice(
            creator = wallet.address,
            address = wallet.address,
            model = model,
            os = os,
        )
        return build(wallet, listOf(message), accountNumber, sequence, memo, gas, feeAmount)
    }

    /**
     * 设备认证（depin.MsgAttestDevice）。
     * challenge 与 signature 由认证预言机签发；链上 TeeOracle 会对 deviceAddr|challenge 验签，
     * 并记录 challenge 已消费（重放会被拒绝）。
     */
    fun buildAttestDeviceTx(
        challenge: String,
        signatureBase64: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "attest-device",
        gas: Long = DEFAULT_GAS_ATTEST,
        feeAmount: String = DEFAULT_FEE_UMC,
    ): BuiltTx {
        val wallet = requireWallet()
        val message = DepinTx.attestDevice(
            creator = wallet.address,
            address = wallet.address,
            challenge = challenge,
            signature = signatureBase64,
        )
        return build(wallet, listOf(message), accountNumber, sequence, memo, gas, feeAmount)
    }

    /**
     * 贡献提交（depin.MsgSubmitContribution）。
     * taskId 为链上幂等键（Contribution:<taskID> 唯一），重复提交返回 ErrTaskExists。
     * taskType 必须是链上支持的 inference / data_label / bandwidth，score 为 [0,100] 整数字符串。
     */
    fun buildSubmitContributionTx(
        taskId: String,
        taskType: String,
        score: Int,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
        gas: Long = DEFAULT_GAS_CONTRIB,
        feeAmount: String = DEFAULT_FEE_UMC,
    ): BuiltTx {
        val wallet = requireWallet()
        val message = DepinTx.submitContribution(
            creator = wallet.address,
            taskId = taskId,
            taskType = taskType,
            score = score.toString(),
        )
        return build(wallet, listOf(message), accountNumber, sequence, memo, gas, feeAmount)
    }

    /**
     * 移动节点注册（phonenode.MsgRegisterNode）。depin 贡献发币闸口要求
     * Creator 地址已在 phonenode 注册（HasNode），重复注册返回 ErrNodeExists。
     */
    fun buildRegisterNodeTx(
        model: String,
        os: String,
        role: String = "depin",
        accountNumber: Long,
        sequence: Long,
        memo: String = "register-node",
        gas: Long = DEFAULT_GAS_REGISTER,
        feeAmount: String = DEFAULT_FEE_UMC,
    ): BuiltTx {
        val wallet = requireWallet()
        val message = PhonenodeTx.registerNode(
            creator = wallet.address,
            address = wallet.address,
            model = model,
            os = os,
            role = role,
        )
        return build(wallet, listOf(message), accountNumber, sequence, memo, gas, feeAmount)
    }

    /**
     * 设备自签 attestation（phonenode.MsgSubmitAttestation）。
     * signatureHex = 设备私钥对 "deviceIdHash|nonce" 的签名（64 字节 r||s hex）。
     */
    fun buildPhonenodeAttestTx(
        rootHash: String,
        nonce: String,
        deviceIdHash: String,
        devicePubKeyHex: String,
        signatureHex: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "attest-node",
        gas: Long = DEFAULT_GAS_ATTEST,
        feeAmount: String = DEFAULT_FEE_UMC,
    ): BuiltTx {
        val wallet = requireWallet()
        val message = PhonenodeTx.submitAttestation(
            creator = wallet.address,
            rootHash = rootHash,
            nonce = nonce,
            deviceIdHash = deviceIdHash,
            devicePubKeyHex = devicePubKeyHex,
            signatureHex = signatureHex,
        )
        return build(wallet, listOf(message), accountNumber, sequence, memo, gas, feeAmount)
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
        const val DEFAULT_GAS_REGISTER = 200_000L
        const val DEFAULT_GAS_ATTEST = 250_000L
        const val DEFAULT_GAS_CONTRIB = 350_000L
    }
}
