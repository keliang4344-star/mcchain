package com.mcchain.miner.shared

import com.mcchain.miner.shared.crypto.Secp256k1Signer
import com.mcchain.miner.shared.crypto.Secp256k1Verifier
import com.mcchain.miner.shared.util.Sha256
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class Secp256k1VerifierTest {

    @Test
    fun verifiesSignerOutput() {
        val signer = Secp256k1Signer(ByteArray(32) { (it + 1).toByte() })
        val digest = Sha256.digest("mcchain-verifier-test".toByteArray())
        val signature = signer.signDigest(digest)

        assertTrue(Secp256k1Verifier.verify(signer.publicKeyCompressed(), digest, signature))
        // 换一个 digest 必须失败
        assertFalse(Secp256k1Verifier.verify(signer.publicKeyCompressed(), Sha256.digest("other".toByteArray()), signature))
    }

    @Test
    fun rejectsMalformedInputs() {
        val signer = Secp256k1Signer(ByteArray(32) { (it + 2).toByte() })
        val digest = Sha256.digest("x".toByteArray())
        val signature = signer.signDigest(digest)

        assertFalse(Secp256k1Verifier.verify(ByteArray(32), digest, signature))
        assertFalse(Secp256k1Verifier.verify(signer.publicKeyCompressed(), ByteArray(31), signature))
        assertFalse(Secp256k1Verifier.verify(signer.publicKeyCompressed(), digest, ByteArray(63)))
    }
}
