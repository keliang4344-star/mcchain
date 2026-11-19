package com.mcchain.miner.shared.wallet

import com.mcchain.miner.shared.crypto.Secp256k1Math
import com.mcchain.miner.shared.util.Sha512

/**
 * BIP-32 私钥派生（secp256k1）。
 *
 * Cosmos 标准路径 m/44'/118'/0'/0/{accountIndex}。只实现私钥派生：
 *  - 硬化子键：HMAC key = 0x00 || ser256(k_par) || ser32(i)
 *  - 普通子键：HMAC key = serP(K_par) || ser32(i)
 *  - 子私钥 = (IL + k_par) mod n，IL 必须 < n 且结果非 0
 *
 * 派生逻辑与 Android 端 bitcoinj 实现等价，jvmTest 用 BIP-32 官方测试向量校验。
 */
object Bip32 {

    const val HARDENED_BIT = 0x80000000L
    private val SEED_KEY = "Bitcoin seed".encodeToByteArray()

    class PrivateNode(
        val privateKey: ByteArray,
        val chainCode: ByteArray,
        val depth: Int,
        val path: String,
    )

    fun fromSeed(seed: ByteArray): PrivateNode {
        val i = Sha512.hmac(SEED_KEY, seed)
        val key = i.copyOfRange(0, 32)
        require(!Secp256k1Math.isZero(Secp256k1Math.fromBytesBE(key))) { "BIP-32 主私钥为 0，种子无效" }
        return PrivateNode(key, i.copyOfRange(32, 64), 0, "m")
    }

    fun derive(parent: PrivateNode, index: Long): PrivateNode {
        require(index in 0L until (HARDENED_BIT * 2)) { "派生索引越界：$index" }
        val hardened = index >= HARDENED_BIT
        val indexBytes = ser32(index)
        val data = if (hardened) {
            byteArrayOf(0) + parent.privateKey + indexBytes
        } else {
            publicKeyOf(parent.privateKey) + indexBytes
        }
        val i = Sha512.hmac(parent.chainCode, data)
        val il = i.copyOfRange(0, 32)
        val ilLimbs = Secp256k1Math.fromBytesBE(il)
        require(!Secp256k1Math.gte(ilLimbs, Secp256k1Math.N.limbs)) { "BIP-32 派生 IL ≥ n，请更换索引" }
        val childLimbs = Secp256k1Math.addMod(
            ilLimbs,
            Secp256k1Math.fromBytesBE(parent.privateKey),
            Secp256k1Math.N,
        )
        require(!Secp256k1Math.isZero(childLimbs)) { "BIP-32 派生子私钥为 0，请更换索引" }
        val suffix = if (hardened) "${index - HARDENED_BIT}'" else index.toString()
        return PrivateNode(
            privateKey = Secp256k1Math.toBytesBE(childLimbs),
            chainCode = i.copyOfRange(32, 64),
            depth = parent.depth + 1,
            path = "${parent.path}/$suffix",
        )
    }

    /** 解析 "m/44'/118'/0'/0/0" 这类路径（' 与 h 均表示硬化）。 */
    fun parsePath(path: String): List<Long> {
        val parts = path.trim().split('/')
        require(parts.first() == "m") { "派生路径必须以 m 开头：$path" }
        return parts.drop(1).map { part ->
            val hardened = part.endsWith("'") || part.endsWith("h") || part.endsWith("H")
            val digits = if (hardened) part.dropLast(1) else part
            val value = digits.toLongOrNull() ?: throw IllegalArgumentException("非法派生路径段：$part")
            require(value in 0 until HARDENED_BIT) { "非法派生索引：$part" }
            if (hardened) value + HARDENED_BIT else value
        }
    }

    fun derivePath(seed: ByteArray, path: String): PrivateNode =
        parsePath(path).fold(fromSeed(seed)) { node, index -> derive(node, index) }

    /** 33 字节压缩公钥。 */
    fun publicKeyOf(privateKey: ByteArray): ByteArray {
        val point = Secp256k1Math.mulG(Secp256k1Math.fromBytesBE(privateKey))
        val (x, y) = Secp256k1Math.toAffine(point)
        return Secp256k1Math.compressedPubKey(x, y)
    }

    private fun ser32(index: Long): ByteArray = byteArrayOf(
        (index ushr 24).toByte(),
        (index ushr 16).toByte(),
        (index ushr 8).toByte(),
        index.toByte(),
    )
}
