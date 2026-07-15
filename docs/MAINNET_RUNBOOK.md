# MobileChain 主网 Runbook

本手册记录主网节点从初始化到稳定运行、升级的完整运维步骤。配套 `Dockerfile` / `docker-compose.yml` / `deploy/init.sh` / `deploy/start.sh`。

> 前提：目标机为 Linux（Ubuntu 22.04 LTS），已装 Docker 或 Go 1.22+；链 home 目录为空。
> 沙箱（Windows 开发机）仅用于构建与单节点验证，不承载公网主网。

---

## 1. 编译 / 取镜像

```bash
# 方式 A：Docker（推荐）
git clone <私有仓> mcchain && cd mcchain
docker build -t mcchaind:mainnet .

# 方式 B：直编
go build -o mcchaind ./cmd/mcchaind
```

## 2. 初始化链

```bash
export HOME_DIR=$HOME/.mcchain
export CHAIN_ID=mcchain-mainnet-1
mcchaind init validator --chain-id $CHAIN_ID --home $HOME_DIR
```

### 2.1 修复模块默认参数

`init` 产生的 genesis 中 `phonenode` 和 `edgeai` 模块的 proto-JSON 序列化可能为空，需手动补齐默认值后再做其他操作：

```python
# 使用任意 Python 3 执行
python3 -c "
import json
g = json.load(open('$HOME_DIR/config/genesis.json'))
# phonenode
pn = g.setdefault('app_state',{}).setdefault('phonenode',{})
pn['params'] = {
    'attestation_required': True, 'attestation_validity': '2592000',
    'sybil_device_binding': True, 'offline_grace_blocks': '100',
    'offline_slash_bps': '500', 'contrib_slash_bps': '1000',
    'attest_slash_bps': '2000',
}
# edgeai arbitrator = 团队多签地址（从编译链获取或查询 tokenomics 创世配置）
g['app_state'].setdefault('edgeai',{}).setdefault('params',{})['arbitrator'] = 'mc1uq85t4erj44lf3x23xnrr97lt4wlyfz5kkf96f'
json.dump(g, open('$HOME_DIR/config/genesis.json','w'), indent=2)
print('params patched')
"
```

> **注意**：**绝不**使用 `add-genesis-account`。全部 1e9 MC（= 1e15 umc）由 `tokenomics` 模块在创世时一次性铸造并分配到团队/社区/生态三个池，`add-genesis-account` 会导致超出上限的**通胀**。

### 2.2 规范化生产 genesis
```bash
python3 scripts/make_genesis.py \
  --genesis $HOME_DIR/config/genesis.json \
  --out $HOME_DIR/config/genesis.json \
  --config scripts/mainnet-genesis-config.json
mcchaind validate-genesis $HOME_DIR/config/genesis.json
```
固化内容：bond_denom/staking/mint/gov/crisis = umc；DePIN 设备激励池 5.5e14 umc（55%）；tokenomics 上限 1e15 umc；chain_id；断言普通账户存在。

> **固定总量双保险（P0/R1）**：本链为固定总量（1e9 MC = 1e15 umc），全部由 `tokenomics` 模块在创世时一次性铸造并强约束 `total_supply_cap`，`depin` 模块**无铸币权限**（其 BankKeeper 接口已移除 `MintCoins`）。`mint` 模块默认通胀 ≈13% 且持有 Minter，会绕过 cap 二次铸币——因此 `make_genesis.py` 已将其 `inflation_rate_change/inflation_max/inflation_min` 与 `minter.inflation` 归零，且 `app.InitChainer` 在**每次启动**时再次兜底强制归零（`goal_bonded` 故意保留默认值，因其若为 0 会在首区块 `bondedRatio/goal_bonded` 除零 panic 导致链 halt）。上线后务必核验 `mcchaind q mint params` 显示 `inflation_rate_max: "0.000000000000000000"`。

### 2.3 出块间隔
`make_genesis` / `edit_configs.py` 已将 `timeout_commit = "4s"`，无需手改。核验：
```bash
grep timeout_commit $HOME_DIR/config/config.toml   # 应为 "4s"
```

## 3. 团队多签验证人 + 创世交易

> 以下步骤需在 **安全、离线或受控网络** 的机器上执行，因为涉及 5 个团队助记词的恢复。
> 本链团队池（1.2e14 umc = 12% 总量，五池模型）由一个 3-of-5 多签 vesting 账户控制，该多签同时也是**创世验证人的委托账户**，
> 自抵押全部团队池（1.2e14 umc），确保资金100%参与共识。
> 恢复任意 3 个即可签名。

### 3.1 恢复 5 个团队密钥

```bash
# 逐个恢复，每个团队成员输入各自的助记词
mcchaind keys add team1 --recover --keyring-backend file --home $HOME_DIR
mcchaind keys add team2 --recover --keyring-backend file --home $HOME_DIR
mcchaind keys add team3 --recover --keyring-backend file --home $HOME_DIR
mcchaind keys add team4 --recover --keyring-backend file --home $HOME_DIR
mcchaind keys add team5 --recover --keyring-backend file --home $HOME_DIR
```

### 3.2 构建多签

```bash
# 默认按地址排序，与链上编译的 TeamAddress 一致
mcchaind keys add teammultisig \
  --multisig=team1,team2,team3,team4,team5 \
  --multisig-threshold=3 \
  --keyring-backend file --home $HOME_DIR
```

### 3.3 创世交易（gentx）—— 离线多签协作

```bash
# 生成 unsigned gentx
mcchaind gentx teammultisig 120000000000000umc \
  --chain-id $CHAIN_ID --from teammultisig --home $HOME_DIR \
  --keyring-backend test \
  --min-self-delegation 30000000000 --generate-only \
  --output-document unsigned_gentx.json

# 用 3 个团队成员签名（可能需要 –-offline）
mcchaind tx sign unsigned_gentx.json \
  --from team1 --multisig=teammultisig --signature-only \
  --offline --account-number 0 --sequence 0 \
  --chain-id $CHAIN_ID --home $HOME_DIR --keyring-backend test \
  > sig1.json
# 同理 team2 / team3 → sig2.json, sig3.json

# 合并为完整签名
mcchaind tx multisign unsigned_gentx.json teammultisig \
  sig1.json sig2.json sig3.json \
  --chain-id $CHAIN_ID --home $HOME_DIR --keyring-backend test \
  > signed_gentx.json
```

### 3.4 收集 + 生成生产 genesis

```bash
# 放入 gentx 目录（注意：目录名是 gentx 单数，不是 gentxs）
mkdir -p $HOME_DIR/config/gentx
cp signed_gentx.json $HOME_DIR/config/gentx/gentx_teammultisig.json

# 收集（此时可能报 "balance not in genesis" 警告，但 gentx 仍被添加）
mcchaind collect-gentxs --home $HOME_DIR

# 删除之前为了通过 gentx 校验而添加的临时 genesis-account（如果有）
# 供给实际上由 tokenomics 模块在 InitChain 时铸造，无需 genesis-account

# 规范化
python3 scripts/make_genesis.py \
  --genesis $HOME_DIR/config/genesis.json \
  --out $HOME_DIR/config/genesis.json \
  --config scripts/mainnet-genesis-config.json

mcchaind validate-genesis $HOME_DIR/config/genesis.json
```

> 固化后核验：`tokenomics.cap = 1e15`、`depin.initial_pool = 5.5e14`（设备激励 55%）、`chain_id = mcchain-mainnet-1`、`bond_denom = umc`、mint 通胀归零。

### 3.5 基金会 / 早期开发 创世拨付（占位地址须替换）
`tokenomics` 在创世时（`InitGenesis`）自动将基金会池（13% = 1.3e14 umc）拆分为「运营流动地址 T0 即时 5000 万 MC」+「2 年期线性释放地址（ContinuousVesting）8000 万 MC」，早期开发池（5% = 5000 万 MC）T0 全额拨付到开发资助地址（详见白皮书第八章 8.3）。三者当前为代码内**占位确定性地址**（`EarlyDevAddress` / `FoundationOpsAddress` / `FoundationVestingAddress`，由 `keys.go` 的 `derivedPlaceholder` 固定种子派生），**主网前必须替换为真实多签/运营地址**：修改 `x/tokenomics/types/keys.go` 的占位种子或接入真实公钥，重新编译、`init` 重生成 genesis 并 `validate-genesis` 通过后再上线。替换规则与 `TeamAddress` 一致（任何改动须重新端到端验证创世）。

## 4. 启动

### 4.1 Docker（推荐）
```bash
# 先把 $HOME_DIR 内容放到 deploy/validator，再：
docker compose up -d validator
docker compose logs -f validator
```

### 4.2 systemd（裸机）
`/etc/systemd/system/mcchaind.service`：
```ini
[Unit]
Description=MobileChain
After=network.target

[Service]
User=mc
WorkingDirectory=/home/mc
ExecStart=/usr/local/bin/mcchaind start --home /home/mc/.mcchain
Restart=always
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now mcchaind
journalctl -u mcchaind -f
```

## 5. 健康验收

```bash
mcchaind status | jq '.SyncInfo.latest_block_height'   # 高度持续递增
mcchaind q staking params          # bond_denom: umc
mcchaind q tokenomics supply       # 1e15 umc（=1e9 MC）
mcchaind q depin params            # initial_pool 5.5e14 umc（设备激励池 55%）
mcchaind q mint params             # inflation_rate_max: "0..."（固定总量双保险，绝不为 13%）
# 连续两次查总供应应完全相等（验证无二次通胀）
mcchaind q bank total 2>/dev/null | head -3
```

### 5.1 贡献即挖矿闭环验证
```bash
depin register-device <addr> pixel8 android
depin attest-device <addr> ch sig
phonenode register-node <addr> pixel8 android contributor
phonenode submit-attestation roothash nonce devicehash
depin submit-contribution t1 inference 80    # 节点余额 +400 umc
```
> 节点需周期性 `phonenode submit-state-proof`（心跳）保活；超 100 区块无心跳判离线 slash（设计内）。

## 6. 升级（plan / height）

```bash
# 提交升级提案并投票后，到高度触发
mcchaind tx gov submit-proposal software-upgrade v1.1 \
  --upgrade-height <H> --from validator --chain-id $CHAIN_ID --keyring-backend file
# 到高度前停服替换二进制，再 start（须配置 halt-height 自动停）
```

## 7. 常见问题

| 现象 | 排查 |
|---|---|
| 启动报 `min self delegation < lower bound` | gentx 的 min-self-delegation < 3e10 umc |
| 出块间隔 != 4s | 确认 timeout_commit=4s 并重启 |
| 贡献发币为 0，报 `device not attested` | 节点 attestation 被离线 slash 吊销；检查心跳是否按期提交 |
| 私钥泄露风险 | 验证人私钥用 tmkms/horcrux 隔离，勿与本机日常环境混用 |

## 8. 安全基线

- 验证人私钥：HSM / 独立 signer，不与其他服务同机。
- 防火墙：仅开放 26656(p2p)/26657(rpc,可限 IP)/1317(api,可关)。
- 备份：定期快照 `$HOME_DIR/data`；密钥单独加密备份（绝不进 git）。
- 监控：Prometheus(26660) + Grafana + 出块停滞/签名 miss 告警。

## 9. 上线后加固项（非阻塞，建议尽快）

- **最小 Gas 价格**：当前 `minimum-gas-prices = "0umc"`（为兼容移动端「贡献即挖矿」0 手续费交易）。公网暴露后建议设为小额正数（如 `0.0025umc`）以抵御免费 spam/DoS；切换前提是移动端 SDK 在上报 `depin submit-contribution` 等交易时附带 `--fees`/`--gas-prices`，否则交易会被节点拒收。可通过 `app.toml` 或 `edit_configs.py` 调整。
- **多验证人**：单验证人主网无容错，尽快扩到 ≥3 个独立验证人（不同机房/运营方），`docker-compose.yml` 已预留布局。
- **IBC 通道**：若暂不跨链，可通过治理关闭 IBC 相关模块以缩小攻击面。
