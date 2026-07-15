# T2 预言机框架（链侧可插拔 attestation 验证）

## 现状（2026-07-12）
- `x/depin/types/oracle.go` 定义了 `AttestationOracle` 接口与两种实现。
- `AttestDevice`（`x/depin/keeper/msg_server_attest_device.go`）已改为调用 `types.DefaultOracle.VerifyDeviceAttestation(...)`，不再硬编码软逻辑。
- 默认 `DefaultOracle = &SoftOracle{}`，行为与历史占位一致（challenge+signature 非空即通过），保证测试网/本地挖矿流程不变。
- 全量 `go build` / `go test` 通过，`REPEAT MINING OK 3/3`。

## 两种实现
| 实现 | 适用 | 逻辑 |
|---|---|---|
| `SoftOracle` | 开发/测试/测试网 | 仅校验 challenge、signature 均非空 |
| `TeeOracle` | 生产主网 | 校验 signature 为预言机私钥对 `(deviceAddr\|challenge)` 的 secp256k1 签名（链上验签） |

## 生产启用 TeeOracle（链侧就绪，一行切换）
在 app 初始化阶段（如 `app.go` 的 `NewMcchain` 内，模块装配后）注入：
```go
import "mcchain/x/depin/types"

// bz = 预言机账户的 33 字节压缩 secp256k1 公钥（由你掌控的预言机私钥对应）
types.SetOracle(types.NewTeeOracle(types.NewSecp256k1PubKey(bz)))
```
此后 `AttestDevice` 即走真实验签，伪造的自签名 challenge 会被拒绝（`ErrInvalidAttestation`）。

## 仍需你接入的部分（物理/SDK 限制，我无法代写）
1. **设备端 TEE 出证**：Android `Key Attestation` / iOS `DeviceCheck`，在手机端生成硬件背书凭证。
2. **链下预言机服务**：收到设备凭证后，用预言机私钥对 `(deviceAddr|challenge)` 签名，signature 随 `AttestDevice` 上链。
   - 链上验签约定消息格式：`deviceAddr + "|" + challenge`（见 `oracle.go` `TeeOracle.VerifyDeviceAttestation`）。
   - 预言机私钥请离线保管，公钥（33 字节压缩）注入 `SetOracle`。
3. 若先做"准生产"，可临时让预言机=你自己的一个普通账户，用手动签名脚本喂 signature 跑通闭环，再替换为真实 TEE 服务。

## 验证要点
- 启用 TeeOracle 后，未带正确预言机签名的 `AttestDevice` 必须返回 `code 1106`（invalid attestation）。
- 带正确签名的请求，设备 `Attested` 置真，后续贡献可正常发币。
