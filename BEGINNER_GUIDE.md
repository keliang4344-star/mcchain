# MC 公链 · 节点部署指南

> 目标：在云服务器上部署 MC 公链创世节点，组成出块网络。
> 前置：已拥有一台云服务器（Linux/Ubuntu 22.04，≥2 vCPU / ≥4 GB / ≥60 GB SSD，具备公网固定 IPv4）。

---

## 一、环境准备

将以下两个必需文件传输至目标服务器：

1. `build/mcchaind` —— 链程序（Linux 二进制）
2. `server_setup.sh` —— 自动化部署脚本

### 文件传输方式

**推荐：SCP 上传**

```bash
scp build/mcchaind root@<服务器IP>:/root/
scp server_setup.sh root@<服务器IP>:/root/
```

或通过云服务商控制台的文件管理功能上传至 `/root/` 目录。

---

## 二、执行部署

在服务器终端执行：

```bash
cd /root
chmod +x server_setup.sh
sudo ./server_setup.sh
```

脚本自动完成以下步骤：

1. 初始化节点
2. 规范创世配置（币种 umc、通胀清零、治理参数设定）
3. 创建验证人密钥（**助记词保存于 `validator_key.json`，务必妥善备份**）
4. 为验证人拨款 200k MC
5. 生成 gentx（自抵押 30k MC）
6. 收集并校验创世文件
7. **后台启动节点**

---

## 三、验证出块状态

脚本执行完毕后，检查日志：

```bash
tail -f chain.log
```

若观察到类似以下输出，且区块高度持续递增，即表示节点正常运行：

```
committed state ... height=1
committed state ... height=2
committed state ... height=3
...
```

按 `Ctrl+C` 退出日志查看（节点仍在后台运行）。

> 检查进程状态：`ps aux | grep mcchaind`
> 停止节点：`pkill -9 -f mcchaind`

---

## 四、常见问题排查

1. **币种必须为 umc**：默认初始化的币种为 `stake`，MC 实际使用 `umc`。部署脚本已自动完成替换，否则链无法启动。
2. **自抵押最低 30k MC**：gentx 须包含 `--min-self-delegation 30000000` 参数，否则创世阶段将触发 panic。脚本已固化为默认。
3. **重启前需终止旧进程**：若节点未正常退出，旧进程将锁住数据目录，再次启动会报 `failed to initialize database: ... used by another process`。解决方式：先执行 `pkill -9 -f mcchaind` 再启动。

---

## 五、部署注意事项

- 本指南覆盖的是**单验证人网络**：单个验证人出块、持有 100% 质押。服务器宕机将导致链停摆（单点故障风险），不构成去中心化主网。
- 密钥默认使用 **test 后端（免密）** 存储于服务器上，仅适用于测试/演示环境。**生产环境**须切换为 `file` 后端 + 独立签名机架构，并妥善备份 `validator_key.json` 中的助记词（持有助记词即控制对应质押资产）。
- 若需通过浏览器仪表盘（`web/index.html`）连接节点，需在服务器安全组中放行 `26657`（RPC）端口，并配置 RPC 监听 `0.0.0.0`（默认仅监听本地回环）。建议先确认出块正常后，再单独配置网络安全策略。
- 去中心化主网还需：≥4 个独立验证人、代币透明分配、TMKMS / 签名机安全架构、第三方安全审计。详见 `docs/MAINNET_DEPLOY_RUNBOOK.md`。

---

## 六、从单节点扩展到多验证人联盟

单节点网络仅适用于开发测试。**去中心化主网的最低配置为 4 个独立验证人**，分布在至少 3 台物理/云服务器上。

### 前置条件

- 4 台独立 Linux 服务器（或虚拟机），均已安装 `mcchaind`
- 服务器间网络互通（防火墙放行 26656 p2p 端口）
- 每台服务器具备独立公网 IP 或内网互通

### 步骤 1：各验证人初始化

在 **每一台** 服务器上执行：

```bash
# 服务器 A（协调节点，汇总 genesis）
mcchaind init validator-a --chain-id mcchain-mainnet-1 --home $HOME/.mcchain

# 服务器 B
mcchaind init validator-b --chain-id mcchain-mainnet-1 --home $HOME/.mcchain

# 服务器 C
mcchaind init validator-c --chain-id mcchain-mainnet-1 --home $HOME/.mcchain

# 服务器 D
mcchaind init validator-d --chain-id mcchain-mainnet-1 --home $HOME/.mcchain
```

### 步骤 2：写入创世账户并生成 gentx

各服务器执行（以 A 为例，B/C/D 同理替换 moniker 与金额）：

```bash
# 创建验证人密钥（生产环境必须使用 file 后端，严禁 test 后端）
mcchaind keys add validator --keyring-backend file --home $HOME/.mcchain

# 获取地址
VALIDATOR_ADDR=$(mcchaind keys show validator -a --keyring-backend file --home $HOME/.mcchain)

# 写入创世账户（每验证人拨款 200k MC = 200000000000umc）
mcchaind add-genesis-account $VALIDATOR_ADDR 200000000000umc --home $HOME/.mcchain

# 生成 gentx（自抵押 30k MC，不低于 min-self-delegation）
mcchaind gentx validator 30000000000umc \
  --chain-id mcchain-mainnet-1 \
  --home $HOME/.mcchain \
  --keyring-backend file \
  --min-self-delegation 30000000000
```

### 步骤 3：在协调节点汇总 gentx

将 B、C、D 三台服务器的 `$HOME/.mcchain/config/gentx/` 目录下的 JSON 文件拷贝至服务器 A 对应目录：

```bash
# 在 A 上执行汇总
mcchaind collect-gentxs --home $HOME/.mcchain

# 验证 genesis 完整性
mcchaind validate-genesis $HOME/.mcchain/config/genesis.json
```

> 建议同时使用 `scripts/make_genesis.py` 规范化 genesis（denom=umc、通胀清零、DePIN 池初始化等），详见 `docs/MAINNET_DEPLOY_PLAN.md`。

### 步骤 4：分发 genesis.json

```bash
scp $HOME/.mcchain/config/genesis.json root@<B_IP>:$HOME/.mcchain/config/genesis.json
scp $HOME/.mcchain/config/genesis.json root@<C_IP>:$HOME/.mcchain/config/genesis.json
scp $HOME/.mcchain/config/genesis.json root@<D_IP>:$HOME/.mcchain/config/genesis.json
```

### 步骤 5：全员启动

4 台服务器全部执行：

```bash
mcchaind start --home $HOME/.mcchain
```

或使用 `systemd` 守护：

```bash
sudo systemctl enable --now mcchaind
```

### 步骤 6：验证出块

任意节点执行：

```bash
# 查看区块高度是否持续增长
mcchaind status | jq '.SyncInfo.latest_block_height'

# 查看验证人集合（期望 4 个）
mcchaind q staking validators --output json | jq '.validators | length'
# 期望输出：4
```

### 关键要点

- **单节点仅用于开发测试**，不具备去中心化保障
- **4 验证人为主网最低配置**——任意 1 个故障仍可正常出块（BFT 容错：f < n/3，4 节点最多容忍 1 个故障）
- **生产环境必须使用 `file` 密钥后端**，配合 TMKMS / Horcrux 签名机隔离私钥
- 验证人节点应**跨可用区 / 跨云服务商部署**，避免单点故障导致链停摆

---

## 七、故障排查

收集服务器终端完整的错误日志输出，参照第四节"常见问题排查"定位根因。以上三种常见问题均有已验证的解决方案。
