# 验证人运行手册（VALIDATOR_RUNBOOK）

> **白话**：下面所有「你」都指**值班人**。本手册按「症状 → 检查项 → 处置」结构，
> 24x7 都能照着点，不需要记任何额外的背景。配合 `docs/ALERTS.md` 的告警使用。

## 0. 一张图看懂拓扑

```
                  [ 钱包 / 用户 ]
                         |
                         | HTTPS (TLS via nginx)
                         v
                +---------------+
                |  Sentry 节点  |   ← 公开 RPC / REST / gRPC / P2P 入口
                |  (只转发)     |
                +-------+-------+
                        |  (内部 IP / private_peer_ids 限制)
                        v
              +-------------------+
              |  签名验证人节点  |   ← **唯一持有签名私钥的机器**
              |  (签块 / 投票)   |
              +-------------------+
```

签块机器**不直接暴露**在公网，所有公网流量都先到 Sentry。Sentry 可以挂、
可以重启、可以轮换，不会影响出块。

## 1. 告警来了！对应手册

### 1.1 `ValidatorMissedBlocksTooHigh`

**症状**：签块错过率 > 5% / 1 小时。

**检查**：
```bash
# 1) 看是不是本地没签
mcchaind query slashing signing-info <consPubKey> --node tcp://localhost:26657

# 2) 看时间同步（最常见原因）
chronyc tracking       # 或 ntpq -p

# 3) 看系统负载
uptime; top -bn1 | head -20
free -h; df -h
```

**处置**：
- 如果 `time_drift > 1s`：NTP 没工作，先解决 NTP。
- 如果 CPU/MEM 饱和：看 `docs/PERF_BASELINES.md` 是不是 mnesia-like 序列化阻塞。
- 如果 `min_signed_per_window < 0.5`：快到 slash 阈值（50% missed），立刻重启 + 跟进。

### 1.2 `ValidatorJailed`

**症状**：节点被 jailed。

**检查**：
```bash
mcchaind query slashing signing-info <consPubKey> --node tcp://localhost:26657
mcchaind query staking validator <valoperAddr> --node tcp://localhost:26657
```

**处置**：
- 如果 `jailed_until` 在将来：等不到就 `unjail`。
- 如果被 slash：这是共识已记账的损失，按治理流程提交事后报告。
- 修好后：
  ```bash
  mcchaind tx slashing unjail --from <key-name> \
    --chain-id mcchain-mainnet-1 --node ... -y
  ```

### 1.3 `NodeStuckLowHeight`

**症状**：本地高度长期落后公网。

**检查**：
```bash
mcchaind status 2>&1 | grep -E "latest_block_height|catching_up"
curl -s localhost:26657/net_info | jq '.result.n_peers'
```

**处置**：
- `catching_up = true` 且高度差在缩：等。
- `catching_up = true` 但卡住：看磁盘（`df -h`）+ 看 p2p peer 数。
- peers < 5：检查 sentry 是否可达，DNS / 防火墙。

### 1.4 `DiskFillingUp`

**症状**：磁盘使用率 > 80%。

**检查**：
```bash
du -sh /root/.mcchaind/data 2>/dev/null
mcchaind query block --type=height <latest>  # 验证链能跑
```

**处置**：
- 如果是 archive 节点：扩容磁盘或清理历史 tx_index。
- 如果是 pruning `custom`：把 `pruning-keep-recent` 调小一档（不破环 SLA 的前提下）。
- **绝不删 data/ 目录**，那等于全链丢失。

### 1.5 `OracleAttestationStale`

**症状**：价格预言机 30 分钟没更新。

**检查**：
```bash
mcchaind query oracle attestation <pair> 2>/dev/null || echo "no oracle module"
journalctl -u tee-oracle --since "30 min ago"
```

**处置**：
- 检查 TEE 服务运行状态（TeeOracle 部署文档 `docs/ORACLE_FRAMEWORK.md`）。
- 检查 `MC_ORACLE_PUBKEY` 环境变量是否还指向当前正确的 TEE。
- 主网**不要**临时切到 `MC_ORACLE_ALLOW_SOFT=1` —— 这是红线项，会让 dApp 拿到错误价格。

## 2. 日常维护清单（每天 5 分钟）

```bash
# 1) 本节点状态
mcchaind status 2>&1 | grep -E "latest_block_height|catching_up"

# 2) 签名信息（missed_blocks_counter 应该是 0 或缓慢上升）
mcchaind query slashing signing-info <consPubKey>

# 3) 即将 jailed 阈值告警
echo "Miss ratio: $(echo "scale=4; $(mcchaind q slashing signing-info <consPubKey> | jq .missed_blocks_counter) / 21600" | bc)"

# 4) 磁盘 / 内存
df -h /root; free -h
```

## 3. 升级流程（hard-fork / 软件升级）

1. 看治理提案里的升级高度 H。
2. 升级前 ≥24 小时发运维公告。
3. 升级前 1 小时做一次数据快照（`mcchaind export` 或文件系统 snapshot）。
4. 在 H - 10 块时停止签块节点。
5. 升级二进制并重启。
6. 节点启动后立刻 `mcchaind status` 确认 `catching_up=false` 且在出块。
7. 在社区频道回报。

## 4. 事故复盘模板

```
时间：
症状：
告警 ID：
根因：
处置时间线：
影响范围（被 slash / 漏签 / 资金损失）：
改进项：
```

复盘报告 24 小时内发到公开治理论坛。

## 5. 与其他文档的关系

- 告警阈值 → `docs/ALERTS.md`
- 节点配置参数 → `docs/NODE_CONFIG.md`
- 出块参数口径 → `docs/PROTOCOL_PARAMS.md`
- 部署服务器清单与规格 → `docs/DEPLOYMENT_GUIDE.md`