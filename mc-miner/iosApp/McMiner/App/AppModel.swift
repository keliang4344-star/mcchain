import CryptoKit
import Foundation
import McChainShared
import UIKit

/// 应用状态与四条链路编排（钱包 → 认证 → 贡献/心跳 → 余额）。
///
/// 所有交易构造/签名都走 `MobileSdk`（shared commonMain），Swift 只做：
/// 账户查询、广播、回执轮询、Keychain 存取与 UI 状态。
@MainActor
final class AppModel: ObservableObject {

    // MARK: - 持久化设置

    @Published var restEndpoint: String {
        didSet { UserDefaults.standard.set(restEndpoint, forKey: Keys.restEndpoint) }
    }
    @Published var oracleEndpoint: String {
        didSet { UserDefaults.standard.set(oracleEndpoint, forKey: Keys.oracleEndpoint) }
    }
    @Published var chainId: String {
        didSet { UserDefaults.standard.set(chainId, forKey: Keys.chainId) }
    }

    // MARK: - 链上状态

    @Published private(set) var address: String = ""
    @Published private(set) var publicKeyHex: String = ""
    @Published private(set) var balanceUmc: Int64 = 0
    @Published private(set) var accountNumber: Int64 = 0
    @Published private(set) var sequence: Int64 = 0
    @Published private(set) var nodeRegistered = false
    @Published private(set) var attestedAt: Date?
    @Published private(set) var busy = false
    @Published private(set) var log: [String] = []

    /// 助记词只在创建钱包时返回一次，UI 展示后即丢弃，不落盘。
    private var pendingMnemonic: String?
    private let keychain = KeychainStore()

    private enum Keys {
        static let restEndpoint = "mc.restEndpoint"
        static let oracleEndpoint = "mc.oracleEndpoint"
        static let chainId = "mc.chainId"
        static let nodeRegistered = "mc.nodeRegistered"
    }

    init() {
        restEndpoint = UserDefaults.standard.string(forKey: Keys.restEndpoint) ?? "http://127.0.0.1:1317"
        oracleEndpoint = UserDefaults.standard.string(forKey: Keys.oracleEndpoint) ?? "http://127.0.0.1:8090"
        chainId = UserDefaults.standard.string(forKey: Keys.chainId) ?? "mcchain-dev-1"
        nodeRegistered = UserDefaults.standard.bool(forKey: Keys.nodeRegistered)
        loadStoredWallet()
    }

    // MARK: - 钱包

    var hasWallet: Bool { !address.isEmpty }

    func loadStoredWallet() {
        guard let hex = keychain.loadPrivateKeyHex() else { return }
        applyWallet(privateKeyHex: hex)
    }

    /// 生成新钱包；返回的助记词需由 UI 让用户抄写，随后本地不再保存。
    func createWallet() -> String? {
        guard let entropy = randomEntropy(16) else { return nil }
        let mnemonic = MobileSdk.shared.createMnemonic(entropyHex: hexString(entropy))
        guard MobileSdk.shared.isValidMnemonic(mnemonic: mnemonic) else { return nil }
        let info = MobileSdk.shared.walletFromMnemonic(mnemonic: mnemonic, accountIndex: 0, bech32Prefix: "mc")
        try? keychain.savePrivateKeyHex(info.privateKeyHex)
        applyWallet(privateKeyHex: info.privateKeyHex)
        pendingMnemonic = mnemonic
        return mnemonic
    }

    func importWallet(mnemonic: String) -> Bool {
        let normalized = mnemonic.trimmingCharacters(in: .whitespacesAndNewlines)
        guard MobileSdk.shared.isValidMnemonic(mnemonic: normalized) else {
            append("助记词无效（词表或校验和错误）")
            return false
        }
        let info = MobileSdk.shared.walletFromMnemonic(
            mnemonic: normalized, accountIndex: 0, bech32Prefix: "mc"
        )
        try? keychain.savePrivateKeyHex(info.privateKeyHex)
        applyWallet(privateKeyHex: info.privateKeyHex)
        return true
    }

    func deleteWallet() {
        keychain.deletePrivateKey()
        address = ""
        publicKeyHex = ""
        balanceUmc = 0
        accountNumber = 0
        sequence = 0
        append("已删除本地钱包（请确认助记词已备份）")
    }

    // MARK: - 查询

    func refresh() async {
        guard hasWallet else { return }
        await perform("刷新余额/账户") {
            try await refreshAccount()
        }
    }

    // MARK: - 认证

    func registerDevice() async {
        await perform("注册设备（depin.MsgRegisterDevice）") {
            let result = try await self.txService().submit(address: self.address) { number, sequence in
                MobileSdk.shared.signRegisterDevice(
                    privateKeyHex: self.privateKeyHex(),
                    chainId: self.chainId,
                    model: UIDevice.current.model,
                    os: "iOS \(UIDevice.current.systemVersion)",
                    accountNumber: number,
                    sequence: sequence,
                    memo: "register-device"
                )
            }
            self.append("设备注册上链：\(result.txHashHex)")
        }
    }

    func registerNode() async {
        await perform("注册节点（phonenode.MsgRegisterNode）") {
            let result = try await self.txService().submit(address: self.address) { number, sequence in
                MobileSdk.shared.signRegisterNode(
                    privateKeyHex: self.privateKeyHex(),
                    chainId: self.chainId,
                    model: UIDevice.current.model,
                    os: "iOS \(UIDevice.current.systemVersion)",
                    role: "depin",
                    accountNumber: number,
                    sequence: sequence,
                    memo: "register-node"
                )
            }
            self.nodeRegistered = true
            UserDefaults.standard.set(true, forKey: Keys.nodeRegistered)
            self.append("节点注册上链：\(result.txHashHex)")
        }
    }

    /// 自签 attestation（phonenode.MsgSubmitAttestation）：
    /// deviceIdHash = SHA256(identifierForVendor)；签名 = 私钥签 SHA256("deviceIdHash|nonce")。
    func attestDevice() async {
        await perform("设备自签认证 + App Attest") {
            let deviceId = UIDevice.current.identifierForVendor?.uuidString ?? "unknown-ios-device"
            let deviceIdHash = Sha256Hex.digest(deviceId)
            let nonce = UUID().uuidString.replacingOccurrences(of: "-", with: "")
            let signature = MobileSdk.shared.signSelfAttestation(
                privateKeyHex: self.privateKeyHex(),
                deviceIdHash: deviceIdHash,
                nonce: nonce
            )
            let rootHash = "apple_app_attest|app_attest|\(deviceIdHash.prefix(16))"

            let result = try await self.txService().submit(address: self.address) { number, sequence in
                MobileSdk.shared.signSubmitAttestation(
                    privateKeyHex: self.privateKeyHex(),
                    chainId: self.chainId,
                    rootHash: rootHash,
                    nonce: nonce,
                    deviceIdHash: deviceIdHash,
                    devicePubKeyHex: signature.devicePubKeyHex,
                    signatureHex: signature.signatureHex,
                    accountNumber: number,
                    sequence: sequence,
                    memo: "attest-node"
                )
            }
            self.attestedAt = Date()
            self.append("自签认证上链：\(result.txHashHex)")

            // App Attest → 预言机签名 → depin.MsgAttestDevice（服务端需支持 apple_app_attest）
            do {
                let challenge = randomBytes(32)
                let attestation = try await AppAttester.shared.attest(challenge: challenge)
                let oracle = OracleClient(baseURL: URL(string: self.oracleEndpoint)!)
                let sig = try await oracle.requestSignature(
                    root: "apple_app_attest",
                    payloadBase64: attestation.base64EncodedString(),
                    challengeHex: hexString(challenge),
                    deviceAddress: self.address
                )
                let tx = try await self.txService().submit(address: self.address) { number, sequence in
                    MobileSdk.shared.signAttestDevice(
                        privateKeyHex: self.privateKeyHex(),
                        chainId: self.chainId,
                        challenge: sig.challenge,
                        signatureBase64: sig.signature,
                        accountNumber: number,
                        sequence: sequence,
                        memo: "attest-device"
                    )
                }
                self.append("预言机认证上链：\(tx.txHashHex)")
            } catch {
                self.append("App Attest/预言机链路未完成（节点仍处于自签认证）：\(error.localizedDescription)")
            }
        }
    }

    // MARK: - 心跳

    func heartbeat() async {
        await perform("在线心跳（MsgSubmitStateProof）") {
            let leaf = "\(self.address)|\(Int64(Date().timeIntervalSince1970))|\(UUID().uuidString)"
            let result = try await self.txService().submit(address: self.address) { number, sequence in
                MobileSdk.shared.signHeartbeat(
                    privateKeyHex: self.privateKeyHex(),
                    chainId: self.chainId,
                    leaf: leaf,
                    accountNumber: number,
                    sequence: sequence,
                    memo: ""
                )
            }
            self.append("心跳已上链：\(result.txHashHex)")
        }
    }

    // MARK: - 转账

    func send(to: String, mc: Double) async {
        let amount = Int64((mc * 1_000_000).rounded())
        guard amount > 0 else { append("转账金额必须大于 0"); return }
        await perform("转账 \(mc) MC → \(to)") {
            let result = try await self.txService().submit(address: self.address) { number, sequence in
                MobileSdk.shared.signSend(
                    privateKeyHex: self.privateKeyHex(),
                    chainId: self.chainId,
                    toAddress: to,
                    amountUmc: amount,
                    accountNumber: number,
                    sequence: sequence,
                    memo: ""
                )
            }
            self.append("转账成功：\(result.txHashHex)")
        }
    }

    // MARK: - 内部

    private func applyWallet(privateKeyHex: String) {
        let info = MobileSdk.shared.walletFromPrivateKey(privateKeyHex: privateKeyHex, bech32Prefix: "mc")
        address = info.address
        publicKeyHex = info.publicKeyHex
        append("钱包已就绪：\(info.address)")
    }

    private func privateKeyHex() -> String {
        keychain.loadPrivateKeyHex() ?? ""
    }

    private func txService() -> TxService {
        TxService(chain: ChainClient(baseURL: URL(string: restEndpoint)!), chainId: chainId)
    }

    private func refreshAccount() async throws {
        let chain = ChainClient(baseURL: URL(string: restEndpoint)!)
        let state = try await chain.accountState(address: address)
        accountNumber = state.accountNumber
        sequence = state.sequence
        balanceUmc = try await chain.balanceUmc(address: address)
        append("余额 \(Double(balanceUmc) / 1_000_000) MC，sequence=\(sequence)")
    }

    private func perform(_ title: String, _ work: @escaping () async throws -> Void) async {
        guard !busy else { return }
        guard hasWallet else { append("请先创建或导入钱包"); return }
        busy = true
        defer { busy = false }
        append("▶ \(title)")
        do {
            try await work()
        } catch {
            append("✗ \(title)失败：\(error.localizedDescription)")
        }
    }

    private func append(_ message: String) {
        log.insert("[\(Self.timeFormatter.string(from: Date()))] \(message)", at: 0)
        if log.count > 120 { log.removeLast(log.count - 120) }
    }

    private func randomBytes(_ count: Int) -> Data {
        var bytes = [UInt8](repeating: 0, count: count)
        _ = SecRandomCopyBytes(kSecRandomDefault, count, &bytes)
        return Data(bytes)
    }

    private func randomEntropy(_ count: Int) -> Data? {
        let data = randomBytes(count)
        return data.count == count ? data : nil
    }

    private func hexString(_ data: Data) -> String {
        data.map { String(format: "%02x", $0) }.joined()
    }

    private static let timeFormatter: DateFormatter = {
        let f = DateFormatter()
        f.dateFormat = "HH:mm:ss"
        return f
    }()
}

/// SHA-256 → hex（设备标识哈希，与 Android 的 `Hex.encode(Sha256.digest(...))` 对齐）。
enum Sha256Hex {
    static func digest(_ text: String) -> String {
        Data(SHA256.hash(data: Data(text.utf8))).map { String(format: "%02x", $0) }.joined()
    }
}
