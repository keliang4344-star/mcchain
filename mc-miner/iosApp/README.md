# iOS 壳工程（SwiftUI + Kotlin/Native）

Phase 5 的 iOS 侧实现。**Windows 上无法编译**（Kotlin/Native iOS、Xcode、App Attest 都依赖 macOS），
因此这里提供完整的源码、工程定义与构建脚本；macOS 上按下面三步即可出包。

## 结构

```
iosApp/
├── project.yml                  # XcodeGen 工程定义（生成 McMiner.xcodeproj）
├── scripts/build_shared.sh      # shared → McChainShared.xcframework
├── McMiner/
│   ├── App/McMinerApp.swift     # @main + BGTask 注册
│   ├── App/AppModel.swift       # 四条链路编排（钱包/认证/心跳/余额）
│   ├── Core/KeychainStore.swift # 私钥存 Keychain（ThisDeviceOnly）
│   ├── Core/ChainClient.swift   # REST：账户/余额/广播/回执
│   ├── Core/TxService.swift     # sequence 重签 + 回执轮询
│   ├── Core/AppAttester.swift   # App Attest + 认证预言机
│   ├── Core/BackgroundHeartbeat.swift
│   ├── UI/…                     # 节点 / 钱包 / 设置
│   └── Resources/Info.plist
└── Frameworks/                  # 构建产物（不入库）
```

## macOS 构建步骤

```bash
# 1. 生成 XCFramework（真机 arm64 + Apple Silicon 模拟器）
bash iosApp/scripts/build_shared.sh

# 2. 生成 Xcode 工程（brew install xcodegen）
cd iosApp && xcodegen generate

# 3. 打开并运行（Team / Bundle ID 在 Signing & Capabilities 里改）
open McMiner.xcodeproj
```

## 密码学边界（重要）

| 能力 | 实现位置 | 说明 |
| --- | --- | --- |
| 助记词 / BIP-39 / BIP-32 | `shared/commonMain/wallet` | 与 Android 同码，`m/44'/118'/0'/0/i` |
| secp256k1 签名 | `shared/commonMain/crypto/PureSecp256k1Signer` | 纯 Kotlin，RFC6979 + low-S，与 CLI golden 逐字节一致 |
| 地址（bech32） | `shared/commonMain/wallet/Bech32` | `mc1…` |
| 交易编码 | `shared/commonMain/sdk/MobileSdk` | Swift 只传 hex 参数，不摆弄 ByteArray |
| 私钥保管 | iOS Keychain | Secure Enclave 只支持 P-256，**不能**托管 secp256k1 私钥 |
| 环境证明 | App Attest | 证明 App/设备完整性，不参与链上签名 |

## 已知限制 / 待办

1. **后台心跳**：`BGAppRefreshTask` 由系统调度，无法保证 120s 周期。链上 100 块（≈400s）无心跳即吊销认证，
   因此长期熄屏会掉线；当前策略是「前台定时 + 开屏补发 + 后台尽力而为」，文案需如实说明。
2. **App Attest 预言机**：服务端需新增 `apple_app_attest` 校验分支（attestation 对象、nonce、AAGUID、counter）。
   未接入前 iOS 只能完成 phonenode 自签认证（`MsgSubmitAttestation`），depin `MsgAttestDevice` 会失败并提示。
3. **贡献计量**：iOS 无法读取 per-UID 带宽/存储，贡献只在真实执行 EdgeAI 任务后提交；
   任务路由需与链上 edgeai 查询接口对齐（当前壳工程未接入，避免伪造数据）。
4. **ATS**：`Info.plist` 允许明文 HTTP 仅用于开发网，上架前必须换 HTTPS 并删除该段。

## 商店合规要点

- 不能宣称「后台挖矿即可收益」：iOS 后台执行受限，App Review 2.5.2 禁止此类表述；
  文案应为「贡献算力/带宽获得奖励，需保持 App 在前台」。
- 加密出口合规：仅使用标准 secp256k1/SHA-2，提交 App Store 时按 `ITSAppUsesNonExemptEncryption=false`
  申报（标准算法且不包含专有加密）。
- 钱包涉及金融属性：需在审核备注中说明「非托管钱包，私钥仅存本机 Keychain，无托管/法币兑换」。
