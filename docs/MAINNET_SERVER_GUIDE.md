# 主网服务器开通与部署指南（T3：服务器资源）

> 说明：服务器需**付费购买/租赁**且涉及你的云账号与 SSH，我无法代购。本指南给出精确到机型的清单与一键部署脚本，你付费后点几下即可拉起真实主网。

## 一、为什么不能在本机直接跑公网主网
本机实测：Windows 10 + mid-range consumer CPU + 32GB + 住宅 WLAN（动态 IP/NAT/无 SLA）。
- 硬件负载够单节点，但 **OS（Windows）+ 网络（动态IP/无 SLA）+ 单机单点** 均不满足公网主网要求。
- 结论：租 Linux 服务器，本机仅作开发/签名/监控终端。

## 二、推荐配置（Cosmos 主网基准，跨可用区容灾）
| 角色 | 数量 | 规格（云服务商 CVM 参考） | 月成本(估) |
|---|---|---|---|
| 验证人 validator | ≥3（跨可用区） | 标准型 S5：4 vCPU / 8 GB / 200 GB SSD（增强型云盘） | ~¥400-600/台 |
| 全节点 / RPC | 2 | 4 vCPU / 8 GB / 200 GB SSD | ~¥400-600/台 |
| 快照/归档 snapshot | 1 | 2 vCPU / 4 GB / 100 GB SSD | ~¥200/台 |
| 对象存储（快照备份） | 1 | COS 标准存储 | 按量 |
| 固定公网 IP + 带宽 | 每节点 | 按固定带宽 5-10 Mbps 或按量 | 含上表 |
| **合计** | | | **约 ¥1500-2500/月（≈ $200-350）** |

系统：Ubuntu 22.04 LTS。务必**跨可用区**部署验证人，避免单点。

## 三、云服务商开通步骤（你操作，约 10 分钟）
1. 登录云服务商控制台 → 云服务器 CVM → 新建实例。
2. 镜像：Ubuntu Server 22.04 LTS；地域选离你近且支持多可用区（如广州/上海）。
3. 机型：标准型 S5，按上表选规格；系统盘 SSD 云硬盘 200 GB。
4. 网络：勾选「分配公网 IPv4」，带宽按量或固定 5-10 Mbps；安全组先放通 **26656(TCP,p2p) / 26657(TCP,RPC,仅内网或受限) / 1317(REST,内网)**。
5. 密钥：创建 SSH 密钥对并下载私钥（`.pem`），妥善保存。
6. 重复创建 ≥3 台验证人 + 2 台全节点 + 1 台快照（分布在不同可用区）。
7. 开通 COS 用于存储快照归档。

## 四、服务器上部署（代码已具备，docker 一键起）
1. 安装 Docker + docker-compose（Ubuntu）：
   ```bash
   sudo apt update && sudo apt -y install docker.io docker-compose-plugin
   sudo usermod -aG docker $USER   # 重登生效
   ```
2. 拉代码（私有仓，已 .gitignore 隔离私钥）：
   ```bash
   git clone <你的私有仓> mcchain && cd mcchain
   ```
3. 生成主网 genesis（把示例公钥换成真实团队/验证人多签，见 team_multisig_vault.txt 与 scripts/make_genesis.py）：
   ```bash
   python3 scripts/make_genesis.py --genesis testnet/config/genesis.json \
     --out build/genesis_mainnet.json --config scripts/genesis-config.example.json
   mcchaind validate-genesis build/genesis_mainnet.json
   ```
4. 起链（docker-compose 已含 validator/fullnode/snapshot 三件套）：
   ```bash
   docker compose up -d validator
   docker compose up -d fullnode snapshot
   ```
   或裸机：`mcchaind start --home /data/mcchain`（参考 docs/MAINNET_RUNBOOK.md）。

## 五、必须你亲自操作的环节（我无法代做）
- [ ] 付费开通云服务器（涉及你的支付与账号）
- [ ] 提供 SSH 公网 IP + 密钥，授权我执行部署命令（或你本地按本指南执行）
- [ ] 提供真实**验证人/团队多签公钥**（替换 `team_multisig_vault.txt` 占位为你掌控的密钥，或保留已生成的 5 把真实密钥并分散保管助记词）
- [ ] 域名（可选，RPC 公网暴露时用）
- [ ] 安全组收敛（RPC/REST 仅对需访问方开放，勿 0.0.0.0 全开）

## 六、上线前检查清单（防密钥泄露）
- [ ] `git status` 无 `testnet/`、`mcchaind.exe`、`team_multisig_vault.txt`
- [ ] `git ls-files | grep testnet` 为空
- [ ] 源码无硬编码 mnemonic/私钥（`grep -rniE 'mnemonic|privkey' --include=*.go .` 仅命中运行时派生）
- [ ] genesis 中团队多签地址与 `team_multisig_vault.txt` 公钥一致
