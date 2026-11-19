package com.mcchain.miner.domain.node

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.security.keystore.StrongBoxUnavailableException
import android.util.Base64
import dagger.hilt.android.qualifiers.ApplicationContext
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.cert.X509Certificate
import java.security.spec.ECGenParameterSpec
import javax.inject.Inject
import javax.inject.Singleton

/** 设备不支持硬件级 Key Attestation（或生成密钥失败）。 */
class KeyAttestationException(message: String, cause: Throwable? = null) : Exception(message, cause)

/**
 * Android Key Attestation：在 AndroidKeyStore 中生成绑定 challenge 的 EC 密钥，
 * 导出的证书链可交给预言机验证「密钥确实由 TEE/StrongBox 持有且 challenge 未被篡改」。
 *
 * 与预言机 `internal/oraclesvc/attestation.go` 的 android_key_attestation 规则对齐：
 *  - 证书链 PEM（leaf 在前）；
 *  - KeyDescription 扩展（OID 1.3.6.1.4.1.11129.2.1.17）中的 challenge == "deviceAddr|challenge"；
 *  - 链必须能验证到设备厂商根（由预言机侧完成）。
 */
@Singleton
class AndroidKeyAttestation @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    enum class SecurityLevel { SOFTWARE, TEE, STRONGBOX }

    data class AttestedKey(
        val alias: String,
        val certificateChainPem: String,
        val securityLevel: SecurityLevel,
        val challenge: ByteArray,
    )

    /** 生成绑定 challenge 的密钥；StrongBox 可用时优先，失败自动回退 TEE。 */
    fun generate(alias: String, challenge: ByteArray): AttestedKey {
        val keyStore = try {
            KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }
        } catch (e: Exception) {
            throw KeyAttestationException("AndroidKeyStore 不可用：${e.message}", e)
        }

        val strongBoxAvailable = Build.VERSION.SDK_INT >= Build.VERSION_CODES.P &&
            context.packageManager.hasSystemFeature(PackageManager.FEATURE_STRONGBOX_KEYSTORE)

        var keyPair: java.security.KeyPair? = null
        if (strongBoxAvailable) {
            keyPair = try {
                generateKeyPair(keyStore, alias, challenge, strongBox = true)
            } catch (e: StrongBoxUnavailableException) {
                null
            } catch (e: Exception) {
                throw KeyAttestationException("StrongBox 生成失败：${e.message}", e)
            }
        }

        if (keyPair == null) {
            keyPair = try {
                generateKeyPair(keyStore, alias, challenge, strongBox = false)
            } catch (e: Exception) {
                throw KeyAttestationException("TEE 密钥生成失败：${e.message}", e)
            }
        }

        val chain = (keyStore.getCertificateChain(alias) ?: emptyArray())
            .mapNotNull { it as? X509Certificate }
        if (chain.isEmpty()) {
            throw KeyAttestationException("证书链为空（设备可能不支持 Key Attestation）")
        }

        val description = parseKeyDescription(chain.first())
        if (!description.challenge.contentEquals(challenge)) {
            throw KeyAttestationException("证书中的 challenge 与请求不一致，拒绝使用该证明")
        }

        return AttestedKey(
            alias = alias,
            certificateChainPem = toPem(chain),
            securityLevel = description.securityLevel,
            challenge = description.challenge,
        )
    }

    fun deleteKey(alias: String) {
        runCatching {
            KeyStore.getInstance(ANDROID_KEYSTORE).apply { load(null) }.deleteEntry(alias)
        }
    }

    private fun generateKeyPair(
        keyStore: KeyStore,
        alias: String,
        challenge: ByteArray,
        strongBox: Boolean,
    ): java.security.KeyPair {
        // 同一 alias 的旧密钥绑定了旧 challenge，必须删除后重新生成。
        if (keyStore.containsAlias(alias)) keyStore.deleteEntry(alias)

        val builder = KeyGenParameterSpec.Builder(
            alias,
            KeyProperties.PURPOSE_SIGN or KeyProperties.PURPOSE_VERIFY,
        )
            .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
            .setDigests(KeyProperties.DIGEST_SHA256)
            .setAttestationChallenge(challenge)
            .setUserAuthenticationRequired(false)
        if (strongBox) builder.setIsStrongBoxBacked(true)

        val generator = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, ANDROID_KEYSTORE)
        generator.initialize(builder.build())
        return generator.generateKeyPair()
    }

    private fun toPem(chain: List<X509Certificate>): String = chain.joinToString("\n") { cert ->
        "-----BEGIN CERTIFICATE-----\n" +
            Base64.encodeToString(cert.encoded, Base64.NO_WRAP) + "\n" +
            "-----END CERTIFICATE-----\n"
    }

    // ------------------------------------------------------------------
    // KeyDescription 解析（只需前 5 个字段）
    // KeyDescription ::= SEQUENCE {
    //   attestationVersion INTEGER,
    //   attestationSecurityLevel SecurityLevel,  -- ENUMERATED
    //   keymasterVersion INTEGER,
    //   keymasterSecurityLevel SecurityLevel,    -- ENUMERATED
    //   attestationChallenge OCTET STRING, ...
    // }
    // ------------------------------------------------------------------

    private data class KeyDescription(val securityLevel: SecurityLevel, val challenge: ByteArray)

    private fun parseKeyDescription(cert: X509Certificate): KeyDescription {
        val extension = cert.getExtensionValue(KEY_DESCRIPTION_OID)
            ?: throw KeyAttestationException("证书缺少 KeyDescription 扩展")
        // getExtensionValue 返回 OCTET STRING( OCTET STRING( SEQUENCE... ) )
        val outer = DerCursor(extension)
        val inner = outer.readTlv(0x04)
        val sequence = DerCursor(DerCursor(inner).readTlv(0x30))

        sequence.readTlv(0x02)                  // attestationVersion
        val attestationLevel = sequence.readTlv(0x0A)   // ENUMERATED
        sequence.readTlv(0x02)                  // keymasterVersion
        sequence.readTlv(0x0A)                  // keymasterSecurityLevel
        val challenge = sequence.readTlv(0x04)  // attestationChallenge

        return KeyDescription(securityLevel(attestationLevel), challenge)
    }

    private fun securityLevel(encoded: ByteArray): SecurityLevel = when (encoded.lastOrNull()?.toInt()) {
        2 -> SecurityLevel.STRONGBOX
        1 -> SecurityLevel.TEE
        else -> SecurityLevel.SOFTWARE
    }

    private class DerCursor(private val data: ByteArray) {
        private var pos = 0

        fun readTlv(expectedTag: Int): ByteArray {
            if (pos >= data.size) throw KeyAttestationException("KeyDescription 截断")
            val tag = data[pos++].toInt() and 0xFF
            if (tag != expectedTag) {
                throw KeyAttestationException(
                    "KeyDescription 标签不匹配：期望 0x${expectedTag.toString(16)}，实际 0x${tag.toString(16)}"
                )
            }
            val length = readLength()
            if (pos + length > data.size) throw KeyAttestationException("KeyDescription 长度越界")
            val value = data.copyOfRange(pos, pos + length)
            pos += length
            return value
        }

        private fun readLength(): Int {
            val first = data[pos++].toInt() and 0xFF
            if (first < 0x80) return first
            val bytes = first and 0x7F
            if (bytes > 3 || pos + bytes > data.size) throw KeyAttestationException("KeyDescription 长度编码非法")
            var length = 0
            repeat(bytes) { length = (length shl 8) or (data[pos++].toInt() and 0xFF) }
            return length
        }
    }

    private companion object {
        const val ANDROID_KEYSTORE = "AndroidKeyStore"
        const val KEY_DESCRIPTION_OID = "1.3.6.1.4.1.11129.2.1.17"
    }
}
