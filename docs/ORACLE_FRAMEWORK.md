# T2 预言机框架（链上验签 + 链下硬件证明）

## 现状（与实现一一对应）
- `x/depin/types/oracle.go` 定义 `AttestationOracle` 接口与两种实现：`SoftOracle` / `TeeOracle`。
- `AttestDevice`（`x/depin/keeper/msg_server_attest_device.go`）调用 `types.DefaultOracle.VerifyDeviceAttestation(...)`。
- **生产强制（双层）**：
  1. `app.New` 要求 `MC_ORACLE_PUBKEY` 必须设置，否则 panic；
  2. `SoftOracle.VerifyDeviceAttestation` 自身在 `chainprofile.IsProduction(chainID)` 时直接拒绝——
     即使运维误设 `MC_ORACLE_ALLOW_SOFT=1`，生产链也不会接受「非空即通过」的软认证。
- **信任边界（必须区分）**：链上 `TeeOracle` 只验证「预言机账户对 `deviceAddr|challenge` 的签名」，
  它证明的是预言机背书，**不是**硬件证明本身；证书链、nonce、过期、撤销、重放由链下 `cmd/oracle`
  的真实 verifier 完成（见下）。

## 支持的 verifier 模式（唯一权威表）

| `ORACLE_VERIFIER` | 适用环境 | 启动要求 | 行为 |
|---|---|---|---|
| `soft`（默认） | 仅开发/测试 | `ORACLE_SIGNER_MNEMONIC` | 仅校验 `SHA256(deviceID)==proof`，不构成抗女巫证据 |
| `tee` | 生产/测试网 | `ORACLE_TEE_ROOTS_FILE` 必填；strict 模式还会强制校验 signer 后端与撤销文件 | 校验设备证明包：证书链到本地可信根、设备/链/用途绑定、nonce 一次性、有效期/时钟偏差、撤销列表、安全等级 ≥ 阈值 |

strict 模式（`ORACLE_STRICT=1` 或 `ORACLE_ENV=production`）规则：
- `ORACLE_VERIFIER` 不得为 `soft`；`ORACLE_TEE_ROOTS_FILE` 缺失直接拒绝启动；
- 禁止 `ORACLE_SIGNER_MNEMONIC`（助记词不得作为常驻生产签名材料）；
- 禁止 `ORACLE_KEYRING_BACKEND=test|memory`，file 后端必须提供 `ORACLE_KEYRING_PASSWORD_FILE`；
- `ORACLE_TEE_REVOCATION_FILE` 配置后文件不存在会拒绝启动（杜绝对撤销列表的「以为在检查」）；
- `ORACLE_GAS_PRICES`（默认 `0.0025umc`）必须 **不低于** 节点 `minimum-gas-prices`，
  否则交易在 CheckTx 因手续费不足被拒（表现为「本地验证通过但链上提交失败」）。

## 硬件证明包格式（`attestation_proof`，base64(JSON)）

```json
{
  "device_id": "mc1...",
  "chain_id": "mcchain-mainnet-1",
  "purpose": "device_attestation",
  "nonce": "<一次性 challenge>",
  "expires_at": 1700000000,
  "security_level": "tee",
  "cert_chain_pem": ["-----BEGIN CERTIFICATE-----..."],
  "signature": "<leaf 私钥对规范化上下文的 ECDSA/RSA 签名，base64>"
}
```

规范化签名消息：`device_id \x00 chain_id \x00 purpose \x00 nonce \x00 expires_at \x00 security_level`。
根证书只来自本地 `ORACLE_TEE_ROOTS_FILE`，**绝不采信证明包自带的根**。

## 链上 TeeOracle 的启用（默认强制）
`app.go` 在初始化阶段读取 `MC_ORACLE_PUBKEY`（33 字节压缩 secp256k1 公钥的 base64）并 `SetOracle(NewTeeOracle(...))`：
```go
import "mcchain/x/depin/types"

// bz = 预言机账户的 33 字节压缩 secp256k1 公钥（由你掌控的预言机私钥对应）
types.SetOracle(types.NewTeeOracle(types.NewSecp256k1PubKey(bz)))
```
- 未设置 `MC_ORACLE_PUBKEY` 且未设 `MC_ORACLE_ALLOW_SOFT=1` → 节点启动即 panic（fail-fast）。
- 此后 `AttestDevice` 走真实验签，伪造的自签名 challenge 会被拒绝（`ErrInvalidAttestation`）。

## 链下预言机真验硬件
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
- `/sign` 在未提交有效 attestation 时必须返回 403。
