# T2 预言机框架（链侧可插拔 attestation 验证）

## 现状（2026-07-26，P0②/P0③ 收尾）
- `x/depin/types/oracle.go` 定义了 `AttestationOracle` 接口与两种实现（`SoftOracle` / `TeeOracle`）。
- `AttestDevice`（`x/depin/keeper/msg_server_attest_device.go`）调用 `types.DefaultOracle.VerifyDeviceAttestation(...)`。
- **生产强制（P0③）**：`app.go` 在 `NewMcchain` 模块装配后要求 `MC_ORACLE_PUBKEY` 必须设置，否则直接 `panic`，杜绝「默认 SoftOracle 静默放行任意 attestation」的隐患；仅本地开发可通过 `MC_ORACLE_ALLOW_SOFT=1` 显式退回 SoftOracle。
- **链下真验硬件（P0②）**：`internal/oraclesvc` 的 `/sign` 在签发 `deviceAddr|challenge` 前，必须先验证设备提交的真实硬件 attestation（多根：Play Integrity / 华为 / Android Key Attestation），fail-closed。

## 两种实现
| 实现 | 适用 | 逻辑 |
|---|---|---|
| `SoftOracle` | 仅本地开发/测试（`MC_ORACLE_ALLOW_SOFT=1`） | 仅校验 challenge、signature 均非空（来者不拒，不可用于生产） |
| `TeeOracle` | 生产主网（默认强制） | 校验 signature 为预言机私钥对 `(deviceAddr\|challenge)` 的 secp256k1 签名（链上验签） |

## 生产启用 TeeOracle（默认强制，无需改代码）
`app.go` 在初始化阶段读取 `MC_ORACLE_PUBKEY`（33 字节压缩 secp256k1 公钥的 base64）并 `SetOracle(NewTeeOracle(...))`：
```go
import "mcchain/x/depin/types"

// bz = 预言机账户的 33 字节压缩 secp256k1 公钥（由你掌控的预言机私钥对应）
types.SetOracle(types.NewTeeOracle(types.NewSecp256k1PubKey(bz)))
```
- 未设置 `MC_ORACLE_PUBKEY` 且未设 `MC_ORACLE_ALLOW_SOFT=1` → 节点启动即 panic（fail-fast）。
- 此后 `AttestDevice` 走真实验签，伪造的自签名 challenge 会被拒绝（`ErrInvalidAttestation`）。

## 链下预言机真验硬件（P0②）
`/sign` 请求须携带 `attestation` 字段（root + payload），由 `internal/oraclesvc` 的 `Registry` 按根分发验证：
- `play_integrity` / `huawei`：ES256 JWS，提取 `nonce`/`requestDetails.nonce`（base64 解码）必须 == `deviceAddr|challenge`。
- `android_key_attestation`：X.509 证书链校验到可信根（Google/Huawei 设备根），并提取 KeyDescription 扩展中的 challenge == `deviceAddr|challenge`。
- 验证失败 → `/sign` 返回 403，绝不签发（fail-closed）。
- 根公钥通过环境变量注入：`ORACLE_GOOGLE_PUBKEY[_FILE]` / `ORACLE_HW_PUBKEY[_FILE]` / `ORACLE_ANDROID_ROOTS_FILE`；`ORACLE_ACCEPT_ROOTS` 控制允许列表（默认全三根）。

## 仍需你接入的部分（物理/SDK 限制，我无法代写）
1. **设备端 TEE 出证**：Android `Key Attestation` / 华为 `HWAttestation` / Google Play Integrity，在手机端生成硬件背书凭证，并把 `deviceAddr|challenge` 作为 nonce/challenge 绑定进去。
2. **链下预言机服务部署**：注入对应根公钥（见上），用预言机私钥对 `(deviceAddr|challenge)` 签名；公钥（33 字节压缩）注入 `MC_ORACLE_PUBKEY` 启动验证人节点。

## 验证要点
- 启用 TeeOracle 后，未带正确预言机签名的 `AttestDevice` 必须返回 `code 1106`（invalid attestation）。
- 带正确签名的请求，设备 `Attested` 置真，后续贡献可正常发币。
- `/sign` 在未提交有效 attestation 时必须返回 403（P0②）。
