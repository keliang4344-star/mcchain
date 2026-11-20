package com.mcchain.miner.domain.wallet

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * 链上错误分类回归：codespace 精确匹配优先，raw_log 兜底，未知错误原样返回。
 */
class TxErrorClassifierTest {

    @Test
    fun sequenceMismatchByCodespaceAndCode() {
        assertTrue(
            TxErrorClassifier.isSequenceMismatch(
                TxErrorClassifier.SDK_ERR_WRONG_SEQUENCE,
                TxErrorClassifier.CODESPACE_SDK,
                "",
            )
        )
        assertFalse(
            TxErrorClassifier.isSequenceMismatch(
                TxErrorClassifier.SDK_ERR_WRONG_SEQUENCE,
                TxErrorClassifier.CODESPACE_DEPIN,
                "",
            )
        )
    }

    @Test
    fun classifiesSequenceMismatch() {
        val message = TxErrorClassifier.classify(
            TxErrorClassifier.SDK_ERR_INVALID_SEQUENCE,
            TxErrorClassifier.CODESPACE_SDK,
            "account sequence mismatch",
        )
        assertTrue(message.contains("sequence mismatch"))
    }

    @Test
    fun classifiesInsufficientFeeAndFunds() {
        assertTrue(
            TxErrorClassifier.classify(
                TxErrorClassifier.SDK_ERR_INSUFFICIENT_FEE,
                TxErrorClassifier.CODESPACE_SDK,
                "",
            ).contains("手续费不足")
        )
        assertTrue(
            TxErrorClassifier.classify(
                TxErrorClassifier.SDK_ERR_INSUFFICIENT_FUNDS,
                TxErrorClassifier.CODESPACE_SDK,
                "",
            ).contains("余额不足")
        )
    }

    @Test
    fun duplicateTaskIsIdempotent() {
        val byCode = TxErrorClassifier.classify(
            TxErrorClassifier.DEPIN_ERR_TASK_EXISTS,
            TxErrorClassifier.CODESPACE_DEPIN,
            "",
        )
        val byLog = TxErrorClassifier.classify(
            0,
            "",
            "contribution task id already exists",
        )
        assertTrue(byCode.contains("幂等收敛"))
        assertTrue(byLog.contains("幂等收敛"))
    }

    @Test
    fun deviceAlreadyRegisteredIsReadable() {
        val message = TxErrorClassifier.classify(
            TxErrorClassifier.DEPIN_ERR_DEVICE_EXISTS,
            TxErrorClassifier.CODESPACE_DEPIN,
            "",
        )
        assertTrue(message.contains("设备已上链注册"))
    }

    @Test
    fun edgeAiErrorsAreMapped() {
        assertTrue(
            TxErrorClassifier.classify(
                TxErrorClassifier.EDGEAI_ERR_TASK_NOT_OPEN,
                TxErrorClassifier.CODESPACE_EDGEAI,
                "",
            ).contains("不再开放")
        )
        assertTrue(
            TxErrorClassifier.classify(
                TxErrorClassifier.EDGEAI_ERR_NOT_ASSIGNED,
                TxErrorClassifier.CODESPACE_EDGEAI,
                "",
            ).contains("其他节点")
        )
    }

    @Test
    fun unknownInvalidAndBlankHaveFallbacks() {
        assertTrue(
            TxErrorClassifier.classify(0, "", "invalid request").startsWith("交易无效")
        )
        assertEquals(
            "广播失败，请稍后重试",
            TxErrorClassifier.classify(0, "", "   "),
        )
        assertEquals(
            "some raw log",
            TxErrorClassifier.classify(0, "", "some raw log"),
        )
    }

    @Test
    fun permanentRejectionCoversNonRetryableCases() {
        assertTrue(
            TxErrorClassifier.isPermanentRejection(
                TxErrorClassifier.SDK_ERR_INSUFFICIENT_FUNDS,
                TxErrorClassifier.CODESPACE_SDK,
                "",
            )
        )
        assertTrue(
            TxErrorClassifier.isPermanentRejection(0, "", "signature verification failed")
        )
        assertFalse(TxErrorClassifier.isPermanentRejection(0, "", "timeout"))
    }

    @Test
    fun alreadyKnownDetection() {
        assertTrue(TxErrorClassifier.isAlreadyKnown("tx already exists in cache"))
        assertTrue(TxErrorClassifier.isAlreadyKnown("ALREADY EXISTS"))
        assertFalse(TxErrorClassifier.isAlreadyKnown("internal error"))
    }
}
