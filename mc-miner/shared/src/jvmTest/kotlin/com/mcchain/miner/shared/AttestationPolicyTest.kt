package com.mcchain.miner.shared

import com.mcchain.miner.shared.util.AttestationPolicy
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AttestationPolicyTest {

    @Test
    fun validTokenBeforeExpiryPasses() {
        assertTrue(AttestationPolicy.isValid("token", 2_000L, 1_000L))
    }

    @Test
    fun expiryBoundaryIsExpired() {
        assertFalse(AttestationPolicy.isValid("token", 1_000L, 1_000L))
        assertTrue(AttestationPolicy.isExpired(1_000L, 1_000L))
    }

    @Test
    fun blankTokenIsInvalid() {
        assertFalse(AttestationPolicy.isValid(null, 2_000L, 1_000L))
        assertFalse(AttestationPolicy.isValid("", 2_000L, 1_000L))
        assertFalse(AttestationPolicy.isValid(" ", 2_000L, 1_000L))
    }

    @Test
    fun unexpiredIgnoresToken() {
        assertTrue(AttestationPolicy.isUnexpired(2_000L, 1_000L))
        assertFalse(AttestationPolicy.isUnexpired(1_000L, 1_000L))
    }

    @Test
    fun missingExpiryIsNotExpired() {
        assertFalse(AttestationPolicy.isExpired(0L, 1_000L))
        assertFalse(AttestationPolicy.isExpired(-1L, 1_000L))
    }
}
