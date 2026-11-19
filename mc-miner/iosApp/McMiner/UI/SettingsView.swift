import McChainShared
import SwiftUI

struct SettingsView: View {

    @EnvironmentObject private var model: AppModel

    var body: some View {
        NavigationStack {
            Form {
                Section("链连接") {
                    TextField("REST 节点（http://host:1317）", text: $model.restEndpoint)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    TextField("Chain ID", text: $model.chainId)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    TextField("认证预言机（http://host:8090）", text: $model.oracleEndpoint)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                }

                Section("说明") {
                    Text("iOS 不做 PoW、不跑全节点；余额只来自链上。")
                    Text("后台刷新由系统调度，长时间熄屏可能因心跳超时（100 块宽限）被吊销认证，建议前台挂机。")
                        .foregroundStyle(.secondary)
                }

                Section("版本") {
                    LabeledContent("App", value: Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "-")
                    LabeledContent("共享核心", value: MobileSdk.shared.VERSION)
                }
            }
            .navigationTitle("设置")
        }
    }
}
