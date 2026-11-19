import SwiftUI

struct RootView: View {

    @EnvironmentObject private var model: AppModel

    var body: some View {
        TabView {
            DashboardView()
                .tabItem { Label("节点", systemImage: "antenna.radiowaves.left.and.right") }
            WalletView()
                .tabItem { Label("钱包", systemImage: "wallet.pass") }
            SettingsView()
                .tabItem { Label("设置", systemImage: "gearshape") }
        }
        .task { await model.refresh() }
    }
}

struct DashboardView: View {

    @EnvironmentObject private var model: AppModel

    var body: some View {
        NavigationStack {
            List {
                Section("账户") {
                    LabeledContent("地址", value: model.hasWallet ? model.address : "未创建钱包")
                    LabeledContent("余额", value: model.hasWallet ? mc(model.balanceUmc) : "-")
                    LabeledContent("sequence", value: model.hasWallet ? "\(model.sequence)" : "-")
                    LabeledContent("节点注册", value: model.nodeRegistered ? "已注册" : "未注册")
                    if let attestedAt = model.attestedAt {
                        LabeledContent("最近认证", value: attestedAt.formatted(date: .omitted, time: .standard))
                    }
                }

                Section("四条链路") {
                    Button("刷新余额/账户") { Task { await model.refresh() } }
                    Button("1. 注册设备") { Task { await model.registerDevice() } }
                    Button("2. 注册节点") { Task { await model.registerNode() } }
                    Button("3. 设备认证（自签 + App Attest）") { Task { await model.attestDevice() } }
                    Button("4. 发送在线心跳") { Task { await model.heartbeat() } }
                }
                .disabled(model.busy || !model.hasWallet)

                Section("日志") {
                    if model.log.isEmpty {
                        Text("暂无操作记录").foregroundStyle(.secondary)
                    }
                    ForEach(model.log, id: \.self) { line in
                        Text(line)
                            .font(.footnote.monospaced())
                            .textSelection(.enabled)
                    }
                }
            }
            .navigationTitle("MC Miner")
            .overlay {
                if model.busy { ProgressView().controlSize(.large) }
            }
        }
    }

    private func mc(_ umc: Int64) -> String {
        String(format: "%.6f MC", Double(umc) / 1_000_000)
    }
}
