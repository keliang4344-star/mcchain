import Foundation
import McChainShared

/// 交易提交（串行 + sequence 自动重签 + 回执轮询）。
///
/// 关键时序（与 Android 端一致，见 Phase 1 记录）：
///  交易回执可见时，auth/bank 查询状态可能仍落后一个区块，直接用旧 sequence 重签会被 CheckTx 拒（sdk/32）。
///  因此这里在重试前先等待 4.5s（>1 个出块间隔 4s），重新拉取 account_number/sequence 后再签。
///  所有签名都由 shared 的 MobileSdk 完成，Swift 侧不接触私钥以外的密码学。
actor TxService {

    enum TxError: LocalizedError {
        case rejected(code: Int, codespace: String, rawLog: String)
        case receiptTimeout(String)
        case sequenceExhausted(String)

        var errorDescription: String? {
            switch self {
            case .rejected(let code, let codespace, let rawLog):
                return "交易被拒（\(codespace)/\(code)）：\(rawLog.prefix(300))"
            case .receiptTimeout(let hash):
                return "已广播但 30s 内未查到回执：\(hash)"
            case .sequenceExhausted(let detail):
                return "sequence 连续冲突，重试耗尽：\(detail)"
            }
        }
    }

    private let chain: ChainClient
    private let chainId: String
    private let maxAttempts = 3
    private let settleSeconds: UInt64 = 4_500_000_000

    init(chain: ChainClient, chainId: String) {
        self.chain = chain
        self.chainId = chainId
    }

    /// 拉取账户状态 → 构造并签名 → 广播 → 等回执。闭包参数为 (accountNumber, sequence)。
    func submit(
        address: String,
        build: @escaping (Int64, Int64) -> SignedTxResult
    ) async throws -> SignedTxResult {
        var lastDetail = ""
        for attempt in 0..<maxAttempts {
            let state = try await chain.accountState(address: address)
            let signed = build(state.accountNumber, state.sequence)
            let outcome = try await chain.broadcast(txBase64: signed.txRawBase64)

            if outcome.code == 0 {
                try await waitIncluded(outcome.txHash.isEmpty ? signed.txHashHex : outcome.txHash)
                return signed
            }

            lastDetail = "\(outcome.codespace)/\(outcome.code) \(outcome.rawLog.prefix(120))"
            if outcome.code == 32 || outcome.rawLog.contains("account sequence mismatch") {
                if attempt < maxAttempts - 1 {
                    try await Task.sleep(nanoseconds: settleSeconds)
                    continue
                }
                throw TxError.sequenceExhausted(lastDetail)
            }
            throw TxError.rejected(code: outcome.code, codespace: outcome.codespace, rawLog: outcome.rawLog)
        }
        throw TxError.sequenceExhausted(lastDetail)
    }

    /// 用私钥推导出的地址换取账户状态（供 UI 显示 sequence 变化）。
    func accountState(address: String) async throws -> ChainClient.AccountState {
        try await chain.accountState(address: address)
    }

    private func waitIncluded(_ hash: String) async throws {
        guard !hash.isEmpty else { return }
        for _ in 0..<20 {
            if try await chain.txIncluded(txHash: hash) == true { return }
            try await Task.sleep(nanoseconds: 1_500_000_000)
        }
        throw TxError.receiptTimeout(hash)
    }
}
