package com.mcchain.miner.shared.protocol

/**
 * mcchain.phonenode 交易消息编码（与 contracts/mcchain/phonenode/tx.proto 字段号一致）。
 *
 * 移动端三条：
 *  - MsgRegisterNode：把钱包地址注册为移动节点（depin 贡献发币闸口要求 HasNode）；
 *  - MsgSubmitAttestation：设备自签 attestation（pubkey+signature 绑定 device_id_hash|nonce），
 *    depin 防刷量防线 Layer2 要求 phonenode.IsAttested；
 *  - MsgSubmitStateProof：在线心跳，防止离线 slash（revoke attestation + 冷却）。
 * UpdateVerifierStatus 等治理消息由链侧使用，不在此提供。
 */
object PhonenodeTx {
    const val TYPE_MSG_REGISTER_NODE = "/mcchain.phonenode.MsgRegisterNode"
    const val TYPE_MSG_SUBMIT_ATTESTATION = "/mcchain.phonenode.MsgSubmitAttestation"
    const val TYPE_MSG_SUBMIT_STATE_PROOF = "/mcchain.phonenode.MsgSubmitStateProof"

    /**
     * 在线心跳（MsgSubmitStateProof）。链上验证 (leaf,index,proof) 能重建出 root：
     * 本构造函数用单叶树（root=SHA256(leaf)、index=0、proof 空），App 可自证在线。
     * 不按时心跳的节点会被离线 slash（revoke attestation + 冷却期禁止重认证）。
     */
    fun submitStateProof(creator: String, root: String, leaf: String, index: String, proof: String): ByteArray {
        val value = ProtoWriter().apply {
            writeStringField(1, creator)
            writeStringField(2, root)
            writeStringField(3, leaf)
            writeStringField(4, index)
            writeStringField(5, proof)
        }.toByteArray()
        return CosmosTxEncoder.encodeAny(TYPE_MSG_SUBMIT_STATE_PROOF, value)
    }

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
