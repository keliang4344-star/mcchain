package com.mcchain.miner.domain.node

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ContributionMeterTest {

    private fun meterAt(nowMs: Long) = ContributionMeter { nowMs }

    @Test
    fun epochIsDailyBucket() {
        val day = 24L * 60 * 60 * 1000
        assertEquals(0L, meterAt(0).currentEpoch())
        assertEquals(1L, meterAt(day).currentEpoch())
        assertEquals(20000L, meterAt(20000L * day + 123).currentEpoch())
    }

    @Test
    fun taskIdIsStableWithinEpochAndType() {
        val day = 24L * 60 * 60 * 1000
        val m = meterAt(100L * day)
        val id = m.taskId("phone_abc", m.currentEpoch(), "bandwidth")
        assertEquals("phone_abc|100|bandwidth", id)
        // 同 epoch 同类型再生成一次结果一致（幂等键）
        assertEquals(id, m.taskId("phone_abc", 100L, "bandwidth"))
    }

    @Test
    fun bandwidthScoreMapsBytesLinearlyAndClamps() {
        val per = ContributionMeter.BANDWIDTH_BYTES_PER_SCORE
        assertEquals(0, meterAt(0).bandwidthScore(0))
        assertEquals(0, meterAt(0).bandwidthScore(per - 1))
        assertEquals(1, meterAt(0).bandwidthScore(per))
        assertEquals(30, meterAt(0).bandwidthScore(30 * per)) // 恰达提交阈值
        assertEquals(100, meterAt(0).bandwidthScore(10 * per * 100)) // 封顶 100
        assertEquals(100, meterAt(0).bandwidthScore(Long.MAX_VALUE / 2))
    }

    @Test
    fun inferenceScoreByTaskCount() {
        assertEquals(0, meterAt(0).inferenceScore(9))
        assertEquals(1, meterAt(0).inferenceScore(10))
        assertEquals(100, meterAt(0).inferenceScore(10_000))
    }

    @Test
    fun chainTaskTypeMapping() {
        val m = meterAt(0)
        assertEquals("bandwidth", m.chainTaskType("BANDWIDTH"))
        assertEquals("inference", m.chainTaskType("AI_INFERENCE"))
        assertEquals("data_label", m.chainTaskType("DATA_LABELING"))
        // 链上不支持的类型必须返回 null，避免注定被 ErrUnsupportedType 拒绝的提交
        assertNull(m.chainTaskType("DATA_STORAGE"))
        assertNull(m.chainTaskType("COMPUTATION"))
        assertNull(m.chainTaskType("UNKNOWN"))
    }
}
