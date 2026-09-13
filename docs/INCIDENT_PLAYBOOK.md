# 应急预案（INCIDENT_PLAYBOOK）

> 配套 `docs/VALIDATOR_RUNBOOK.md`（告警→处置）使用。本手册只收**可直接复制执行的命令**，
> 按事故场景组织。每个场景最后都有一行「验证恢复」命令——执行完必须跑一遍确认。

---

## S1 · 节点进程崩溃 / 卡死（NodeDown / NodeFallingBehind）

**判断**：`systemctl status mcchaind` 非 active，或日志尾部有 panic。

```bash
# 1) 看最后 50 行日志定位原因
journalctl -u mcchaind -n 50 --no-pager

# 2a) 若是 OOM：先降内存占用（pruning 收紧 / 关闭其他进程），再重启
sudo systemctl restart mcchaind

# 2b) 若是数据库损坏（leveldb 报错）：
#     不要原地重试 —— 用昨天的快照恢复（见 backup.sh restore），丢块靠 P2P 追回
sudo systemctl stop mcchaind
sudo ./scripts/backup.sh restore <node_home> <昨天的快照.tar.zst>
sudo systemctl start mcchaind

# 3) 重启后确认追平
mcchaind status --node http://127.0.0.1:26657 | grep catching_up   # 期望 false
```

**验证恢复**：`./scripts/launch_verify.sh http://127.0.0.1:26657 120`（观察 2 分钟出块）

⚠️ **validator 节点重启前**：先确认 sentry 还在出块（validator 短暂离线 100 块内不会被
slashing，窗口=signed_blocks_window 21600 块，但连续缺席会累计 missed_blocks_counter）。

---

## S2 · 验证人被 jail（ValidatorJailed）

**判断**：`mcchaind query staking validator <valoper>` 状态为 `jailed`。

```bash
# 1) 修复根因（NTP 漂移 / 进程崩溃 / 网络断）——先修根因再 unjail，否则马上再进
chronyc tracking

# 2) 等待 jail 期满（downtime_jail_duration = 600s）
mcchaind query slashing signing-info <cons-pubkey> --node http://127.0.0.1:26657

# 3) unjail（用验证人运营账户签名）
mcchaind tx slashing unjail --from <operator-key> \
  --chain-id mcchain-mainnet-1 --node http://127.0.0.1:26657 \
  --gas auto --gas-adjustment 1.5 --fees 5000umc -y

# 4) 确认恢复签名
watch -n 5 'mcchaind query slashing signing-info <cons-pubkey> --node http://127.0.0.1:26657'
```

**验证恢复**：missed_blocks_counter 不再增长；下一个窗口签署率回到 > 90%。

---

## S3 · 链级 invariant 触发 halt（ChainHalted）

**这是最高级别事故**。crisis invariant 失败 → 链停止出块。

```bash
# 1) 确认 halt 原因（日志里 invariant 名字）
journalctl -u mcchaind -n 100 --no-pager | grep -iE "invariant|halt"

# 2) 立即通知全体验证人 + 社区公告（用预置话术包，见运营公告模板 §4）
#    不要自行「重启解决」——invariant halt 重启还会再 halt。

# 3) 处置路径二选一（需全体验证人协调）：
#    a) 软件缺陷 → 走 docs/GOVERNANCE.md 紧急升级流程，发布补丁版本后
#       全体验证人同高度重启
#    b) 误报 → 用 --halt-height 越过该高度重启（仅在 core team 确认误报后）

# 4) 恢复后全面复核
./scripts/launch_verify.sh http://127.0.0.1:26657 600
```

**升级路径**：任何值班人 → core team → 全体验证人频道。**30 分钟内无响应即升级**。

---

## S4 · 对外 RPC 不可用（NodeDown / 429 风暴）

**判断**：blackbox 探测 `rpc.mcchain.example` 失败，或 nginx 429 占比 > 20%。

```bash
# 1) 确认上游是否活着
curl -sf http://127.0.0.1:26657/status | head -c 200

# 2a) 上游挂 → 按 S1 处理；nginx 自动返回 503，客户端按熔断策略切换（web/endpoints.json）

# 2b) 被打爆（429 占比高）→ 收紧限流并扩容
sudo vim /etc/nginx/sites-available/mcchain.conf   # 降 rate 或加 upstream
sudo nginx -t && sudo systemctl reload nginx

# 3) 启用备用 RPC 域名（提前在 DNS 配好，TTL 60s）
#    运营公告切换公告（用预置话术 §3）
```

**验证恢复**：`curl -sf https://rpc.mcchain.example/status | jq .result.sync_info.latest_block_height`

---

## S5 · APP 需紧急强更（客户端致命 bug / 钓鱼仿冒）

```bash
# 1) 服务端把 minimum_version 拉到当前版本（客户端启动时拉取该值，低于即强制升级页）
#    配置中心：app-config API 的 min_version 字段
curl -X PUT https://api.mcchain.example/admin/config \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"min_version":"1.0.1"}'

# 2) 分发页更新 APK + sha256（官网 direct 链接）
# 3) 全渠道公告（预置话术 §5），社群置顶
```

**验证恢复**：老版本客户端启动后进入强更页；新版本注册/挖矿链路全通。

---

## S6 · 预言机 attestation 停更（OracleAttestationStale）

```bash
# 1) 检查 TeeOracle 服务
journalctl -u tee-oracle -n 50 --no-pager

# 2) 确认 MC_ORACLE_PUBKEY 与 TEE 输出的公钥一致
systemctl show tee-oracle | grep Environment

# 3) TEE 实例故障 → 切换备用 TEE（预置了同 pubkey 的备用实例）
#    ⚠️ 生产禁止切 SoftOracle（MC_ORACLE_ALLOW_SOFT=1）—— 这是红线
```

**验证恢复**：`mcchaind query oracle <latest-attestation>` 时间戳 < 5 分钟。

---

## 通用守则

1. **先看日志再动手**：所有处置前先截取 50 行日志存档（事后复盘用）。
2. **validator 慎重启**：先确认 sentry/对端在出块，再动签名节点。
3. **每处置一次，复盘一次**：24h 内按 RUNBOOK §4 模板出报告。
4. **本手册缺场景时**：按「隔离 → 观察 → 上报 → 恢复」四步走，不要临场发明操作。
