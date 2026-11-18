package com.mcchain.miner.shared.protocol

/**
 * 最小 protobuf 编码器，只实现 Cosmos Tx 所需的 wire type 0（varint）与 2（length-delimited）。
 *
 * 手写而不依赖 Wire 代码生成的原因：
 *  - Phase 0 只涉及 TxRaw / TxBody / AuthInfo / SignDoc / 4 个 depin 消息，字段固定，手写可逐字节审计；
 *  - 避免在移动端引入 protoc / Wire 代码生成链路，保证 commonMain 零第三方依赖。
 *
 * 与 gogoproto 保持一致的关键点：proto3 默认值（0 / 空串）不写字段。
 */
internal class ProtoWriter(initialCapacity: Int = 128) {
    private var buffer = ByteArray(initialCapacity)
    private var size = 0

    private fun ensureCapacity(extra: Int) {
        val required = size + extra
        if (required <= buffer.size) return
        var newSize = buffer.size
        while (newSize < required) newSize = newSize shl 1
        buffer = buffer.copyOf(newSize)
    }

    fun writeRaw(bytes: ByteArray) {
        ensureCapacity(bytes.size)
        bytes.copyInto(buffer, size)
        size += bytes.size
    }

    fun writeVarint(value: Long) {
        require(value >= 0) { "varint must be non-negative: $value" }
        var v = value
        ensureCapacity(10)
        while (true) {
            if ((v and 0x7FL.inv()) == 0L) {
                buffer[size++] = v.toByte()
                return
            }
            buffer[size++] = ((v and 0x7F) or 0x80).toByte()
            v = v ushr 7
        }
    }

    fun writeTag(fieldNumber: Int, wireType: Int) {
        require(fieldNumber > 0) { "field number must be positive: $fieldNumber" }
        writeVarint(((fieldNumber shl 3) or wireType).toLong())
    }

    fun writeVarintField(fieldNumber: Int, value: Long) {
        if (value == 0L) return
        writeTag(fieldNumber, WIRE_VARINT)
        writeVarint(value)
    }

    fun writeBytesField(fieldNumber: Int, value: ByteArray) {
        writeTag(fieldNumber, WIRE_LENGTH_DELIMITED)
        writeVarint(value.size.toLong())
        writeRaw(value)
    }

    fun writeStringField(fieldNumber: Int, value: String) {
        if (value.isEmpty()) return
        writeBytesField(fieldNumber, value.encodeToByteArray())
    }

    fun writeMessageField(fieldNumber: Int, value: ByteArray) = writeBytesField(fieldNumber, value)

    fun toByteArray(): ByteArray = buffer.copyOf(size)

    companion object {
        const val WIRE_VARINT = 0
        const val WIRE_LENGTH_DELIMITED = 2
    }
}
