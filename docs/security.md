# MobileChain B2 安全·验收文档

**批次**: B2（安全） — attestation 反女巫 / Slashing / 最小治理
**状态**: 已落地，可验收
**配套设计**: `docs/system_design_b2_security.md`
**验收日期**: 2026-07-11

---

## 1. 变更摘要

| 文件 | 变更 |
|------|------|
| `x/phonenode/types/params.go` | 新增 7 个安全参数（attestation_required / attestation_validity / sybil_device_binding / offline_grace_blocks / offline_slash_bps / contrib_slash_bps / attest_slash_bps）+ DefaultParams + Validate |
| `x/phonenode/types/keys.go` | 新增 AttestationKey / SlashRecordKey / DeviceHashKey / NonceKey 存储键 |
| `x/phonenode/types/attestation.go` | Attestation 状态机常量（pending/valid/invalid/revoked）+ NewValidAttestation + IsExpired |
| `x/phonenode/types/errors.go` | 新增 ErrAttestationRequired / ErrNonceReused / ErrDeviceAlreadyBound / ErrInvalidAttestation / ErrNotBondedValidator |
| `x/phonenode/types/expected_keepers.go` | 新增 StakingKeeper / SlashingKeeper 接口（依赖注入，避免模块耦合） |
| `x/phonenode/types/message_submit_attestation.go` | MsgSubmitAttestation 消息实现 |
| `x/phonenode/keeper/keeper.go` | Keeper 注入 StakingKeeper + SlashingKeeper |
| `x/phonenode/keeper/attestation.go` | SubmitAttestation（nonce 防重放、device_id_hash 防女巫）+ Set/Get/IsAttested + 设备绑定索引 |
| `x/phonenode/keeper/slash.go` | SlashIfBad（吊销 attestation + 记录 SlashRecord + staking.Slash/Jail 仅对 bonded 验证人）+ 事件 phonenode.Slash |
| `x/phonenode/keeper/heartbeat.go` | DetectOffline（BeginBlock 离线检测：按区块高度 LastProofBlock 判断心跳超时） |
| `x/phonenode/keeper/store.go` / `state.go` | NodeState 新增 LastProofBlock + SubmitStateProof 落地 |
| `x/phonenode/keeper/query_attestation.go` / `query_slashes.go` | gRPC Query Attestation / Slashes |
| `x/phonenode/module.go` | BeginBlock 实现（调 DetectOffline） |
| `x/phonenode/client/cli/*` | 新增 CLI: `tx submit-attestation` / `q attestation` / `q slashes` |
| `x/depin/types/expected_keepers.go` | PhonenodeKeeper 接口增 IsAttested |
| `x/depin/keeper/msg_server_submit_contribution.go` | 拨付前增 IsAttested 闸口（与 HasNode 叠加） |
| `app/app.go` | phonenode NewKeeper 注入 StakingKeeper + SlashingKeeper |

---

## 2. 安全验收锚点

### A. 未 attest 节点注册/拨付被拒
- **状态**: 已实现
- **机制**: `depin.SubmitContribution` 在拨付前校验 `phonenodeKeeper.IsAttested(addr)`；未 attest 返回 `ErrDeviceNotAttested`
- **验证**: `go test ./x/depin/...` 通过（mockPhonenodeKeeper 含 IsAttested）

### B. 离线超宽限触发 slash + 无新 mint
- **状态**: 已实现
- **机制**: `phonenode.BeginBlock → DetectOffline → SlashIfBad`。仅对已 attest 节点检测；离线判据为 `ctx.BlockHeight() - LastProofBlock > OfflineGraceBlocks`（区块高度，非秒级时间以避弱网抖动）
- **SlashIfBad 三步**:
  1. 吊销 attestation（status → revoked）
  2. 记录 SlashRecord（JSON 列表，`q phonenode slashes` 可见）
  3. 若为 bonded 验证人 → `stakingKeeper.Slash(...) + Jail`；非验证人仅吊销
- **硬约束**: `SlashIfBad` **绝不调用 `tokenomics.MintCoins`**；`minted_supply` 不变（B1 cap 不受影响）
- **事件**: `phonenode.Slash`（address/reason/penalty_bps）

### C. Attestation 查询
- **状态**: 已实现
- **CLI**: `mcchaind q phonenode attestation <addr>` / `mcchaind q phonenode slashes <addr>`
- **gRPC**: `QueryServer.Attestation` / `QueryServer.Slashes`

### D. 参数治理
- **状态**: subspace 已注册（`app.go` phonenode subspace + `govRouter paramproposal`）
- **可改**: phonenode 安全参数（attestation_required / validity / binding / grace_blocks / slash_bps）
- **不可改**: `total_supply_cap`、各池占比、团队 vesting 曲线（均为 genesis 常量/B1 记账）

---

## 3. Nonce 防重放

- 每次 `SubmitAttestation` 记录 `NonceKey(addr, nonce)` 到 KVStore
- 重复 nonce → `ErrNonceReused`
- Nonce 不过期；bech32 地址不含 "/"，用 "/" 作 key 分隔符安全

## 4. 设备绑定防女巫

- 若 `params.SybilDeviceBinding == true`，`device_id_hash` 1:1 绑定地址
- 通过 `DeviceHashKey(hash) → addr` 反查索引；冲突返回 `ErrDeviceAlreadyBound`

## 5. 离线宽限

- 默认 `OfflineGraceBlocks = 100`（≈100 区块）；与 B4 弱网协调后可能提高
- 从未提交过 state proof（`LastProofBlock == 0`）也视为离线

---

## 6. 待定（主网前）

1. 真实 attestation 验证（Play Integrity / Key Attestation）由客户端/预言机链下完成；链上仅校验根/nonce
2. 团队多签真实密钥：当前占位 5 个种子派生 pubkey；主网前替换为真实 3-of-5
3. 非验证人节点 slash 标的物：当前仅吊销 attestation + 记录，不罚币
4. 审计机构：待定

---

*本文档随 B2 代码同步生成，commit 时一并签入。*
