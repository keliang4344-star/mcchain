package com.mcchain.miner.domain.wallet

import com.mcchain.miner.MCParams
import com.google.gson.Gson
import com.google.gson.annotations.SerializedName
import java.security.PrivateKey
import javax.inject.Inject
import javax.inject.Singleton

/**
 * Cosmos SDK StdTx 构建器。
 * 支持 send/delegate/undelegate/claim_rewards 等标准消息。
 */
@Singleton
class TxBuilder @Inject constructor(
    private val walletManager: WalletManager,
    private val gson: Gson
) {
    data class Coin(
        @SerializedName("denom") val denom: String,
        @SerializedName("amount") val amount: String
    )

    data class StdFee(
        @SerializedName("amount") val amount: List<Coin>,
        @SerializedName("gas") val gas: String
    )

    data class StdSignature(
        @SerializedName("pub_key") val pubKey: PubKeyValue,
        @SerializedName("signature") val signature: String
    )

    data class PubKeyValue(
        @SerializedName("type") val type: String = "tendermint/PubKeySecp256k1",
        @SerializedName("value") val value: String
    )

    data class StdTx(
        @SerializedName("msg") val msg: List<Any>,
        @SerializedName("fee") val fee: StdFee,
        @SerializedName("signatures") val signatures: List<StdSignature>,
        @SerializedName("memo") val memo: String
    )

    data class SendMsg(
        @SerializedName("type") val type: String = "cosmos-sdk/MsgSend",
        @SerializedName("value") val value: SendMsgValue
    )

    data class SendMsgValue(
        @SerializedName("from_address") val from: String,
        @SerializedName("to_address") val to: String,
        @SerializedName("amount") val amount: List<Coin>
    )

    data class DelegateMsg(
        @SerializedName("type") val type: String = "cosmos-sdk/MsgDelegate",
        @SerializedName("value") val value: DelegateMsgValue
    )

    data class DelegateMsgValue(
        @SerializedName("delegator_address") val delegator: String,
        @SerializedName("validator_address") val validator: String,
        @SerializedName("amount") val amount: Coin
    )

    /**
     * 构建转账交易并签名
     */
    fun buildSendTx(
        fromAddress: String,
        toAddress: String,
        amountUmc: Long,
        denom: String = MCParams.DENOM,
        gas: Long = 200_000,
        memo: String = "",
        accountNumber: Long,
        sequence: Long,
        privateKey: PrivateKey
    ): String {
        val sendMsg = SendMsg(value = SendMsgValue(
            from = fromAddress,
            to = toAddress,
            amount = listOf(Coin(denom, amountUmc.toString()))
        ))

        return buildAndSign(
            msgs = listOf(sendMsg),
            gas = gas,
            memo = memo,
            accountNumber = accountNumber,
            sequence = sequence,
            privateKey = privateKey
        )
    }

    /**
     * 构建质押交易
     */
    fun buildDelegateTx(
        delegator: String,
        validator: String,
        amountUmc: Long,
        gas: Long = 250_000,
        accountNumber: Long,
        sequence: Long,
        privateKey: PrivateKey
    ): String {
        val delegateMsg = DelegateMsg(value = DelegateMsgValue(
            delegator = delegator,
            validator = validator,
            amount = Coin(MCParams.DENOM, amountUmc.toString())
        ))

        return buildAndSign(
            msgs = listOf(delegateMsg),
            gas = gas,
            memo = "",
            accountNumber = accountNumber,
            sequence = sequence,
            privateKey = privateKey
        )
    }

    private fun buildAndSign(
        msgs: List<Any>,
        gas: Long,
        memo: String,
        accountNumber: Long,
        sequence: Long,
        privateKey: PrivateKey
    ): String {
        val fee = StdFee(
            amount = emptyList(),
            gas = gas.toString()
        )

        // 构建签名文档 (Cosmos 标准)
        val signDoc = mapOf(
            "account_number" to accountNumber.toString(),
            "chain_id" to MCParams.CHAIN_ID,
            "fee" to fee,
            "memo" to memo,
            "msgs" to msgs,
            "sequence" to sequence.toString()
        )

        val signBytes = gson.toJson(signDoc).toByteArray(Charsets.UTF_8)
        val signature = walletManager.signWithPrivateKey(privateKey, signBytes)

        val pubKeyBytes = getPublicKeyBytes(privateKey)
        val pubKey = PubKeyValue(value = android.util.Base64.encodeToString(pubKeyBytes, android.util.Base64.NO_WRAP))

        val stdSig = StdSignature(
            pubKey = pubKey,
            signature = android.util.Base64.encodeToString(signature, android.util.Base64.NO_WRAP)
        )

        val stdTx = StdTx(
            msg = msgs,
            fee = fee,
            signatures = listOf(stdSig),
            memo = memo
        )

        return gson.toJson(stdTx)
    }

    private fun getPublicKeyBytes(privateKey: PrivateKey): ByteArray {
        // 从 EC 私钥计算公钥
        val ecSpec = org.bouncycastle.jce.ECNamedCurveTable.getParameterSpec("secp256k1")
        val keyFactory = java.security.KeyFactory.getInstance("ECDSA", "BC")
        val ecPrivateKey = privateKey as org.bouncycastle.jce.interfaces.ECPrivateKey
        val Q = ecSpec.g.multiply(ecPrivateKey.d).normalize()

        // Cosmos 格式：33 字节压缩公钥
        val x = Q.affineXCoord.toBigInteger().toByteArray()
        val y = Q.affineYCoord.toBigInteger()
        val prefix = if (y.testBit(0)) 0x03.toByte() else 0x02.toByte()
        return byteArrayOf(prefix) + x.takeLast(32).let {
            if (it.size < 32) ByteArray(32 - it.size) + it else it
        }
    }
}
