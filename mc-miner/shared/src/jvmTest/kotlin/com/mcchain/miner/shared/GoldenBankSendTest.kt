package com.mcchain.miner.shared

import com.google.gson.JsonObject
import com.google.gson.JsonParser
import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.protocol.Coin
import com.mcchain.miner.shared.protocol.CosmosMsgs
import com.mcchain.miner.shared.tx.DirectTxBuilder
import com.mcchain.miner.shared.tx.DirectTxInput
import com.mcchain.miner.shared.util.Hex
import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * bank send 的 golden 对照：验证 MsgSend 编码与整笔 TxRaw 和 mcchaind 逐字节一致。
 */
class GoldenBankSendTest {

    @Test
    fun bankSendTxMatchesCliGoldenVector() {
        val vector = loadVector()
        val msg = vector.getAsJsonObject("msg")
        val amount = msg.getAsJsonObject("amount")
        val fee = vector.getAsJsonObject("fee")

        val input = DirectTxInput(
            messages = listOf(
                CosmosMsgs.send(
                    fromAddress = msg["fromAddress"].asString,
                    toAddress = msg["toAddress"].asString,
                    amount = Coin(amount["denom"].asString, amount["amount"].asString),
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
        assertEquals("signatureHex", vector["signatureHex"].asString, Hex.encode(signed.signature))
        assertEquals("txRawHex", vector["txRawHex"].asString, Hex.encode(signed.txRawBytes))
        assertEquals("txHashHex", vector["txHashHex"].asString, signed.txHashHex)
    }

    private fun loadVector(): JsonObject {
        val stream = javaClass.classLoader.getResourceAsStream("golden/bank_send.json")
            ?: error("缺少 golden vector：src/jvmTest/resources/golden/bank_send.json")
        val text = stream.use { it.readBytes().toString(Charsets.UTF_8) }
        return JsonParser.parseString(text).asJsonObject
    }
}
