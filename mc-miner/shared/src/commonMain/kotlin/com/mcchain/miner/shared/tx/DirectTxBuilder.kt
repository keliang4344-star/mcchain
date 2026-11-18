package com.mcchain.miner.shared.tx

import com.mcchain.miner.shared.crypto.TxSigner
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosTxEncoder
import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Sha256

/**
 * 构造一笔 SIGN_MODE_DIRECT 交易所需的全部输入。
 * messages 为 Any 编码后的字节，使用 [com.mcchain.miner.shared.protocol.DepinTx] 生成。
 */
data class DirectTxInput(
    val messages: List<ByteArray>,
    val chainId: String,
    val accountNumber: Long,
    val sequence: Long,
    val fee: List<Coin>,
    val gasLimit: Long,
    val memo: String = "",
    val timeoutHeight: Long = 0L,
)

data class SignedTx(
    val publicKey: ByteArray,
    val bodyBytes: ByteArray,
    val authInfoBytes: ByteArray,
    val signDocBytes: ByteArray,
    val signDocSha256: ByteArray,
    val signature: ByteArray,
    val txRawBytes: ByteArray,
) {
    /** Cosmos RPC 使用大写十六进制。 */
    val txHashHex: String get() = Hex.encode(Sha256.digest(txRawBytes), upper = true)
}

object DirectTxBuilder {
    fun build(input: DirectTxInput, signer: TxSigner): SignedTx {
        val publicKey = signer.publicKeyCompressed()
        val bodyBytes = CosmosTxEncoder.encodeTxBody(
            messages = input.messages,
            memo = input.memo,
            timeoutHeight = input.timeoutHeight,
        )
        val feeBytes = CosmosTxEncoder.encodeFee(input.fee, input.gasLimit)
        val authInfoBytes = CosmosTxEncoder.encodeAuthInfo(
            signerInfos = listOf(CosmosTxEncoder.encodeSignerInfo(publicKey, input.sequence)),
            fee = feeBytes,
        )
        val signDocBytes = CosmosTxEncoder.encodeSignDoc(
            bodyBytes = bodyBytes,
            authInfoBytes = authInfoBytes,
            chainId = input.chainId,
            accountNumber = input.accountNumber,
        )
        val signDocSha256 = Sha256.digest(signDocBytes)
        val signature = signer.signDigest(signDocSha256)
        val txRawBytes = CosmosTxEncoder.encodeTxRaw(
            bodyBytes = bodyBytes,
            authInfoBytes = authInfoBytes,
            signatures = listOf(signature),
        )
        return SignedTx(
            publicKey = publicKey,
            bodyBytes = bodyBytes,
            authInfoBytes = authInfoBytes,
            signDocBytes = signDocBytes,
            signDocSha256 = signDocSha256,
            signature = signature,
            txRawBytes = txRawBytes,
        )
    }
}
