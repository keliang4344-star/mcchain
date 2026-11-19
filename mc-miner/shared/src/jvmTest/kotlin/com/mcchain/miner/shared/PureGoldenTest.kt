package com.mcchain.miner.shared

import com.google.gson.JsonParser
import com.mcchain.miner.shared.crypto.PureSecp256k1Signer
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosMsgs
import com.mcchain.miner.shared.protocol.DepinTx
import com.mcchain.miner.shared.tx.DirectTxBuilder
import com.mcchain.miner.shared.tx.DirectTxInput
import com.mcchain.miner.shared.util.Hex
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * 用 mcchaind 生成的 golden 向量裁决 PureSecp256k1Signer 的 RFC6979 实现
 * （register_device.json / bank_send.json 的 signatureHex 由链 CLI 产生，是最终权威）。
 */
class PureGoldenTest {

    @Test
    fun registerDeviceGoldenWithPureSigner() {
        val v = load("golden/register_device.json")
        val msg = v.getAsJsonObject("msg")
        val fee = v.getAsJsonObject("fee")
        val messages = listOf(
            DepinTx.registerDevice(
                creator = msg["creator"].asString,
                address = msg["address"].asString,
                model = msg["model"].asString,
                os = msg["os"].asString,
            ),
        )
        assertGolden(v, messages, fee)
    }

    @Test
    fun bankSendGoldenWithPureSigner() {
        val v = load("golden/bank_send.json")
        val msg = v.getAsJsonObject("msg")
        val amount = msg.getAsJsonObject("amount")
        val fee = v.getAsJsonObject("fee")
        val messages = listOf(
            CosmosMsgs.send(
                fromAddress = msg["fromAddress"].asString,
                toAddress = msg["toAddress"].asString,
                amount = Coin(amount["denom"].asString, amount["amount"].asString),
            ),
        )
        assertGolden(v, messages, fee)
    }

    private fun assertGolden(v: com.google.gson.JsonObject, messages: List<ByteArray>, fee: com.google.gson.JsonObject) {
        val input = DirectTxInput(
            messages = messages,
            chainId = v["chainId"].asString,
            accountNumber = v["accountNumber"].asLong,
            sequence = v["sequence"].asLong,
            fee = listOf(Coin(fee["denom"].asString, fee["amount"].asString)),
            gasLimit = fee["gasLimit"].asLong,
            memo = v["memo"].asString,
            timeoutHeight = v["timeoutHeight"].asLong,
        )
        val signed = DirectTxBuilder.build(input, PureSecp256k1Signer(Hex.decode(v["privateKeyHex"].asString)))
        assertEquals("publicKeyHex", v["publicKeyHex"].asString, Hex.encode(signed.publicKey))
        assertEquals("signatureHex", v["signatureHex"].asString, Hex.encode(signed.signature))
        assertEquals("txRawHex", v["txRawHex"].asString, Hex.encode(signed.txRawBytes))
        assertEquals("txHashHex", v["txHashHex"].asString, signed.txHashHex)
    }

    private fun load(name: String): com.google.gson.JsonObject {
        val stream = javaClass.classLoader.getResourceAsStream(name)
            ?: error("缺少 golden vector: $name")
        return JsonParser.parseString(stream.use { it.readBytes().toString(Charsets.UTF_8) }).asJsonObject
    }
}
