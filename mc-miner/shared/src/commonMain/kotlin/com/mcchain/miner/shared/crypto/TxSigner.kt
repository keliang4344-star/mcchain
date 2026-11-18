package com.mcchain.miner.shared.crypto

/**
 * 交易签名抽象。实现必须保证与 Cosmos SDK `crypto/keys/secp256k1` 字节级一致：
 *  - secp256k1 + RFC6979 确定性 nonce（HMAC-SHA256）
 *  - 对入参摘要直接签名（调用方已做 SHA-256）
 *  - low-S 规范化，输出 64 字节 r||s（大端，各 32 字节，不补 DER 头）
 */
interface TxSigner {
    /** 33 字节压缩公钥。 */
    fun publicKeyCompressed(): ByteArray

    /** @param digest32 SHA-256 摘要（32 字节） */
    fun signDigest(digest32: ByteArray): ByteArray
}
