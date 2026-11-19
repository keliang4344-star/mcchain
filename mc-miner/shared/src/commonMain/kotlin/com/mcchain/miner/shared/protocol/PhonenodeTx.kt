package com.mcchain.miner.shared.protocol

/**
 * mcchain.phonenode 交易消息编码（与 contracts/mcchain/phonenode/tx.proto 字段号一致）。
 *
 * 移动端只用这两条：
 *  - MsgRegisterNode：把钱包地址注册为移动节点（depin 贡献发币闸口要求 HasNode）；
 *  - MsgSubmitAttestation：设备自签 attestation（pubkey+signature 绑定 device_id_hash|nonce），
 *    depin 防刷量防线 Layer2 要求 phonenode.IsAttested。
 * MsgSubmitStateProof / UpdateVerifierStatus 等由节点/治理侧使用，不在此提供。
 */
object PhonenodeTx {
    const val TYPE_MSG_REGISTER_NODE = "/mcchain.phonenode.MsgRegisterNode"
    const val TYPE_MSG_SUBMIT_ATTESTATION = "/mcchain.phonenode.MsgSubmitAttestation"

    /** creator 必须与签名账户一致；role 为链上合法角色（depin / validator / fullnode）。 */
    fun registerNode(
        creator: String,
        address: String,
        model: String,
        os: String,
        role: String,
    ): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, address)
            writeStringField(3, model)
            writeStringField(4, os)
            writeStringField(5, role)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_REGISTER_NODE, value)
    }

    /**
     * 设备自签 attestation。
     * signature 为设备私钥对 "deviceIdHash|nonce" 的 secp256k1 签名（64 字节 r||s，hex）；
     * devicePubKey 为 33 字节压缩公钥 hex。链上会验签并绑定公钥（轮换会被拒）。
     */
    fun submitAttestation(
        creator: String,
        rootHash: String,
        nonce: String,
        deviceIdHash: String,
        devicePubKeyHex: String,
        signatureHex: String,
    ): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, rootHash)
            writeStringField(3, nonce)
            writeStringField(4, deviceIdHash)
            writeStringField(5, devicePubKeyHex)
            writeStringField(6, signatureHex)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_SUBMIT_ATTESTATION, value)
    }
}
