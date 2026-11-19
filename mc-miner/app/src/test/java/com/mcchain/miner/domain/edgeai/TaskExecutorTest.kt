package com.mcchain.miner.domain.edgeai

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Test

class TaskExecutorTest {

    @Test
    fun parseAcceptsWhitelistedHashChain() {
        val spec = TaskSpec.parse("""{"type":"hash_chain","seed":"abc","iters":100}""")
        assertEquals(TaskSpec.Spec.HashChain("abc", 100), spec)
    }

    @Test
    fun parseRejectsUnknownOrOutOfRange() {
        assertNull(TaskSpec.parse("""{"type":"tflite","model":"x"}"""))
        assertNull(TaskSpec.parse("""{"type":"hash_chain","seed":"","iters":10}"""))
        assertNull(TaskSpec.parse("""{"type":"hash_chain","seed":"a","iters":0}"""))
        assertNull(TaskSpec.parse("""{"type":"hash_chain","seed":"a","iters":${TaskSpec.MAX_ITERS + 1}}"""))
        assertNull(TaskSpec.parse("not json"))
        assertNull(TaskSpec.parse(""))
    }

    @Test
    fun hashChainIsDeterministicAndSensitive() {
        val a = TaskExecutor.hashChain("mc42", 1000)
        val b = TaskExecutor.hashChain("mc42", 1000)
        assertEquals("同输入必须同结果（可复算）", a, b)
        assertNotEquals(a, TaskExecutor.hashChain("mc43", 1000))
        assertNotEquals(a, TaskExecutor.hashChain("mc42", 999))
        assertEquals(64, a.length)
    }

    @Test
    fun executeRoutesThroughSpec() {
        val spec = TaskSpec.Spec.HashChain("seed-x", 5)
        assertEquals(TaskExecutor.hashChain("seed-x", 5), TaskExecutor.execute(spec))
    }
}
