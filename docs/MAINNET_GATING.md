# 主网门禁遗留项决策（Mainnet Gating Decisions）

> 本文档记录代码审查（P0/P1/P2 清单）中涉及**协议级行为变更**或**需要运营治理推进**
> 的遗留项：现状、已落地部分、剩余决策与建议推进路径。代码层已闭环的项不在此列。
> 决策主体：项目治理层（验证人集合 + 提案投票），代码侧仅提供可执行选项。

## 1. P1-08 · Oracle TEE 验签闭环（代码已 fail-closed，平台级 TEE 待治理接入）

**现状（已落地）**：

- `ORACLE_VERIFIER` 显式三态：`soft` / `tee` / `strict`；strict 下未接 TEE 时拒绝启动。
- 链上 `TeeOracle` 已接入：验证人注入 `MC_ORACLE_PUBKEY` 即按 TeeOracle 验签，
  生产链由 `internal/chainprofile.IsProduction` 闸门强制启用，禁止退化为 SoftOracle。
- `cmd/oracle` 提供 `/pubkey`、`/sign` 接口，签名私钥固定（重启 pubkey 不变）。

**剩余决策（治理项）**：

| 决策点 | 选项 | 建议 |
|---|---|---|
| 生产预言机 TEE 载体 | A. 云厂商 TEE（如 SGX/KMS 远程签名）；B. 设备端安全元件 + 证书链；C. 自建 HS | 主网启动采用 A（运维可托管），设备端证据校验走证书链（`ORACLE_ACCEPT_ROOTS`），二者可叠加 |
| 证据链信任根 | A. 单一厂商根；B. 多根并集；C. 治理可轮换 | 采用 B 起步、C 兜底：`ORACLE_ACCEPT_ROOTS` 由治理提案维护 |
| 预言机权限边界 | 预言机仅签链下真机验证结果，链上只验签 | 维持现状：链上不信任预言机状态，只信任其签名能力 |

**推进路径**：主网上线前提交一次治理提案，将「验证人必须运行经授权的 TEE 预言机、
证书根列表由治理维护」写入链上参数；部署模板见 `deploy/start.sh` 与 `docs/ORACLE_FRAMEWORK.md`。

## 2. P2-01 · Slash 历史分片存储（已落地，剩余为增强项）

**现状（已落地）**：

- `RecordSlash` 由「整数组读→append→整体重编码」改为**分片存储**：每条记录独立键
  `SlashRecordSeqKey(addr, seq)`，序号由 `SlashSeqKey(addr)` 单调递增（8 字节大端编码，
  键序即记录序）；`GetSlashes` 按前缀遍历并分页返回。O(N) 读写放大与无界膨胀已消除。
- slash 路径不调用 MintCoins，仅吊销 attestation + 记录 + （bonded 验证人）staking.Slash/Jail。

**剩余增强项（非阻塞，按需排期）**：

1. `GetSlashes` 查询结果补充累计 hash（顺序哈希链），供审计校验历史完整性。
2. 分页游标持久化，避免全量扫描（当前 N 为 slash 事件数，量级远小于状态总量，暂不优先）。

**决策结论**：代码层已闭环，增强项随审计需求排期，无需主网前治理。

## 3. 已决策为「不在此轮代码直改」的项

| 条目 | 决策 | 理由 |
|---|---|---|
| P2-04 Layer5 连续计数重置 | 保持现状，如实标注为确定性启发式 | 涉及反女巫行为定义，需运营数据验证后再定规则 |
| P2-07 部署 solo 脚本默认 chain-id | 保持主网默认 + 环境变量显式覆盖 | 部署已 config/env 驱动并接入 make_genesis 规范化 |

## 4. 治理推进节奏（时间线）

- **上线前**：完成第 1 节 TEE 治理提案；验证人运行 `scripts/mainnet_readiness.sh` 全绿。
- **上线后**：随运营数据评估 P2-04 规则调整；审计需求驱动 P2-01 增强项。
- **大版本窗口**：Cosmos SDK 0.47 → 0.50 LTS 迁移见 `docs/SDK_UPGRADE_PLAN.md`，与治理
  提案解耦，由工程排期独立推进。
