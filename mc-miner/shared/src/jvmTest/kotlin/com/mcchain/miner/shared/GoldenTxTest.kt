package com.mcchain.miner.shared

import com.google.gson.JsonObject
import com.google.gson.JsonParser
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.DepinTx
import com.mcchain.miner.shared.tx.DirectTxBuilder
import com.mcchain.miner.shared.tx.DirectTxInput
import com.mcchain.miner.shared.util.Hex
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * Phase 0 验收：shared 构造的注册设备交易必须与 mcchaind 离线签名结果逐字节一致。
 *
 * golden/register_device.json 由 tools/gen_golden_tx.ps1 调用链仓库的 mcchaind 生成，
 * 私钥是公开测试值，禁止在任何网络向对应地址转入资产。
 */
class GoldenTxTest {

    @Test
    fun registerDeviceTxMatchesCliGoldenVector() {
        val vector = loadVector()
        val msg = vector.getAsJsonObject("msg")
        val fee = vector.getAsJsonObject("fee")

        val input = DirectTxInput(
            messages = listOf(
                DepinTx.registerDevice(
                    creator = msg["creator"].asString,
                    address = msg["address"].asString,
                    model = msg["model"].asString,
                    os = msg["os"].asString,
                ),
            ),
            chainId = vector["chainId"].asString,
            accountNumber = vector["accountNumber"].asLong,
            sequence = vector["sequence"].asLong,
            fee = listOf(Coin(fee["denom"].asString, fee["amount"].asString)),
            gasLimit = fee["gasLimit"].asLong,
            memo = vector["memo"].asString,
            timeoutHeight = vector["timeoutHeight"].asLong,
        )

        val signer = Secp256k1Signer(Hex.decode(vector["privateKeyHex"].asString))
        val signed = DirectTxBuilder.build(input, signer)

        assertEquals("publicKeyHex", vector["publicKeyHex"].asString, Hex.encode(signed.publicKey))
        assertEquals("signDocHex", vector["signDocHex"].asString, Hex.encode(signed.signDocBytes))
        assertEquals("signDocSha256Hex", vector["signDocSha256Hex"].asString, Hex.encode(signed.signDocSha256))
        assertEquals("signatureHex", vector["signatureHex"].asString, Hex.encode(signed.signature))
        assertEquals("bodyBytesHex", vector["bodyBytesHex"].asString, Hex.encode(signed.bodyBytes))
        assertEquals("authInfoBytesHex", vector["authInfoBytesHex"].asString, Hex.encode(signed.authInfoBytes))
        assertEquals("txRawHex", vector["txRawHex"].asString, Hex.encode(signed.txRawBytes))
        assertEquals("txHashHex", vector["txHashHex"].asString, signed.txHashHex)
    }

    private fun loadVector(): JsonObject {
        val stream = javaClass.classLoader.getResourceAsStream("golden/register_device.json")
            ?: error("缺少 golden vector：src/jvmTest/resources/golden/register_device.json")
        val text = stream.use { it.readBytes().toString(Charsets.UTF_8) }
        return JsonParser.parseString(text).asJsonObject
    }
}
