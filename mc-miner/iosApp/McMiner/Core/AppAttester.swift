import CryptoKit
import DeviceCheck
import Foundation

/// Apple App Attest 封装。
///
/// 与 Android Key Attestation 的差异：
///  - App Attest 证明的是「App 完整性 + 设备未被越狱/调试」，不是设备型号；
///  - Secure Enclave 只支持 P-256，App Attest 的硬件密钥无法用来签 secp256k1，
///    所以链上签名仍用钱包私钥（纯 Kotlin secp256k1），App Attest 只用于向预言机证明环境可信。
final class AppAttester {

    static let shared = AppAttester()

    private let service = DCAppAttestService.shared

    var isSupported: Bool { service.isSupported }

    enum AppAttestError: LocalizedError {
        case unsupported
        case keyGenerationFailed

        var errorDescription: String? {
            switch self {
            case .unsupported: return "当前设备不支持 App Attest（需要真机）"
            case .keyGenerationFailed: return "App Attest 密钥生成失败"
            }
        }
    }

    /// 对 challenge 生成 attestation 对象（base64 交给预言机，根标识 apple_app_attest）。
    func attest(challenge: Data) async throws -> Data {
        guard service.isSupported else { throw AppAttestError.unsupported }
        let keyId = try await service.generateKey()
        let clientDataHash = Data(SHA256.hash(data: challenge))
        return try await service.attestKey(keyId, clientDataHash: clientDataHash)
    }
}

/// 认证预言机客户端（与 Android `OracleClient` 同一契约）。
///
/// 注意：预言机目前实现 android_key_attestation 校验；接入 iOS 需要在服务端增加
/// `apple_app_attest` 校验分支（验证 x5c/attestation 链、nonce、AAGUID 与 counter）。
/// 客户端这里保持同一请求结构，方便服务端复用签名逻辑。
struct OracleClient {

    var baseURL: URL

    struct Signature: Decodable {
        let challenge: String
        let signature: String
    }

    enum OracleError: LocalizedError {
        case http(Int, String)
        case missingFields

        var errorDescription: String? {
            switch self {
            case .http(let code, let body): return "预言机 HTTP \(code)：\(body.prefix(200))"
            case .missingFields: return "预言机响应缺少 challenge/signature"
            }
        }
    }

    func requestSignature(
        root: String,
        payloadBase64: String,
        challengeHex: String,
        deviceAddress: String
    ) async throws -> Signature {
        let body: [String: Any] = [
            "root": root,
            "payload": payloadBase64,
            "challenge": challengeHex,
            "device_addr": deviceAddress,
        ]
        var request = URLRequest(url: baseURL.appendingPathComponent("sign"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: body)

        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw OracleError.missingFields }
        guard (200..<300).contains(http.statusCode) else {
            throw OracleError.http(http.statusCode, String(data: data, encoding: .utf8) ?? "")
        }
        return try JSONDecoder().decode(Signature.self, from: data)
    }
}
