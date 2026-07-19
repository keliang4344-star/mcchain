# MC 公链（MobileChain）主网部署 Runbook

> 目标：用云服务商新加坡服务器作为**创世验证人（genesis validator）**把 `mcchain` 网络拉起。
> 适用范围：Cosmos SDK v0.47.3 + Ignite 脚手架，二进制 `mcchaind`，代币 `umc`（1 MC = 1e6 umc，硬顶 1e15 umc = 10 亿 MC）。
>
> ⚠️ **前提声明**：单台服务器 ≠ 去中心化主网。它是 solo 网络（你占 100% 质押、0 容错）。
> 要成为承载真实价值的真主网，必须额外完成本文 §9（多验证人 + 代币 genesis 分配 + 安全架构）。
> 本 runbook 先把链跑起来，并标注每一步的主网化缺口。

---

## 0. 服务器准备（云服务商控制台）

| 项 | 建议值 | 说明 |
|----|--------|------|
| 地域 | 新加坡 | 已购 |
| vCPU / 内存 | ≥2 vCPU / ≥8 GB | 出块 + 状态机够用 |
| 系统盘 | ≥50 GB SSD（系统） | |
| 数据盘 | ≥100 GB SSD（挂载 `/data`，存 `~/.mcchain`） | 链数据持续增长 |
| 公网 IP | **弹性公网 IP**（EIP） | peer 连接稳定性 |
| 带宽 | ≥5 Mbps 按量/包月 | 出块广播 |
| 安全组 | 见 §6 | 只开必要端口 |

---

## 1. 获取 `mcchaind` 二进制

**方式 A：在服务器上编译（推荐，环境干净）**
```bash
# 需 Go >= 1.21
sudo apt-get update && sudo apt-get install -y build-essential git
# 安装 Go 1.21.x（略，按官方文档）
git clone <你的仓库地址> mcchain && cd mcchain
make build            # 或: go build -o mcchaind ./...
sudo mv mcchaind /usr/local/bin/
mcchaind version
```

**方式 B：从开发环境交叉编译**
```powershell
# 在项目根目录下，设置 Go 交叉编译环境
$env:GOOS="linux"; $env:GOARCH="amd64"
go build -o mcchaind ./...
# 然后 scp mcchaind 到服务器 /usr/local/bin/
```

---

## 2. 初始化节点

```bash
export CHAIN_ID=mcchain-mainnet-1
export MONIKER=<你的节点名，如 mc-sg-val-1>
export HOME_DIR=$HOME/.mcchain

mcchaind init $MONIKER --chain-id $CHAIN_ID --home $HOME_DIR
# 生成 $HOME_DIR/config/genesis.json / config.toml / app.toml
```

---

## 3. 创建验证人密钥

```bash
mcchaind keys add validator --keyring-backend file --home $HOME_DIR
# 记下地址（addr）和 pubkey，后续 gentx 与 genesis 账户用
mcchaind keys show validator --bech val -a --keyring-backend file --home $HOME_DIR
```

> 🔒 **主网化缺口**：本机 `file` keyring 把私钥存在服务器磁盘。真主网应改用
> **TMKMS / 独立签名机**（见 §9），不要把 validator 私钥长期裸放在出块机。

---

## 4. 配置创世（genesis）

### 4.1 代币 genesis 账户
```bash
# 给验证人地址预置初始质押金（示例 1000 MC = 1e9 umc）
mcchaind add-genesis-account $(mcchaind keys show validator -a --keyring-backend file --home $HOME_DIR) 200000000000umc --home $HOME_DIR
# 如有生态/团队/基金账户，继续 add-genesis-account
```

### 4.2 tokenomics 模块参数
编辑 `$HOME_DIR/config/genesis.json` 中 `tokenomics` 段：
- `total_supply_cap`: 固定 `1000000000000000`（1e15 umc = 10 亿 MC 硬顶）
- `allocations` / `release_schedule`: 按你的释放表填（团队锁仓、生态激励等）
> 注意：tokenomics 是**唯一 Minter**，通胀/释放由该模块控制，创世必须写死硬顶。

### 4.3 其他模块 genesis
`depin` / `phonenode` / `edgeai` 段保持 `DefaultGenesis` 即可（空状态起步）。

---

## 5. 生成创世交易（gentx）并收集

```bash
# 自抵押（示例 30k MC），commission 5%
mcchaind gentx validator 30000000000umc \
  --chain-id $CHAIN_ID \
  --moniker $MONIKER \
  --commission-rate "0.05" \
  --commission-max-rate "0.20" \
  --commission-max-change-rate "0.01" \
  --keyring-backend file \
  --home $HOME_DIR

mcchaind collect-gentxs --home $HOME_DIR
mcchaind validate-genesis --home $HOME_DIR   # 应输出 "Genesis ... valid"
```

---

## 6. 安全组 / 防火墙（云服务商）

| 端口 | 协议 | 来源 | 用途 |
|------|------|------|------|
| 26656 | TCP | 0.0.0.0/0 | P2P（其他验证人/全节点连入） |
| 26657 | TCP | 受限 IP / 内网 | RPC（**勿对公网全开**，防 DOS） |
| 1317  | TCP | 受限 IP / 内网 | LCD REST（前端/查询用） |
| 9090  | TCP | 内网 | gRPC |
| 22    | TCP | 你的办公 IP | SSH |

```bash
# 服务器本机 iptables（可选加固）
sudo ufw allow 26656/tcp
sudo ufw allow from <你的IP> to any port 26657,1317,22 proto tcp
sudo ufw enable
```

---

## 7. 以 systemd 常驻运行

`/etc/systemd/system/mcchain.service`：
```ini
[Unit]
Description=MC Chain Node
After=network.target

[Service]
User=ubuntu
ExecStart=/usr/local/bin/mcchaind start --home /home/ubuntu/.mcchain
Restart=always
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now mcchain
journalctl -u mcchain -f    # 看出块日志
```

---

## 8. 健康检查

```bash
# 本地 RPC 查状态（限内网/办公 IP）
mcchaind status --node tcp://localhost:26657
# 或
curl http://localhost:26657/status | jq '.result.sync_info'
# 看到 latest_block_height 递增 = 出块正常
```

---

## 9. 成为"真主网"必须补的项（单台之后）

1. **多验证人（≥4）**：让其他独立方各自 `init` + `gentx`，你把他们的 gentx 放进
   `~/.mcchain/config/gentxs/` 后 `collect-gentxs`。BFT 容错 = 容忍 ⌊(n-1)/3⌋ 故障。
2. **代币 genesis 分配透明化**：团队/生态/社区比例、锁仓与线性释放表写进 tokenomics 创世，
   并公开审计报告。单点 100% 质押不构成主网。
3. **验证人密钥隔离**：部署 **TMKMS**（或 HSM）在独立机签名，出块机只持热公钥；
   采用 **sentry 节点**拓扑（出块机藏在内网，sentry 对外扛 P2P）。
4. **Anti-DDoS**：云服务商开启 Anti-DDoS 基础防护/高防包，P2P 入口前置清洗。
5. **oracle `/sign` 生产加固（P0 安全）**：代码已实现 `ORACLE_SIGN_TOKEN`（Bearer 认证）+ `ORACLE_RATE_LIMIT`（限流）+ `ORACLE_TLS_CERT/KEY`（HTTPS 可选）。
   主网部署**必须**设 `ORACLE_SIGN_TOKEN` 且将服务置于 TLS 反向代理之后（详见 `internal/oraclesvc/service.go` 与 `docs/ORACLE_FRAMEWORK.md`）。
6. **链上作弊验证（zk/TEE）**：当前争议窗口过期走"乐观默认 honest 拨付"，缺真实验证。
   属经济模型决策，需你拍板是否接入 zk/TEE。
7. **链上治理参数**：设置 `voting_period` / `quorum` / `threshold`，让参数变更走治理而非你独断。

---

## 10. 主网化前代码侧待办（来自项目评估）

| 项 | 状态 | 主网前动作 |
|----|------|-----------|
| oracle `/sign` TLS+ACL | ✅ 已实现（Bearer+限流+HTTPS 可选） | 主网部署须设 ORACLE_SIGN_TOKEN + TLS 反向代理 |
| 作弊验证 zk/TEE | 未实现（乐观默认） | 产品/经济决策 |
| 仿真全绿 | ✅ 已完成 | 无需 |
| 前端实时查询+CLI 助手 | ✅ 已完成 | 无需 |
| event-subscriber 指标 | ✅ 已完成 | 接 Prometheus/Grafana |

---

## 快速命令清单（复制即用，替换 <> 占位）

```bash
CHAIN_ID=mcchain-mainnet-1; MONIKER=<节点名>; HD=$HOME/.mcchain
mcchaind init $MONIKER --chain-id $CHAIN_ID --home $HD
mcchaind keys add validator --keyring-backend file --home $HD
mcchaind add-genesis-account $(mcchaind keys show validator -a --keyring-backend file --home $HD) 1000000000umc --home $HD
mcchaind gentx validator 30000000000umc --chain-id $CHAIN_ID --moniker $MONIKER --commission-rate 0.05 --keyring-backend file --home $HD
mcchaind collect-gentxs --home $HD
mcchaind validate-genesis --home $HD
# 配好 §6 安全组 + §7 systemd 后：
sudo systemctl enable --now mcchain
```
