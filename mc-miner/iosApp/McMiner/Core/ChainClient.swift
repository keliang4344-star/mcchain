import Foundation

/// 链上 REST 客户端（Cosmos SDK v0.47 gRPC-gateway）。
///
/// 与 Android `RpcClient` 对齐的四条链路：
///  - 账户：`/cosmos/auth/v1beta1/accounts/{address}`（account_number / sequence）
///  - 余额：`/cosmos/bank/v1beta1/balances/{address}`（以链上为准，不做本地记账）
///  - 广播：`POST /cosmos/tx/v1beta1/txs`（BROADCAST_MODE_SYNC）
///  - 回执：`GET /cosmos/tx/v1beta1/txs/{hash}`
struct ChainClient {

    var baseURL: URL
    private let session: URLSession

    init(baseURL: URL, timeout: TimeInterval = 15) {
        self.baseURL = baseURL
        let config = URLSessionConfiguration.ephemeral
        config.timeoutIntervalForRequest = timeout
        config.timeoutIntervalForResource = timeout * 2
        self.session = URLSession(configuration: config)
    }

    struct AccountState {
        var accountNumber: Int64
        var sequence: Int64
    }

    struct BroadcastOutcome {
        var code: Int
        var txHash: String
        var rawLog: String
        var codespace: String
    }

    enum ChainClientError: LocalizedError {
        case http(Int, String)
        case badResponse(String)

        var errorDescription: String? {
            switch self {
            case .http(let code, let body): return "HTTP \(code): \(body.prefix(200))"
            case .badResponse(let detail): return "响应解析失败：\(detail)"
            }
        }
    }

    // MARK: - 查询

    func accountState(address: String) async throws -> AccountState {
        let json = try await getJSON("/cosmos/auth/v1beta1/accounts/\(address)")
        guard let account = json["account"] as? [String: Any] else {
            throw ChainClientError.badResponse("缺少 account 字段")
        }
        // v0.47 返回 base_account 嵌套；部分版本直接平铺
        let base = (account["base_account"] as? [String: Any]) ?? account
        let number = Self.int64(base["account_number"])
        let sequence = Self.int64(base["sequence"])
        return AccountState(accountNumber: number, sequence: sequence)
    }

    func balanceUmc(address: String) async throws -> Int64 {
        let json = try await getJSON("/cosmos/bank/v1beta1/balances/\(address)")
        let balances = json["balances"] as? [[String: Any]] ?? []
        let umc = balances.first { ($0["denom"] as? String) == "umc" }
        return Self.int64(umc?["amount"])
    }

    func latestHeight() async throws -> Int64 {
        let json = try await getJSON("/cosmos/base/tendermint/v1beta1/blocks/latest")
        let header = json["block"] as? [String: Any]
        return Self.int64(header?["header"].flatMap { ($0 as? [String: Any])?["height"] })
    }

    // MARK: - 广播与回执

    func broadcast(txBase64: String) async throws -> BroadcastOutcome {
        let body: [String: Any] = [
            "tx_bytes": txBase64,
            "mode": "BROADCAST_MODE_SYNC",
        ]
        let json = try await postJSON("/cosmos/tx/v1beta1/txs", body: body)
        guard let response = json["tx_response"] as? [String: Any] else {
            throw ChainClientError.badResponse("缺少 tx_response 字段")
        }
        return BroadcastOutcome(
            code: Int(Self.int64(response["code"])),
            txHash: (response["txhash"] as? String) ?? "",
            rawLog: (response["raw_log"] as? String) ?? "",
            codespace: (response["codespace"] as? String) ?? ""
        )
    }

    /// 回执是否上块。未索引时返回 nil（继续轮询）。
    func txIncluded(txHash: String) async throws -> Bool? {
        do {
            let json = try await getJSON("/cosmos/tx/v1beta1/txs/\(txHash)")
            guard let response = json["tx_response"] as? [String: Any] else { return nil }
            let height = Self.int64(response["height"])
            if height > 0 { return true }
            return nil
        } catch ChainClientError.http(let code, _) where code == 404 || code == 500 {
            // 节点尚未索引到该交易
            return nil
        }
    }

    // MARK: - HTTP

    private func getJSON(_ path: String) async throws -> [String: Any] {
        var request = URLRequest(url: baseURL.appendingPathComponent(path))
        request.httpMethod = "GET"
        return try await send(request)
    }

    private func postJSON(_ path: String, body: [String: Any]) async throws -> [String: Any] {
        var request = URLRequest(url: baseURL.appendingPathComponent(path))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: body)
        return try await send(request)
    }

    private func send(_ request: URLRequest) async throws -> [String: Any] {
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw ChainClientError.badResponse("非 HTTP 响应")
        }
        guard (200..<300).contains(http.statusCode) else {
            throw ChainClientError.http(http.statusCode, String(data: data, encoding: .utf8) ?? "")
        }
        guard let json = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw ChainClientError.badResponse("JSON 顶层不是对象")
        }
        return json
    }

    private static func int64(_ value: Any?) -> Int64 {
        if let s = value as? String { return Int64(s) ?? 0 }
        if let n = value as? NSNumber { return n.int64Value }
        if let i = value as? Int64 { return i }
        return 0
    }
}
