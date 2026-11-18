# MC 实现状态（STATUS）

> 本文件是**对外可见的实现状态唯一口径**。README、路线图与白皮书涉及「已实现/规划中」的表述，
> 与本文件冲突时以本文件为准。更新时机：主网状态、审计状态、验证人集合、重大能力上线。

最后更新：2026-11-18

## 1. 阶段

**Pre-Mainnet（主网启动前）**。主网 chain-id 已保留（`mcchain-mainnet-1`），
但多验证人主网、第三方审计、资产相关模块的正式启用尚未完成。

## 2. 链标识（唯一事实源：`internal/chainprofile`）

| 用途 | chain-id | 档位 |
|---|---|---|
| 主网 | `mcchain-mainnet-1` | production |
| 测试网 | `mcchain-testnet-1` | test |
| 本地/devnet | `mcchain-dev-1` | development |
| 本地 solo | `mcchain-dev-solo-1` | development |
| 本地/仿真 | `mcchain-local-1` / `mcchain-sim-1` | development |

未登记 chain-id 默认拒绝启动（本地临时链可显式 `MC_ALLOW_UNKNOWN_CHAIN_ID=1`）。
查询命令：`mcchaind profile <chain-id>`。

## 3. 已实现并可运行

- 八个原生模块（tokenomics / depin / phonenode / edgeai / referral / dex / liquidstaking / mcchain）。
- 固定总量 1e15 umc、零通胀；五池分配 55/15/12/13/5；DEX 0.30% 手续费（50% 销毁、50% LP）。
- 设备认证链上验签（TeeOracle）+ 链下 TEE 证明校验（`cmd/oracle`，证书链/nonce/撤销/有效期）。
- 推荐十代费率、预算配速、负债守恒；液态质押地址游标与资金守恒审计。
- IBC transfer + ICA host/controller（classic/localhost light client）。
- devnet 一键启链（`scripts/devnet.sh`）、E2E 冒烟、readiness 门禁、docs/secret 门禁。

## 4. 尚未完成（不要在对外材料中宣称已完成）

| 能力 | 现状 | 说明 |
|---|---|---|
| 手机运行全节点/参与共识 | **未实现** | 移动端是设备认证 + 交易客户端（WebView + CosmJS over RPC）；轻客户端验证在路线图 |
| CosmWasm 合约 | 条件编译 | 仅 CGO + libwasmvm 的 Linux 构建启用；默认构建不含 wasm |
| 多验证人主网 | 未启动 | 目标 ≥4 独立验证人 + TMKMS |
| 第三方安全审计 | 未开始 | 审计前不得启用资产相关模块 |
| 真实设备 TEE 出证 | 代码就绪、材料未接 | 需设备端 SDK 与厂商根证书注入 |
| HSM/KMS 签名器 | 未接入 | 生产 signer 当前要求持久化 keyring（禁止助记词常驻） |
| 主网创世多方签名 | 流程已文档化 | 需真实多方仪式与签名发布 |

## 5. 已知风险（按优先级）

1. **出块节奏必须显式锁定 4s**：`deploy/init.sh` 已写入并校验；若运维绕开脚本建链，
   必须在 `config.toml` 手动设置 `timeout_commit = "4s"`，否则 slashing/释放窗口口径全部偏移。
2. **gas 价格必须一致**：节点 `minimum-gas-prices` 与 oracle `ORACLE_GAS_PRICES` 必须匹配；
   主网默认建议 `0.0025umc`。
3. **RPC 暴露面**：验证人禁止把 26657/9090/1317 绑 `0.0.0.0`；对外服务必须经 TLS 网关 + 鉴权 + 限流。
4. **移动端密钥**：助记词仅允许存 EncryptedSharedPreferences（Keystore 保护）；
   加密不可用时应用必须拒绝写入（已改为 fail-closed），不得回退明文。
5. **审计缺口**：核心资金与共识路径尚无第三方审计结论；任何「安全」表述均指代码约束，不是审计背书。

## 6. Go/No-Go 参考

发布前必须满足：`bash scripts/mainnet_readiness.sh all --mainnet` 输出 `MAINNET GO`，
创世/制品/SBOM 可追溯，审计问题关闭，密钥与灾备演练通过。详见 `docs/MAINNET_GATING.md`。
