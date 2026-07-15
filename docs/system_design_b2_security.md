# MobileChain B2 批次 · 安全（硬件 attestation 反女巫 / Slashing / 最小治理）· 增量架构设计 + 任务分解

**文档类型**：增量架构设计（基于 B1 已落地 `x/tokenomics` 与现有 `x/depin`/`x/phonenode`，仅描述变更）
**批次**：B2（安全）— 路线图「阶段四 经济模型与安全」安全部分
**作者**：高见远（Gao），Architect
**语言**：简体中文
**技术栈**：Cosmos SDK v0.47.3 + cometbft v0.37.1 + Ignite。**不写实现代码**；验收以本机 `ignite chain build` + `go test` + `mcchaind q ...` 为准。
**配套图**：`docs/b2_security_class-diagram.mermaid`、`docs/b2_security_sequence-diagram.mermaid`。

---

# Part A · 系统设计

## 1. 实现方案与框架选型

| 难点 | 方案 |
|------|------|
| 硬件 attestation 反女巫 | **扩展 `x/phonenode`**（不新建模块）新增 `Attestation` 状态：证明根哈希 + nonce + 设备标识哈希 + 过期时间 + 状态。链上仅校验根/nonce 不可重放；重验证在客户端/预言机侧。 |
| Slashing 落地 | 复用 cosmos `x/slashing` + `x/staking`。phonenode keeper 注入 `StakingKeeper`/`SlashingKeeper`，封装 `SlashIfBad(ctx, addr, reason, penaltyBps)` 内部调 `stakingKeeper.Slash` + `Jail`；BeginBlock 心跳超时触发离线 slash。 |
| 最小治理 | 复用 cosmos `gov v1` + `x/params` subspace（phonenode/slashing 可调参数已由 `app.go` 注册 `paramproposal`）。**`total_supply_cap` 不开放**。 |
| slash 不破 B1 cap（硬约束） | slash = 扣减质押/查封生态拨付/吊销 attestation，**绝不 MintCoins**；`minted_supply` 不变；生态池拨付被扣留时仅余额减少，B1 记账不变。 |

- **框架**：沿用 Cosmos SDK 标准模块模式（与 depin/phonenode/tokenomics 一致）。**无新第三方依赖**。
- **关键决策（推荐默认，不抛用户）**：attestation 扩展进 phonenode（避免新模块装配复杂度）；slash 经 staking 标准接口；depin 拨付前增 `phonenodeKeeper.IsAttested(addr)` 闸口（复用既有 depin→phonenode 依赖，仅扩接口）。

## 2. 文件列表及相对路径

### 2.1 修改 `x/phonenode`
| 文件 | 关键变更 |
|------|----------|
| `x/phonenode/types/params.go` | 新增 params：`AttestationRequired`(bool)、`AttestationValidity`(duration)、`SybilDeviceBinding`(bool)、`OfflineGraceBlocks`(int64)、`OfflineSlashBps`(uint32)、`ContribSlashBps`(uint32)、`AttestSlashBps`(uint32)；`ParamKeyTable`/`ParamSetPairs`/`Validate` 补全 |
| `x/phonenode/types/keys.go` | 新增 KV key：`AttestationKey`(prefix+addr)、`SlashRecordKey`(prefix+addr+idx) |
| `x/phonenode/types/attestation.go` | 新增 `Attestation` 结构（RootHash, Nonce, DeviceIdHash, Expiry, Status） |
| `x/phonenode/types/genesis.go` | `DefaultGenesis`/`Validate` 纳入 attestation 占位与 params |
| `x/phonenode/genesis.go` | `InitGenesis` 初始化 attestation 空状态 + params |
| `x/phonenode/keeper/keeper.go` | 注入 `StakingKeeper`/`SlashingKeeper`/`BankKeeper`；新增 `SetAttestation/GetAttestation/IsAttested/HasNode/SlashIfBad/RecordSlash` |
| `x/phonenode/keeper/attestation.go` | `SubmitAttestation` 校验逻辑（nonce 唯一、设备绑定、过期） |
| `x/phonenode/keeper/slash.go` | `SlashIfBad` 封装（staking.Slash + Jail + 吊销 attestation + 记 SlashRecord + EmitEvent） |
| `x/phonenode/keeper/heartbeat.go` | BeginBlock 离线检测：读 `NodeState.LastRoot` 时间 vs `OfflineGraceBlocks`，超时调 `SlashIfBad` |
| `x/phonenode/module.go` | `BeginBlock` 实现（调 heartbeat 检测）；`RegisterInvariants` 可选 |
| `x/phonenode/keeper/msg_server_submit_attestation.go` | `MsgSubmitAttestation` 处理 |
| `x/phonenode/keeper/query_attestation.go`、`query_slashes.go` | `q phonenode attestation <addr>`、`q phonenode slashes <addr>` |
| `x/phonenode/client/cli/query_attestation.go`、`query_slashes.go` | CLI 子命令 |
| `x/phonenode/types/tx.proto`、`tx.pb.go`(生成) | 新增 `MsgSubmitAttestation` |
| `x/phonenode/types/query.proto`、`query.pb.go`(生成) | 新增 `QueryAttestation`/`QuerySlashes` |
| `x/phonenode/expected_keepers`(无新) | phonenode 不需对外暴露新接口（自身持有依赖） |

### 2.2 修改 `x/depin`（拨付闸口）
| 文件 | 关键变更 |
|------|----------|
| `x/depin/types/expected_keepers.go` | `PhonenodeKeeper` 接口新增 `IsAttested(ctx, addr string) bool` |
| `x/depin/keeper/msg_server_submit_contribution.go` | 拨付前调 `phonenodeKeeper.IsAttested(deviceAddr)`，未 attest 直接 `ErrDeviceNotAttested`（与现有 `Attested` 闸口叠加） |

### 2.3 修改 `app/app.go`（**串行共享，务必精确**）
- `x/phonenode/keeper.NewKeeper` 签名新增 `stakingKeeper stakingkeeper.Keeper`、`slashingKeeper slashingkeeper.Keeper` 参数；`New()` 内 `app.PhonenodeKeeper = *phonenodemodulekeeper.NewKeeper(..., app.StakingKeeper, app.SlashingKeeper)`。
- **无需新增 ModuleBasics / store key / mm 列表 / InitGenesis 顺序**：phonenode 已是模块，subspace 已注册，slashing/staking hooks 已在 `app.StakingKeeper.SetHooks` 装配，`govRouter` 已注册 `paramproposal`。
- `config.yml`：**不变**（沿用 B1 决定，不新增币）。

## 3. 数据结构与接口（类图，见 `docs/b2_security_class-diagram.mermaid`）

**proto/结构要点**
```proto
// x/phonenode/types/tx.proto 新增
message MsgSubmitAttestation { string creator=1; string root_hash=2; string nonce=3; string device_id_hash=4; }
// x/phonenode/types/query.proto 新增
message QueryAttestationRequest { string address=1; }
message QueryAttestationResponse { Attestation attestation=1; }
message QuerySlashesRequest { string address=1; }
message QuerySlashesResponse { repeated SlashRecord records=1; }
message Attestation { string root_hash=1; string nonce=2; string device_id_hash=3; int64 expiry=4; string status=5; }
message SlashRecord { string address=1; string reason=2; uint32 penalty_bps=3; int64 time=4; }
```
**phonenode params（新增字段）**：`attestation_required`、`attestation_validity`、`sybil_device_binding`、`offline_grace_blocks`、`offline_slash_bps`、`contrib_slash_bps`、`attest_slash_bps`。
**核心方法**：`phonenode.Keeper.{SetAttestation,GetAttestation,IsAttested,HasNode,SlashIfBad,RecordSlash}`、`depin` 通过 `PhonenodeKeeper.IsAttested` 闸口、`tokenomics`（只读引用，不被 B2 改动）。

## 4. 程序调用流程（时序图，见 `docs/b2_security_sequence-diagram.mermaid`）

- **① 注册 + attestation**：`RegisterNode` → `SubmitAttestation`（校验 nonce 唯一、device_id_hash 未重复绑定）→ 写 `Attestation{valid}`。`IsAttested` 返回 true。
- **② BeginBlock 离线检测**：`phonenode.BeginBlock` 遍历节点，若 `now - LastRoot_time > OfflineGraceBlocks` → `SlashIfBad(reason=offline)` → `stakingKeeper.Slash`(扣自质押) + `Jail` + 吊销 attestation + `EmitEvent("phonenode.Slash")`。**无 MintCoins**。
- **③ depin 拨付闸口**：`SubmitContribution` 在现有 `st.Attested` 与 `phonenodeKeeper.HasNode` 之间新增 `phonenodeKeeper.IsAttested(deviceAddr)`，未 attest 拒付。
- **④ 最小治理**：`gov` 提交 `ParameterChangeProposal`（phonenode/slashing subspace）→ 投票通过 → `ParamStore` 生效 → `q phonenode params` 可见。`total_supply_cap` 不在可改列表。

## 5. 任务列表（有序、含依赖、优先级）
| Task | 名称 | 依赖 | 优先级 | 涉及文件 |
|------|------|------|--------|----------|
| **T1** | phonenode params + attestation 状态 + genesis 占位 | 无 | P0 | `types/params.go`、`types/keys.go`、`types/attestation.go`、`types/genesis.go`、`genesis.go` |
| **T2** | attestation 登记/校验 + `MsgSubmitAttestation` + CLI/查询 | T1 | P0 | `keeper/attestation.go`、`keeper/msg_server_submit_attestation.go`、`keeper/query_attestation.go`、`client/cli/*`、`tx.proto`/`query.proto` |
| **T3** | Slashing 接线（keeper 注入 Staking/Slashing + `SlashIfBad` + BeginBlock 离线检测） | T1 | P0 | `keeper/keeper.go`、`keeper/slash.go`、`keeper/heartbeat.go`、`module.go`、`app/app.go`（NewKeeper 注入） |
| **T4** | depin 拨付增 `IsAttested` 闸口 | T2,T3 | P0 | `depin/types/expected_keepers.go`、`depin/keeper/msg_server_submit_contribution.go` |
| **T5** | 最小治理（param change 提案 + 事件 + 文档） | T1-T4 | P1 | `app/app.go`(复核)、`docs/security.md`、EmitEvent in slash/attestation |
**P0 验收锚点**：未 attest 节点注册/领取被拒；离线超宽限触发 slash 事件且无新 mint（`minted_supply` 不变）；`q phonenode attestation/slashes` 可见；param change 提案可改安全参数（不含 cap）。

## 6. 依赖包列表
- `github.com/cosmos/cosmos-sdk/x/slashing` / `x/staking`（已有）；`x/gov` v1（已有）；`x/params`（已有）。**无需新增任何第三方依赖**。

## 7. 共享知识（跨文件约定，含与 B1 交互边界）
- **denom/单位**：同 B1（`umc`，`1 MC=1e6 umc`）。
- **slash 不破 cap（铁律）**：slash 一律走 `stakingKeeper.Slash`/`Jail`/吊销 attestation/扣留生态拨付，**绝不调用 `tokenomics.MintCoins`**；`minted_supply` 不受 slash 影响（B1 `tokenomics.invariant.MintedSupplyInvariant` 仍成立）。
- **attestation 状态机**：`pending → valid`（提交有效证明）→ `invalid`（过期/伪造/被 slash）。链上只存根哈希+nonce+device_id_hash+expiry，重验证链下。
- **设备绑定防女巫**：`device_id_hash` 1:1 绑定地址；与 B1 `MinSelfDelegation` 质押门槛形成双重防女巫。
- **phonenode↔depin 接口**：`PhonenodeKeeper` 现有 `HasNode`；B2 新增 `IsAttested`。depin 拨付同时校验 `st.Attested`（本模块）+ `phonenodeKeeper.IsAttested`（跨模块）。
- **离线宽限协调**：`OfflineGraceBlocks`（B2）须 ≥ B4 弱网心跳参数，避免弱网误 slash（B4 设计联动）。
- **治理白名单参数**：可改 = phonenode 安全参数 + slashing 参数；**不可改 = `total_supply_cap`、各池占比、团队 vesting 曲线（B1 常量/genesis）**。
- **事件约定**：`phonenode.Slash` / `phonenode.Attestation`（type 命名供 B5 订阅）。

## 8. 待明确事项（仅真正需后续定稿，占位并注明）
1. **真实 attestation 验证预言机/轻验证方案**：链上仅验根/nonce；重验证（如 Play Integrity/Key Attestation 链下校验）的具体预言机或客户端职责，列为 B4/B5 链下契约，本批不实现。
2. **非验证人节点（role=edge/light）的 slash 标的物**：若无自质押，slash 采用「吊销 attestation + Jail + 生态拨付扣留」而非 staking.Slash；阈值与是否引入小额 escrow 由实现定（推荐：无质押节点仅吊销+Jail，不罚币）。
3. **审计机构**：B6 审计清单占位，真实机构后续定。
4. **团队多签真实密钥**：沿用 B1 占位（主网前替换）。

> 其余均按推荐默认定稿，未向用户抛待确认。
