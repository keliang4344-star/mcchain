# MobileChain 安全自查报告（B2+B3）

**状态**: 自审完成，待正式审计
**审计范围**: B2（phonenode slashing / attestation）+ B3（edgeai attested execution）+ app 接线
**日期**: 2026-07-11

---

## 1. 经济安全（B1 cap 不被破坏）

| 检查项 | 状态 | 说明 |
|--------|------|------|
| Slash 不调 MintCoins | ✅ | phonenode.SlashIfBad 只调 staking.Slash + Jail + 吊销 attestation + RecordSlash；无 mint 点 |
| minted_supply 不变 | ✅ | slashing.Slash 扣自 staking 质押，不增发；MintedSupplyInvariant 仍成立 |
| edgeai 不直接 mint | ✅ | edgeai 无 Minter，不持有 tokenomics keeper；reward 未走 depin（R4 P1 待接入） |
| cap 不可治理 | ✅ | TotalSupplyCap = 常量（x/tokenomics/types/keys.go），不在 params subspace |
| 池占比不可变 | ✅ | Team/Community/Ecosystem 分配比均为常量，不在治理白名单 |

## 2. 访问控制

| 检查项 | 状态 | 说明 |
|--------|------|------|
| SubmitAttestation 仅节点可调 | ✅ | 先 GetNode 校验注册状态 |
| SubmitResult 需 IsAttested | ✅ | edgeai msg_server_submit_result 第一行校验 phonenodeKeeper.IsAttested |
| depin 拨付双重闸口 | ✅ | HasNode AND IsAttested，缺一不可 |
| SlashIfBad 仅 keeper 内调 | ✅ | 非 Msg 接口，BeginBlock / 内部业务逻辑触发 |
| Msg 权限 | ✅ | 所有 Msg 均校验 creator bech32 + ValidateBasic |

## 3. 重放防护

| 检查项 | 状态 | 说明 |
|--------|------|------|
| Attestation nonce 防重放 | ✅ | NonceKey(addr, nonce) 在 KVStore 标记已用；重复返回 ErrNonceReused |
| 设备哈希绑定 | ✅ | DeviceHashKey(hash) → addr 反查；SybilDeviceBinding=true 时 1:1 绑定 |
| edgeai 结果防重复 | ✅ | resultKey(taskID, submitter) 唯一；HasResult 表级判重 |
| dispute 防重复 | ✅ | task.Status=disputed 检查 + disputeKey 唯一 |

## 4. 状态模型

| 检查项 | 状态 | 说明 |
|--------|------|------|
| Attestation 状态机 | ✅ | pending → valid → invalid/revoked；expiry 基于链上区块时间 |
| Task 状态机 | ✅ | open → assigned/done/disputed；转换受 keeper 检查 |
| Dispute 状态机 | ✅ | open → resolved（none/cheat/honest） |

## 5. Genesis 一致性

| 检查项 | 状态 | 说明 |
|--------|------|------|
| DefaultGenesis 有效 | ✅ | 所有模块 DefaultParams 内参数合法（Validate 通过） |
| InitGenesis 顺序 | ✅ | phonenode → depin → edgeai（依赖方向正确） |
| 导出/导入循环 | ✅ | ExportGenesis → InitGenesis 可复现（模块测试通过） |

## 6. 事件可观测

| 检查项 | 状态 | 说明 |
|--------|------|------|
| phonenode.Slash | ✅ | SlashIfBad Emit (address/reason/penalty_bps) |
| phonenode.Attestation | ✅ | SubmitAttestation Emit (address/nonce) |
| edgeai.* 事件 | ✅ | TaskCreated / ResultSubmitted / DisputeOpened 均 EmitEvent |

## 7. 已知不足（主网前须解决）

| 项目 | 风险 | 建议 |
|------|------|------|
| 真实 attestation oracle 未接入 | 链上仅存根/nonce，重验证链下缺失 → 伪造证明可能绕过 | B4/B5 接入 Play Integrity / Key Attestation 预言机 |
| 非验证人节点无币种 slash | edge/light 节点犯错仅吊销 attestation，无经济损失 | B4 考虑小额 escrow 或声誉扣分 |
| edgeai R4 payout 未接入 depin | ~~结果通过争议期后不拨付~~ ✅ **已解决（2026-07-12）**：新增 `depin.PayoutReward` + edgeai `PayoutKeeper` 依赖 + `BeginBlock` 乐观结算，结果过争议窗口即经 depin 生态池拨付（受 B1 cap 约束，不直接 mint） | 见 `x/edgeai/keeper/validate.go` |
| 争议仲裁者未定 | Dispute 开启后暂无链上自动裁决 | 🔶 **部分解决**：`BeginBlock` 在争议窗口结束后乐观默认诚实（honest）结案并拨付；完整仲裁者（质押验证人投票 / optimistic 重算）留待 B3.1 后续 | 待 B3.1 仲裁模块 |

## 8. 团队多签

当前 3-of-5 多签公钥使用固定种子占位（仅测试网安全）。主网前须替换为真实团队公钥（见 `x/tokenomics/types/keys.go` `teamPubKeyStrings`）。

---

*本报告随代码同步生成。正式审计前不视为安全背书。*
