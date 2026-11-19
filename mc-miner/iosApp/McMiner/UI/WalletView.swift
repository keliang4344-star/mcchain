import SwiftUI

struct WalletView: View {

    @EnvironmentObject private var model: AppModel
    @State private var mnemonicInput = ""
    @State private var newlyCreatedMnemonic: String?
    @State private var sendTo = ""
    @State private var sendAmount = ""
    @State private var showDeleteConfirm = false

    var body: some View {
        NavigationStack {
            Form {
                if model.hasWallet {
                    Section("当前钱包") {
                        Text(model.address).font(.footnote.monospaced()).textSelection(.enabled)
                        Text(model.publicKeyHex.prefix(20) + "…")
                            .font(.caption.monospaced())
                            .foregroundStyle(.secondary)
                        Button("删除本地钱包", role: .destructive) { showDeleteConfirm = true }
                    }

                    Section("转账") {
                        TextField("收款地址（mc1…）", text: $sendTo)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                        TextField("金额（MC）", text: $sendAmount)
                            .keyboardType(.decimalPad)
                        Button("发送") {
                            Task {
                                await model.send(to: sendTo, mc: Double(sendAmount) ?? 0)
                                sendTo = ""
                                sendAmount = ""
                            }
                        }
                        .disabled(sendTo.isEmpty || Double(sendAmount) == nil)
                    }
                } else {
                    Section("创建钱包") {
                        Button("生成新助记词（12 词）") {
                            newlyCreatedMnemonic = model.createWallet()
                        }
                    }

                    Section("导入钱包") {
                        TextEditor(text: $mnemonicInput)
                            .frame(minHeight: 88)
                            .font(.footnote.monospaced())
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                        Button("导入") {
                            if model.importWallet(mnemonic: mnemonicInput) {
                                mnemonicInput = ""
                            }
                        }
                        .disabled(mnemonicInput.isEmpty)
                    }
                }
            }
            .navigationTitle("钱包")
            .alert("请抄写助记词（只显示一次）", isPresented: Binding(
                get: { newlyCreatedMnemonic != nil },
                set: { if !$0 { newlyCreatedMnemonic = nil } }
            )) {
                Button("我已抄写") { newlyCreatedMnemonic = nil }
            } message: {
                Text(newlyCreatedMnemonic ?? "")
            }
            .confirmationDialog("确认删除本地钱包？", isPresented: $showDeleteConfirm) {
                Button("删除", role: .destructive) { model.deleteWallet() }
            } message: {
                Text("删除后只能通过助记词恢复。")
            }
        }
    }
}
