package com.mcchain.miner.shared.util

/**
 * 标准 Base64（RFC 4648，带 `=` 填充）。
 *
 * commonMain 里不引入 kotlin.io.encoding 的实验性 API，也不用平台库：
 * 交易字节要 base64 后交给 REST `/cosmos/tx/v1beta1/txs`，两端必须逐字节一致。
 */
object Base64Encode {

    private const val ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

    fun encode(bytes: ByteArray): String {
        if (bytes.isEmpty()) return ""
        val out = StringBuilder((bytes.size + 2) / 3 * 4)
        var i = 0
        while (i + 2 < bytes.size) {
            val n = ((bytes[i].toInt() and 0xFF) shl 16) or
                ((bytes[i + 1].toInt() and 0xFF) shl 8) or
                (bytes[i + 2].toInt() and 0xFF)
            out.append(ALPHABET[(n ushr 18) and 0x3F])
                .append(ALPHABET[(n ushr 12) and 0x3F])
                .append(ALPHABET[(n ushr 6) and 0x3F])
                .append(ALPHABET[n and 0x3F])
            i += 3
        }
        when (bytes.size - i) {
            1 -> {
                val n = (bytes[i].toInt() and 0xFF) shl 16
                out.append(ALPHABET[(n ushr 18) and 0x3F])
                    .append(ALPHABET[(n ushr 12) and 0x3F])
                    .append("==")
            }
            2 -> {
                val n = ((bytes[i].toInt() and 0xFF) shl 16) or ((bytes[i + 1].toInt() and 0xFF) shl 8)
                out.append(ALPHABET[(n ushr 18) and 0x3F])
                    .append(ALPHABET[(n ushr 12) and 0x3F])
                    .append(ALPHABET[(n ushr 6) and 0x3F])
                    .append('=')
            }
        }
        return out.toString()
    }
}
