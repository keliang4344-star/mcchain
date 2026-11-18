package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.Hex
import com.mcchain.miner.shared.util.Sha256
import org.junit.Assert.assertEquals
import org.junit.Test

class Sha256Test {

    @Test
    fun emptyInput() {
        assertEquals(
            "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            Hex.encode(Sha256.digest(ByteArray(0))),
        )
    }

    @Test
    fun abc() {
        assertEquals(
            "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
            Hex.encode(Sha256.digest("abc".encodeToByteArray())),
        )
    }

    @Test
    fun paddingBoundaryAndLongInput() {
        // 长度 56 触发额外分组；再覆盖多分组输入。
        assertEquals(
            "cdc76e5c9914fb9281a1c7e284d73e67f1809a48a497200e046d39ccc7112cd0",
            Hex.encode(Sha256.digest(ByteArray(1_000_000) { 'a'.code.toByte() })),
        )
    }
}
