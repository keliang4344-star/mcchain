# 主网基础设施部署规范

> 说明：服务器需付费购买/租赁，涉及云账号与 SSH 密钥管理。本规范提供精确的机型清单与自动化部署脚本，按步骤操作即可拉起主网。

## 一、生产环境要求

开发验证阶段可在本地环境完成单节点测试，但公网主网需满足以下条件：

- 固定公网 IPv4
- Linux 服务器（Ubuntu 22.04 LTS）
- 多节点冗余（≥3 验证人跨可用区部署）
- 独立隔离的签名与监控基础设施

**结论**：主网部署至独立 Linux 服务器集群，开发/签名/监控在本地隔离环境完成。

## 二、推荐配置（跨可用区容灾）

| 角色 | 数量 | 规格参考 |
|---|---|---|
| 验证人 validator | ≥3（跨可用区） | 标准型：4 vCPU / 8 GB / 200 GB SSD |
| 全节点 / RPC | 2 | 4 vCPU / 8 GB / 200 GB SSD |
| 快照/归档 snapshot | 1 | 2 vCPU / 4 GB / 100 GB SSD |
| 对象存储（快照备份） | 1 | 对象存储服务（标准存储） |
| 固定公网 IP + 带宽 | 每节点 | 固定带宽 5–10 Mbps 或按量计费 |

系统：Ubuntu 22.04 LTS。验证人务必**跨可用区**部署，消除单点故障。

## 三、服务器开通流程

1. 登录云服务商控制台 → 计算实例 → 新建实例。
2. 镜像：Ubuntu Server 22.04 LTS；地域选择支持多可用区的就近地域。
3. 机型：标准型实例，按上表选规格；系统盘 SSD 200 GB。
4. 网络：分配公网 IPv4，带宽按量或固定 5–10 Mbps；安全组放行 **26656（TCP，p2p）/ 26657（TCP，RPC，仅内网或受限）/ 1317（REST，内网）**。
5. 密钥：创建 SSH 密钥对并下载私钥（`.pem`），妥善保存，此为服务器访问唯一凭据。
6. 重复创建 ≥3 台验证人 + 2 台全节点 + 1 台快照（分布在不同可用区）。
7. 开通对象存储服务用于快照归档。

## 四、服务器部署流程

1. 安装 Docker（Ubuntu）：

   ```bash
   sudo apt update && sudo apt -y install docker.io docker-compose-plugin
   sudo usermod -aG docker $USER   # 重新登录生效
   ```

2. 拉取代码（私有仓库，已通过 .gitignore 隔离私钥）：

   ```bash
   git clone <私有仓> mcchain && cd mcchain
   ```

3. 生成主网 genesis（将示例公钥替换为团队/验证人真实多签密钥，参见 `team_multisig_vault.txt` 与 `scripts/make_genesis.py`）：

   ```bash
   python3 scripts/make_genesis.py --genesis testnet/config/genesis.json \
     --out build/genesis_mainnet.json --config scripts/genesis-config.example.json
   mcchaind validate-genesis build/genesis_mainnet.json
   ```

4. 启动链（docker-compose 包含 validator / fullnode / snapshot 三项配置）：

   ```bash
   docker compose up -d validator
   docker compose up -d fullnode snapshot
   ```

   或裸机部署：`mcchaind start --home /data/mcchain`（参考 `docs/MAINNET_RUNBOOK.md`）。

## 五、上线前准备清单

- [ ] 完成云服务器的付费开通
- [ ] 提供 SSH 公网 IP + 密钥，授权部署操作
- [ ] 提供真实的**验证人 / 团队多签公钥**（替换 `team_multisig_vault.txt` 中的占位项为受控密钥，或将已生成的 5 把真实密钥分散保管助记词）
- [ ] 配置域名（可选，用于 RPC 公网暴露场景）
- [ ] 收敛安全组策略（RPC / REST 端口仅对授权访问方开放，禁止 `0.0.0.0` 全放行）

## 六、上线前安全检查清单

- [ ] `git status` 确认无 `testnet/`、`mcchaind.exe`、`team_multisig_vault.txt` 等敏感文件残留
- [ ] `git ls-files | grep testnet` 返回为空
- [ ] 源码中无硬编码助记词/私钥（`grep -rniE 'mnemonic|privkey' --include=*.go .` 仅命中运行时派生逻辑）
- [ ] genesis 中团队多签地址与 `team_multisig_vault.txt` 公钥一致
