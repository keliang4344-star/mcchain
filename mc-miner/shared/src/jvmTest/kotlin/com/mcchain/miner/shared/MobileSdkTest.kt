package com.mcchain.miner.shared

import com.google.gson.JsonParser
import com.mcchain.miner.shared.crypto.Secp256k1Verifier
import com.mcchain.miner.shared.sdk.MobileSdk
import com.mcchain.miner.shared.util.Base64Encode
import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Sha256
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * MobileSdk（iOS 门面）的端到端字节校验：与 mcchaind 离线签名的 golden 交易逐字节对齐。
 */
class MobileSdkTest {

    private val golden by lazy {
        val text = javaClass.classLoader!!.getResource("golden/register_device.json")!!.readText()
        JsonParser.parseString(text).asJsonObject
    }

    @Test
    fun base64MatchesRfc4648() {
        assertEquals("", Base64Encode.encode(ByteArray(0)))
        assertEquals("Zg==", Base64Encode.encode("f".encodeToByteArray()))
        assertEquals("Zm8=", Base64Encode.encode("fo".encodeToByteArray()))
        assertEquals("Zm9v", Base64Encode.encode("foo".encodeToByteArray()))
        assertEquals("Zm9vYg==", Base64Encode.encode("foob".encodeToByteArray()))
        assertEquals("Zm9vYmE=", Base64Encode.encode("fooba".encodeToByteArray()))
        assertEquals("Zm9vYmFy", Base64Encode.encode("foobar".encodeToByteArray()))
    }

    @Test
    fun registerDeviceTxMatchesGoldenFromCli() {
        val msg = golden.getAsJsonObject("msg")
        val result = MobileSdk.signRegisterDevice(
            privateKeyHex = golden["privateKeyHex"].asString,
            chainId = golden["chainId"].asString,
            model = msg["model"].asString,
            os = msg["os"].asString,
            accountNumber = golden["accountNumber"].asLong,
            sequence = golden["sequence"].asLong,
            memo = golden["memo"].asString,
        )
        assertEquals(golden["address"].asString, result.fromAddress)
        assertEquals(golden["txRawBase64"].asString, result.txRawBase64)
        assertEquals(golden["txHashHex"].asString, result.txHashHex)
        assertEquals(golden["signDocSha256Hex"].asString, result.signDocSha256Hex)
    }

    @Test
    fun selfAttestationSignatureVerifiesAgainstChainRule() {
        val privateKeyHex = golden["privateKeyHex"].asString
        val deviceIdHash = Hex.encode(Sha256.digest("ios-device-fixture".encodeToByteArray()))
        val nonce = "0123456789abcdef0123456789abcdef"
        val result = MobileSdk.signSelfAttestation(privateKeyHex, deviceIdHash, nonce)

        assertEquals(128, result.signatureHex.length)
        assertEquals(66, result.devicePubKeyHex.length)
        val digest = Sha256.digest("$deviceIdHash|$nonce".encodeToByteArray())
        assertEquals(
            true,
            Secp256k1Verifier.verify(
                Hex.decode(result.devicePubKeyHex),
                digest,
                Hex.decode(result.signatureHex),
            ),
        )
    }

    @Test
    fun walletFacadeMatchesGoldenAddress() {
        val info = MobileSdk.walletFromPrivateKey(golden["privateKeyHex"].asString, "mc")
        assertEquals(golden["address"].asString, info.address)
        assertEquals(golden["publicKeyHex"].asString, info.publicKeyHex)
        assertEquals(
            info.address,
            MobileSdk.addressFromPublicKey(info.publicKeyHex, "mc"),
        )
    }

    @Test
    fun mnemonicFacadeRoundTrip() {
        val mnemonic = MobileSdk.createMnemonic(Hex.encode(ByteArray(16)))
        assertEquals(
            "abandon abandon abandon abandon abandon abandon " +
                "abandon abandon abandon abandon abandon about",
            mnemonic,
        )
        val a = MobileSdk.walletFromMnemonic(mnemonic, 0, "mc")
        val b = MobileSdk.walletFromMnemonic(mnemonic, 1, "mc")
        assertEquals("m/44'/118'/0'/0/0", a.derivationPath)
        assertEquals(true, a.address.startsWith("mc1"))
        assertEquals(false, a.address == b.address)
    }
}
