# MobileChain 主网部署方案（含资源评估）

> 文档类型：部署方案 + 资源评估
> 适用范围：将 MC 公链从测试网推进到主网真实部署
> 更新日期：2026-07-18（v4.0 新增模块：x/referral + Verifier 抽检 + 销毁机制 + DEX 手续费已就绪）
> 配套：Dockerfile / docker-compose.yml / deploy/init.sh / deploy/start.sh / scripts/make_genesis.py / scripts/mainnet-genesis-config.json / MAINNET_RUNBOOK.md

---

## 1. 验证人节点硬件最低要求

CometBFT 验证人对算力要求较低，单节点最低配置：

| 资源 | 最低要求 |
|---|---|
| CPU | 2 vCPU |
| 内存 | 4 GB |
| 磁盘 | 100 GB SSD |
| 网络 | 公网固定 IPv4，100 Mbps+ |
| OS | Ubuntu 22.04 LTS |

开发/测试环境可使用本地虚拟机或任意 Linux 环境进行单节点验证。主网必须使用独立 Linux 服务器，原因：
1. **操作系统**：Cosmos SDK 节点生产环境标准为 Linux。
2. **网络**：主网验证人需要公网固定 IP 以保证 p2p 连通性和 RPC 可访问性。
3. **可靠性**：生产环境需要冗余、监控、守护进程保障。
4. **安全**：验证人私钥应独立隔离，不与开发环境混用。

**结论**：开发环境仅作**开发 / 构建 / 单节点验证**用途；主网必须部署到独立 Linux 服务器。

---

## 2. 是否需要额外服务器

| 问题 | 结论 |
|---|---|
| 本地环境能否承载主网 | 否（OS + 网络 + 可靠性三重不满足） |
| 是否需采购/租赁服务器 | **是**，需 Linux 服务器（VPS 或裸金属） |

---

## 3. 服务器具体配置要求

### 3.1 单验证人（最小可上线，P0）

| 资源 | 最低 | 推荐 |
|---|---|---|
| vCPU | 2 | 4 |
| 内存 | 4 GB | 8 GB |
| 磁盘 | 100 GB SSD | 200 GB SSD（NVMe 更佳） |
| 带宽 | 100 Mbps 公网 | 500 Mbps 公网 |
| 公网 IP | 固定 IPv4 | 固定 IPv4 + IPv6 |
| OS | Ubuntu 22.04 LTS | Ubuntu 22.04 LTS |
| 数量 | 1（单点风险） | ≥3 验证人分散机房/云服务商 |

### 3.2 生产多验证人（推荐主网形态）

- **验证人节点 ×≥3**：4 vCPU / 8 GB / 200 GB SSD，固定公网 IP，跨可用区/跨云服务商部署避免相关故障。
- **全节点 / RPC 节点 ×2**：对外提供 RPC/API，4 vCPU / 8 GB / 300 GB SSD。
- **快照节点 ×1**：定期快照加速新节点同步，2 vCPU / 4 GB / 500 GB SSD。
- **监控**：Prometheus + Grafana + 告警（磁盘/出块停滞/签名 miss）；可选 COSMOS 专用 exporter。
- **密钥隔离**：验证人私钥存放于 HSM / 独立 signer 节点（tmkms / horcrux），不与公开 RPC 同机。

### 3.3 成本参考

按需询价，参考各云服务商官网。建议起步规模：3 台验证人 + 2 全节点，总成本远低于链停摆风险成本。

---

## 4. 完整部署方案（从构建到跑通）

### 阶段 A · 构建（CI 或本地 Linux）

```bash
# 任一 Linux 环境
git clone <私有仓> mcchain && cd mcchain
docker build -t mcchaind:mainnet .        # 产出 mcchaind 镜像
# 或直编
go build -o mcchaind ./cmd/mcchaind
```

### 阶段 B · 生产 genesis（关键，防超发/防漏账户）

```bash
mcchaind init validator --chain-id mcchain-mainnet-1 --home $HOME/.mcchain
# 1) 写入各账户（验证人/团队多签/生态/DePIN模块账户）
mcchaind add-genesis-account <addr> 100000000000umc ...
# 2) 用生成器规范化（denom=umc / DePIN池1e14 / 上限1e15 / chain_id）
python3 scripts/make_genesis.py \
  --genesis $HOME/.mcchain/config/genesis.json \
  --out $HOME/.mcchain/config/genesis.json \
  --config scripts/mainnet-genesis-config.json
mcchaind validate-genesis $HOME/.mcchain/config/genesis.json
```

> 多签地址须替换为真实 3-of-5 团队多签（T1 阻塞项，见第 6 节）。

### 阶段 C · 验证人创建 + 创世交易

```bash
mcchaind keys add validator --keyring-backend file   # 安全后端
mcchaind gentx validator 30000000000umc \
  --chain-id mcchain-mainnet-1 --home $HOME/.mcchain \
  --min-self-delegation 30000000000
mcchaind collect-gentxs --home $HOME/.mcchain
mcchaind validate-genesis $HOME/.mcchain/config/genesis.json
```

### 阶段 D · 部署运行（docker-compose 或 systemd）

```bash
# docker 方式（见 docker-compose.yml）
docker compose up -d validator
# 或裸机 systemd（见 MAINNET_RUNBOOK.md）
sudo systemctl enable --now mcchaind
```

- 出块间隔已固化 `timeout_commit = "4s"`（make_genesis 或 edit_configs 保证）。
- 监控：Prometheus 抓取 26660，Grafana 看板，出块停滞/签名 miss 告警。

### 阶段 E · 跑通验收（链上行为）

```bash
mcchaind status | jq '.SyncInfo.latest_block_height'   # 高度递增
mcchaind q tokenomics supply                            # 1e15 umc 上限一致
# 贡献即挖矿闭环
depin register-device <addr> pixel8 android
depin attest-device <addr> ch sig
phonenode register-node <addr> pixel8 android contributor
phonenode submit-attestation roothash nonce devicehash
depin submit-contribution t1 inference 80              # 余额 +400 umc
```

### 阶段 F · 多验证人扩展

- 各验证人按 B/C 重复，提交各自 gentx；任一节点 `collect-gentxs` 汇总后分发，全员用同一 genesis 启动即组成联盟主网。

---

## 5. 未完成任务清单（按优先级）

| 项 | 优先级 | 状态 | 阻塞/说明 |
|---|---|---|---|
| B6-R2 生产 genesis 生成器 | P0 | **已完成** | make_genesis.py + validate 通过 |
| B6-R3 docker + runbook | P0 | **已完成** | Dockerfile/compose/runbook/init/start |
| B6-R1 DAO 治理参数/路线 | P0 | 待拍板 | 参数默认值待确认（见 dao_roadmap.md） |
| T3 出块 4s 固化 | P1 | 已完成 | config 4s + 生成器固化 |
| B5-R1/R3-4 事件/SDK 契约 | P1 | 待执行 | event-subscriber 已有，补契约 |
| B6-R4 第三方审计清单 | P1 | 已出文档 | 必由第三方执行（audit_checklist.md） |
| T1 团队多签真实公钥 | P0 | **阻塞** | 待团队提供真实 3-of-5 多签地址 |
| T2 真实 attestation 预言机 | P0 | **阻塞** | 待设计预言机接入方案（手机挖矿信任核心） |
| B2 非验证人 slash 细则 | P2 | 已有 SlashIfBad | 补细则 |
| B3.1 争议仲裁者升级 | P2 | 待设计 | 争议窗口/仲裁人机制 |

---

## 6. 待团队决策的阻塞项

1. **T1 团队多签真实公钥**：genesis 中团队/金库账户须为真实 3-of-5 多签地址。需团队提供地址后替换 `scripts/mainnet-genesis-config.json` 与 `x/tokenomics/types/keys.go` 中的占位项。
2. **T2 真实 attestation 预言机**：当前 `attest-device` 为软认证（非空 challenge+signature 即通过）。主网需真实设备证明源（如手机 TEE 出证 + 预言机转发）。需由团队确定预言机形态（中心化签名服务 / 多预言机投票 / 第三方证明）。
3. **服务器采购**：按第 3 节配置租赁/购买 Linux 服务器，获取公网 IP 与 SSH 凭据用于部署（或按 runbook 自行部署）。

> 以上三项补齐前，链可"技术跑通"（单节点出块+挖矿验证），但不构成完整可信主网。
