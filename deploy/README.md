# deploy/ · 主网部署资产索引

> 本目录只放**部署期**资产（systemd unit / 环境变量模板 / 监控抓取 / 告警规则 / Nginx）。
> 创世生成与上线验收在 `scripts/` 下（`gen_genesis_teamval.sh`、`launch_verify.sh`）。
> 完整流程见 `docs/DEPLOYMENT_GUIDE.md`，上线前核对见 `scripts/launch_verify.sh`。

## 1. 资产清单

| 文件 | 用途 | 装到哪 |
|------|------|--------|
| `mcchaind.service` | 链节点（验证人/全节点） | `/etc/systemd/system/` |
| `mainnet.env.example` | **创世四道闸门** + 节点环境变量模板 | `/etc/mcchain/mainnet.env`（600） |
| `mc-oracle-signer.service` | 链下 attestation 签名服务（`mcchaind oracle`） | `/etc/systemd/system/` |
| `oracle-signer.env.example` | 签名服务全部环境变量（含 [S] 严格模式必填项） | `/etc/mcchain/oracle-signer.env`（600） |
| `mc-indexer.service` | 事件索引器（JSONL 后端，只读不抢节点） | `/etc/systemd/system/` |
| `mc-event-subscriber.service` | 事件订阅 + Prometheus 指标导出 | `/etc/systemd/system/` |
| `prometheus-mcchain.yml` | Prometheus 抓取配置（含 subscriber/oracle 探测） | `/etc/prometheus/` |
| `mcchain_alerts.yml` | 六组告警规则 | `/etc/prometheus/rules/` |
| `nginx-mcchain.conf` | RPC/REST 反向代理 + 限流 | `/etc/nginx/conf.d/` |
| `init.sh` | 目标主机上的链 home 初始化（genesis 已生成的前提下） | — |

## 2. 变量命名：两个预言机，别装错

仓库里有**两套** oracle 实现，用途完全不同：

| | `mcchaind oracle`（internal/oraclesvc） | `mc-oracle`（cmd/oracle） |
|---|---|---|
| 定位 | **主网唯一**认证签名服务 | 开发/CI 工具 |
| 鉴权 | `/sign` Bearer `ORACLE_SIGN_TOKEN` | `/attest` Bearer `ORACLE_API_KEY` |
| 传输 | TLS（`ORACLE_TLS_CERT/KEY`）或显式 `ORACLE_ALLOW_PLAINTEXT` | `ORACLE_BIND_ADDR`，非回环强制 API key |
| 设备证据 | Play Integrity / Huawei / Android Key Attestation **证书链校验**（`ORACLE_ACCEPT_ROOTS`） | `ORACLE_VERIFIER=soft|tee` 闸门；**strict 下必拒绝启动**（tee 未接线） |
| systemd | `mc-oracle-signer.service` | 无（不要装到生产机） |

> 结论：主网装 `mc-oracle-signer.service`；`cmd/oracle` 只用于本机联调。

## 2.5 三种起链入口

MC 提供三种等价能力的入口，按部署形态选其一即可：

| 入口 | 适用 | 等价 systemd 行为 | 备注 |
|---|---|---|---|
| `systemctl start mcchaind` | 标准生产部署 | — | 主网推荐 |
| `./deploy/start.sh` | 容器 / 无 systemd / 临时调试 | ✅ 共享同一套闸门（MC_REQUIRE_REAL_GENESIS_KEYS / MC_ORACLE_PUBKEY / MC_FOUNDATION_*_PUBKEY） | 自带启动前自检、日志归档、`--stop/--status/--foreground` |
| `./scripts/devnet.sh start` | 本地开发/集成测试 | n/a（专用 devnet） | **勿在主网用**（自动注资 devnet 账户，会破坏 1B 上限） |

`./deploy/start.sh` 与 `mcchaind.service` 的关键差异：
- 启动前 fail-fast 自检：home/genesis/闸门齐备，缺一拒绝启动；
- 日志归档：`$LOG_DIR/mcchaind-{timestamp}.log` + `mcchaind-current.log` 软链；7 天前自动清理；
- pid 文件 `$HOME_DIR/.mcchaind.pid`；`--stop` 用 SIGTERM/SIGKILL 两段收；
- 非主网 chain-id 自动注入 `MC_ORACLE_ALLOW_SOFT=1`（生产下会被 app.go 拦下）。

## 3. 装机顺序（单机最小集）

```bash
# ① 链节点
sudo install -m 644 deploy/mcchaind.service      /etc/systemd/system/
sudo install -d -m 750 /etc/mcchain
sudo install -m 600 deploy/mainnet.env.example   /etc/mcchain/mainnet.env   # 填真实值
# ② 签名服务（可与节点同机，建议独立机）
sudo install -m 644 deploy/mc-oracle-signer.service /etc/systemd/system/
sudo install -m 600 deploy/oracle-signer.env.example /etc/mcchain/oracle-signer.env
# ③ 观测
sudo install -m 644 deploy/mc-indexer.service          /etc/systemd/system/
sudo install -m 644 deploy/mc-event-subscriber.service /etc/systemd/system/
sudo install -m 644 deploy/prometheus-mcchain.yml      /etc/prometheus/
sudo systemctl daemon-reload
sudo systemctl enable --now mcchaind mc-oracle-signer mc-indexer mc-event-subscriber
```

各 unit 依赖的专用账户（需先建）：
```bash
sudo useradd -r -m -d /var/lib/mcchain        -s /usr/sbin/nologin mcchain
sudo useradd -r -m -d /var/lib/mcchain-oracle -s /usr/sbin/nologin mcchain-oracle
sudo useradd -r -m -d /var/lib/mcchain-indexer -s /usr/sbin/nologin mcchain-indexer
sudo useradd -r -m -d /var/lib/mcchain-sub    -s /usr/sbin/nologin mcchain-sub
```

## 4. 起链后必做的三项交叉核对

```bash
# 1) 四道闸门真的生效（缺 MC_FOUNDATION_* 节点会直接退出，日志里能看到 tokenomics 报错）
sudo journalctl -u mcchaind -n 50 --no-pager | grep -i "tokenomics\|oracle\|panic"

# 2) 预言机公钥与节点配置一致
curl -s http://127.0.0.1:8080/pubkey | jq -r .pubkey_base64   # 应等于 mainnet.env 的 MC_ORACLE_PUBKEY

# 3) 全量验收（含总供应 == 1e15、验证人在线、参数五项）
./scripts/launch_verify.sh http://127.0.0.1:26657 900
```
