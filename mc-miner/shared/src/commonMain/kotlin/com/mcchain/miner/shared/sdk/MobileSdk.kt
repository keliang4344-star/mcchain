package com.mcchain.miner.shared.sdk

import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosMsgs
import com.mcchain.miner.shared.protocol.DepinTx
import com.mcchain.miner.shared.protocol.EdgeAiTx
import com.mcchain.miner.shared.protocol.PhonenodeTx
import com.mcchain.miner.shared.tx.DirectTxBuilder
import com.mcchain.miner.shared.tx.DirectTxInput
import com.mcchain.miner.shared.util.Base64Encode
import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Sha256
import com.mcchain.miner.shared.wallet.McWallet

/** 设备自签 attestation 材料：签名 hex（r||s）与 33 字节压缩公钥 hex。 */
class AttestationSignature(
    val signatureHex: String,
    val devicePubKeyHex: String,
)

/** 钱包信息（hex 形式，便于 Swift 直接落 Keychain、展示地址）。 */
class WalletInfo(
    val address: String,
    val publicKeyHex: String,
    val privateKeyHex: String,
    val derivationPath: String,
)

/** 已签名交易的移动端交付物：base64 的 TxRaw 可直接 POST 到 REST 广播接口。 */
class SignedTxResult(
    val txRawBase64: String,
    val txHashHex: String,
    val fromAddress: String,
    val signDocSha256Hex: String,
)

/**
 * iOS / 跨端调用门面。
 *
 * 存在的意义：把 Kotlin/Native 的互操作面收敛到一个 object + 两个简单类上，
 * Swift 侧只处理 `String` / `Long` / `Int`，不需要在 Swift 里摆弄 `KotlinByteArray`；
 * 所有交易编码、摘要、secp256k1 签名都在 shared 内完成，与 Android 走同一份实现。
 *
 * 调用方（App 层）负责：账户号/sequence 查询、广播、回执轮询、幂等与重试。
 * 这里只做「确定性、无网络、可单测」的部分。
 */
object MobileSdk {

    const val VERSION = "1.0.0"
    const val DENOM = "umc"
    const val DEFAULT_FEE_UMC = "5000"

    const val GAS_SEND = 200_000L
    const val GAS_REGISTER = 200_000L
    const val GAS_ATTEST = 250_000L
    const val GAS_CONTRIB = 350_000L
    const val GAS_EDGEAI = 300_000L
    const val GAS_HEARTBEAT = 200_000L

    /** 心跳两叶 Merkle 树的固定兄弟叶。 */
    const val HEARTBEAT_PADDING = "mcchain-phone-node-heartbeat-padding"

    // ------------------------------------------------------------------ 钱包

    fun isValidMnemonic(mnemonic: String): Boolean = McWallet.isValidMnemonic(mnemonic)

    /** entropyHex 必须是 16/20/24/28/32 字节（12~24 词）。随机源由调用方提供。 */
    fun createMnemonic(entropyHex: String): String = McWallet.createMnemonic(Hex.decode(entropyHex))

    fun walletFromMnemonic(mnemonic: String, accountIndex: Int, bech32Prefix: String): WalletInfo {
        val w = McWallet.fromMnemonic(mnemonic, accountIndex = accountIndex, bech32Prefix = bech32Prefix)
        return WalletInfo(w.address, Hex.encode(w.publicKey), w.privateKeyHex(), w.derivationPath)
    }

    fun walletFromPrivateKey(privateKeyHex: String, bech32Prefix: String): WalletInfo {
        val w = McWallet.fromPrivateKeyHex(privateKeyHex, bech32Prefix = bech32Prefix)
        return WalletInfo(w.address, Hex.encode(w.publicKey), w.privateKeyHex(), w.derivationPath)
    }

    fun addressFromPublicKey(publicKeyHex: String, bech32Prefix: String): String =
        McWallet.addressOf(Hex.decode(publicKeyHex), bech32Prefix)

    /**
     * 设备自签 attestation（phonenode 防线 Layer2）：
     * 私钥对 SHA256("deviceIdHash|nonce") 签名，返回 64 字节 r||s 与压缩公钥。
     * deviceIdHash 由调用方对设备标识做 SHA256 后 hex 化（与 Android 端一致）。
     */
    fun signSelfAttestation(
        privateKeyHex: String,
        deviceIdHash: String,
        nonce: String,
    ): AttestationSignature {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val digest = Sha256.digest("$deviceIdHash|$nonce".encodeToByteArray())
        return AttestationSignature(
            signatureHex = Hex.encode(wallet.signer().signDigest(digest)),
            devicePubKeyHex = Hex.encode(wallet.publicKey),
        )
    }

    // ---------------------------------------------------------------- 交易构造

    fun signSend(
        privateKeyHex: String,
        chainId: String,
        toAddress: String,
        amountUmc: Long,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
    ): SignedTxResult {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val message = CosmosMsgs.send(wallet.address, toAddress, Coin(DENOM, amountUmc.toString()))
        return sign(wallet, chainId, listOf(message), accountNumber, sequence, memo, GAS_SEND)
    }

    fun signRegisterDevice(
        privateKeyHex: String,
        chainId: String,
        model: String,
        os: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
    ): SignedTxResult {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val message = DepinTx.registerDevice(wallet.address, wallet.address, model, os)
        return sign(wallet, chainId, listOf(message), accountNumber, sequence, memo, GAS_REGISTER)
    }

    fun signRegisterNode(
        privateKeyHex: String,
        chainId: String,
        model: String,
        os: String,
        role: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
    ): SignedTxResult {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val message = PhonenodeTx.registerNode(wallet.address, wallet.address, model, os, role)
        return sign(wallet, chainId, listOf(message), accountNumber, sequence, memo, GAS_REGISTER)
    }

    /** 设备自签 attestation：signatureHex = secp256k1("deviceIdHash|nonce") 的 64 字节 r||s。 */
    fun signSubmitAttestation(
        privateKeyHex: String,
        chainId: String,
        rootHash: String,
        nonce: String,
        deviceIdHash: String,
        devicePubKeyHex: String,
        signatureHex: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
    ): SignedTxResult {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val message = PhonenodeTx.submitAttestation(
            creator = wallet.address,
            rootHash = rootHash,
            nonce = nonce,
            deviceIdHash = deviceIdHash,
            devicePubKeyHex = devicePubKeyHex,
            signatureHex = signatureHex,
        )
        return sign(wallet, chainId, listOf(message), accountNumber, sequence, memo, GAS_ATTEST)
    }

    /** depin 设备认证：challenge/signature 由认证预言机（Apple App Attest 校验后）签发。 */
    fun signAttestDevice(
        privateKeyHex: String,
        chainId: String,
        challenge: String,
        signatureBase64: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
    ): SignedTxResult {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val message = DepinTx.attestDevice(wallet.address, wallet.address, challenge, signatureBase64)
        return sign(wallet, chainId, listOf(message), accountNumber, sequence, memo, GAS_ATTEST)
    }

    fun signContribution(
        privateKeyHex: String,
        chainId: String,
        taskId: String,
        taskType: String,
        score: Int,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
    ): SignedTxResult {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val message = DepinTx.submitContribution(wallet.address, taskId, taskType, score.toString())
        return sign(wallet, chainId, listOf(message), accountNumber, sequence, memo, GAS_CONTRIB)
    }

    fun signEdgeAiResult(
        privateKeyHex: String,
        chainId: String,
        taskId: String,
        resultHash: String,
        attestationNonce: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
    ): SignedTxResult {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val message = EdgeAiTx.submitResult(wallet.address, taskId, resultHash, attestationNonce)
        return sign(wallet, chainId, listOf(message), accountNumber, sequence, memo, GAS_EDGEAI)
    }

    /**
     * 在线心跳：两叶树 root = SHA256(SHA256(leaf) || paddingHash)，index="0"，
     * proof = paddingHash hex。leaf 含高度/时间戳，保证每笔字节不同。
     */
    fun signHeartbeat(
        privateKeyHex: String,
        chainId: String,
        leaf: String,
        accountNumber: Long,
        sequence: Long,
        memo: String = "",
    ): SignedTxResult {
        val wallet = McWallet.fromPrivateKeyHex(privateKeyHex)
        val leafHash = Sha256.digest(leaf.encodeToByteArray())
        val paddingHash = Sha256.digest(HEARTBEAT_PADDING.encodeToByteArray())
        val root = Hex.encode(Sha256.digest(leafHash + paddingHash))
        val message = PhonenodeTx.submitStateProof(
            creator = wallet.address,
            root = root,
            leaf = leaf,
            index = "0",
            proof = Hex.encode(paddingHash),
        )
        return sign(wallet, chainId, listOf(message), accountNumber, sequence, memo, GAS_HEARTBEAT)
    }

    // ------------------------------------------------------------------ 内部

    private fun sign(
        wallet: McWallet,
        chainId: String,
        messages: List<ByteArray>,
        accountNumber: Long,
        sequence: Long,
        memo: String,
        gas: Long,
    ): SignedTxResult {
        val signed = DirectTxBuilder.build(
            input = DirectTxInput(
                messages = messages,
                chainId = chainId,
                accountNumber = accountNumber,
                sequence = sequence,
                fee = listOf(Coin(DENOM, DEFAULT_FEE_UMC)),
                gasLimit = gas,
                memo = memo,
            ),
            signer = wallet.signer(),
        )
        return SignedTxResult(
            txRawBase64 = Base64Encode.encode(signed.txRawBytes),
            txHashHex = signed.txHashHex,
            fromAddress = wallet.address,
            signDocSha256Hex = Hex.encode(signed.signDocSha256),
        )
    }
}
