# 治理手册（GOVERNANCE）

> 这份文档回答三件事：怎么提一个治理提案、提案要走多久、什么提案**不能提**。

## 1. 治理参数（与代码同口径）

```text
最小存款        10,000,000 umc (10,000 MC)
存款期          172,800 s  ≈ 2 天
投票期          172,800 s  ≈ 2 天
quorum          33.4%
threshold       50%
veto_threshold       33.4%
burn_vote_quorum    false
burn_vote_veto      true
burn_proposal_deposit_prevote  false
```

如果想改这些参数，**本身就是一条治理提案**。

## 2. 提案生命周期

```
提交 (deposit 10k MC)
   |
   v
存活性不足 → 7 天后被 burn
   |
   v
进入投票期（quorum 33.4%、simple majority 50%、veto 33.4%）
   |
   v
通过 / 拒绝
   |
   v
通过则进入 Timelock 后执行
```

执行时的消息体由提案 author 预先写好，链只按消息体执行，不做内容审查。
所以**提案被通过 ≠ 提案是好的**——内容质量靠社区讨论。

## 3. 提案模板

```proposal.json
{
  "title": "<人类可读的标题，< 255 字节>",
  "description": "<Markdown 全文 / IPFS 链接。说明动机、影响、回滚路径>",
  "messages": [
    { "@type": "/cosmos.gov.v1.MsgExecLegacyContent", ... }
  ],
  "deposit": "10000000000umc"
}
```

完整 JSON 模板见 `docs/proposals/template.json`。

## 4. 提案提交流程（CLI）

```bash
# 1) 起草（用任何文本编辑器，把消息体写好）
cat > proposal.json <<EOF
{...}
EOF

# 2) 提交 + 押金
mcchaind tx gov submit-proposal proposal.json \
  --from <key> \
  --chain-id mcchain-mainnet-1 \
  --node https://rpc.mcchain.io:443 \
  --gas auto --gas-adjustment 1.5 -y

# 3) 投票（每持 1 MC = 1 票，spam 防护：持币时间加权未来再加）
mcchaind tx gov vote <proposal-id> yes|no|no_with_veto|abstain \
  --from <key> --chain-id mcchain-mainnet-1 --node ... -y

# 4) 查询进度
mcchaind query gov proposal <proposal-id> --node ...
mcchaind query gov tally <proposal-id> --node ...
```

## 5. 治理移交（mcchain.handover）

> 治理移交是 MC 公链特有的渐进治理路径，区别于普通 `MsgUpdateParams`：
> 在投票通过的「提案通过」与「实际执行新治理」之间强制留 48 小时时间锁窗口。

```text
提案 A：MsgInitiateHandover(new_governor=mcNEW...)
       ↓ 通过
时间锁 43200 块 = 48 小时（任何人都能查到、退出、提反对案）
       ↓
提案 B：MsgCompleteHandover()
       ↓ 通过
链从此认定 new_governor 为治理主体
```

- **提交主体**：A 和 B 都必须由 gov 模块账户触发，即只能由治理提案执行；
  普通账户物理上签不出来模块账户的私钥。
- **不可回退**：`Executed=true` 是终态。
- **白皮书口径**：「v1 阶段治理主体 = 链上治理模块账户」，与代码 `DefaultGovernorAddress()`
  一致。

## 6. 提案分类与典型消息类型

| 类别 | 消息类型 | 谁需要提 |
|---|---|---|
| 文本提案 | `MsgExecLegacyContent` | 纯建议，无需执行 |
| 参数变更 | `x/<module>/MsgUpdateParams` | 提案人提议，链上自动执行 |
| 客户端升级 | `x/upgrade.MsgSoftwareUpgrade` | 配合 hard-fork 高度 |
| 紧急暂停 | `x/crisis.MsgVerifyInvariant` | 仅在 invariant 失败时自动触发，不走治理 |
| 治理移交 | `x/mcchain.MsgInitiateHandover` / `MsgCompleteHandover` | 治理主体变更 |
| slash 解禁 | `x/slashing.MsgUnjail` | 自己签，**不是治理提案** |

## 7. 什么提案**不能**提（自动拒绝）

1. **削减他人质押**：`MsgSlashValidator` 不存在（slash 由共识自动触发）。
2. **绕过时间锁**：执行 MsgInitiateHandover 的提案若 ActivationHeight < 当前高度，自动拒绝。
3. **空投票**：voted no_with_veto 票数 > 33.4% → 提案被否且押金被烧。
4. **重复活跃提案**：同一 author 已有 deposit 未结清前不允许新提案。

## 8. 紧急提案快速通道

如果出现需要立即执行的紧急情况（例如发现关键 bug）：

1. 在社区论坛提前 24h 发布「紧急升级」预告。
2. 提案文本必须包含：
   - 触发原因（公开可验证的 PoC 或诊断日志）。
   - 二进制校验和。
   - 升级高度 H（建议 = 当前高度 + 14,400 = 16h 后，给社区反应时间）。
3. 治理参数里**没有**快速通道——紧急通道靠的是「社区快速共识」+ 多签主动投票。

## 9. 提案记录与复盘

- 通过的提案 7 天内归档到 `docs/proposals/archive/`。
- 失败的提案保留标题 + 投票分布，作为下次讨论基础。
- 每个治理周期（季度）做一次治理活跃度报告（参投率 / 通过率 / 拒绝原因）。