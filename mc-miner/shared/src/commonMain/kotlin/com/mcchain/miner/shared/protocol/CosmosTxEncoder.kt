package com.mcchain.miner.shared.protocol

/** cosmos.base.v1beta1.Coin */
data class Coin(
    val denom: String,
    val amount: String,
)

/**
 * Cosmos SDK v0.47 的 protobuf 交易结构编码（SIGN_MODE_DIRECT）。
 *
 * 字段编号与链仓库 proto / cosmos-sdk 保持一致，顺序按字段号升序写出，
 * 与 gogoproto 生成的 Marshal 输出逐字节一致。
 */
object CosmosTxEncoder {
    const val SIGN_MODE_DIRECT = 1

    private const val TYPE_PUBKEY_SECP256K1 = "/cosmos.crypto.secp256k1.PubKey"

    /** google.protobuf.Any */
    fun encodeAny(typeUrl: String, value: ByteArray): ByteArray = ProtoWriter().apply {
        writeStringField(1, typeUrl)
        writeBytesField(2, value)
    }.toByteArray()

    /** cosmos.crypto.secp256k1.PubKey */
    fun encodeSecp256k1PublicKey(publicKey33: ByteArray): ByteArray {
        require(publicKey33.size == 33) { "secp256k1 compressed public key must be 33 bytes" }
        return ProtoWriter().apply { writeBytesField(1, publicKey33) }.toByteArray()
    }

    fun encodeCoin(coin: Coin): ByteArray = ProtoWriter().apply {
        writeStringField(1, coin.denom)
        writeStringField(2, coin.amount)
    }.toByteArray()

    fun encodeFee(
        amounts: List<Coin>,
        gasLimit: Long,
        payer: String = "",
        granter: String = "",
    ): ByteArray = ProtoWriter().apply {
        amounts.forEach { writeMessageField(1, encodeCoin(it)) }
        writeVarintField(2, gasLimit)
        writeStringField(3, payer)
        writeStringField(4, granter)
    }.toByteArray()

    fun encodeSignerInfo(
        publicKey33: ByteArray,
        sequence: Long,
        signMode: Int = SIGN_MODE_DIRECT,
    ): ByteArray {
        val publicKeyAny = encodeAny(TYPE_PUBKEY_SECP256K1, encodeSecp256k1PublicKey(publicKey33))
        // ModeInfo.single = 1；Single.mode = 1（SIGN_MODE_DIRECT）
        val single = ProtoWriter().apply { writeVarintField(1, signMode.toLong()) }.toByteArray()
        val modeInfo = ProtoWriter().apply { writeMessageField(1, single) }.toByteArray()
        return ProtoWriter().apply {
            writeMessageField(1, publicKeyAny)
            writeMessageField(2, modeInfo)
            writeVarintField(3, sequence)
        }.toByteArray()
    }

    /** AuthInfo.tip 为 nil，本方法不写字段 3。 */
    fun encodeAuthInfo(signerInfos: List<ByteArray>, fee: ByteArray): ByteArray = ProtoWriter().apply {
        signerInfos.forEach { writeMessageField(1, it) }
        writeMessageField(2, fee)
    }.toByteArray()

    fun encodeTxBody(
        messages: List<ByteArray>,
        memo: String = "",
        timeoutHeight: Long = 0L,
    ): ByteArray = ProtoWriter().apply {
        messages.forEach { writeMessageField(1, it) }
        writeStringField(2, memo)
        writeVarintField(3, timeoutHeight)
    }.toByteArray()

    /** cosmos.tx.v1beta1.SignDoc */
    fun encodeSignDoc(
        bodyBytes: ByteArray,
        authInfoBytes: ByteArray,
        chainId: String,
        accountNumber: Long,
    ): ByteArray = ProtoWriter().apply {
        writeBytesField(1, bodyBytes)
        writeBytesField(2, authInfoBytes)
        writeStringField(3, chainId)
        writeVarintField(4, accountNumber)
    }.toByteArray()

    /** cosmos.tx.v1beta1.TxRaw */
    fun encodeTxRaw(
        bodyBytes: ByteArray,
        authInfoBytes: ByteArray,
        signatures: List<ByteArray>,
    ): ByteArray = ProtoWriter().apply {
        writeBytesField(1, bodyBytes)
        writeBytesField(2, authInfoBytes)
        signatures.forEach { writeBytesField(3, it) }
    }.toByteArray()
}
