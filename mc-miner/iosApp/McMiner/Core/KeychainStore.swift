import Foundation
import Security

/// 钱包私钥的 Keychain 存储。
///
/// 设计约束：
///  - Secure Enclave 只支持 P-256，无法托管 secp256k1 私钥，因此这里保存的是原始 32 字节私钥；
///  - 使用 `kSecAttrAccessibleWhenUnlockedThisDeviceOnly`：不随 iCloud 备份迁移，锁屏不可读；
///  - 私钥只在进程内交给 shared 的 PureSecp256k1Signer 签名，不落到 UserDefaults / 日志。
struct KeychainStore {

    private let service = "com.mcchain.miner.wallet"
    private let account = "wallet.privateKeyHex"

    enum KeychainError: LocalizedError {
        case osStatus(OSStatus)
        case invalidData

        var errorDescription: String? {
            switch self {
            case .osStatus(let status): return "Keychain 操作失败（OSStatus \(status)）"
            case .invalidData: return "Keychain 数据格式错误"
            }
        }
    }

    func savePrivateKeyHex(_ hex: String) throws {
        guard let data = hex.data(using: .utf8) else { throw KeychainError.invalidData }
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
        SecItemDelete(query as CFDictionary)
        var attributes = query
        attributes[kSecValueData as String] = data
        attributes[kSecAttrAccessible as String] = kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        let status = SecItemAdd(attributes as CFDictionary, nil)
        guard status == errSecSuccess else { throw KeychainError.osStatus(status) }
    }

    func loadPrivateKeyHex() -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var item: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess,
              let data = item as? Data,
              let hex = String(data: data, encoding: .utf8) else { return nil }
        return hex
    }

    func deletePrivateKey() {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
        SecItemDelete(query as CFDictionary)
    }
}
