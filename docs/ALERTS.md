# 告警规则（ALERTS）

Prometheus 告警规则定义与说明。**可加载的规则文件是 `deploy/mcchain_alerts.yml`**
（`deploy/prometheus-mcchain.yml` 的 `rule_files` 指向它）——本文件是它的文档面，
两边必须同步改。基于 `docs/PERF_BASELINES.md` 的基线（基线变化后同步检查阈值）。

## 0. 指标生产方（改告警前先确认指标存在）

| 指标 | 生产方 | 暴露端口 |
|---|---|---|
| `mcchain_latest_block_height`、`mcchain_tokenomics_minted_supply`、`mcchain_tokenomics_total_supply_cap`、`mcchain_bank_total_supply` | `x/tokenomics/keeper/metrics.go`（每块） | API server `/metrics`，:1317 |
| `mcchain_oracle_last_attestation_timestamp` | `x/phonenode/keeper/attestation.go`（每次认证成功） | 同上 |
| `edgeai_*` / `dex_*` / `phonenode_*` / `depin_*` 业务计数器 | 各模块 `telemetry.IncrCounter` | 同上 |
| `cometbft_*` | CometBFT 内建（config.toml `prometheus=true`） | :26660 |
| `up` / `node_*` | Prometheus / node_exporter 内建 | :9090 / :9100 |

> 说明：`mcchain_validator_missed_blocks`、`mcchain_validator_jailed`、
> `mcchain_oracle_price_deviation_bps`、`mcchain_consensus_halt_height` 均无生产方，
> 因此不为它们设告警；节点层面的异常由 BlockStalled / NodeDown / PeerShortage 兜底。
> 精细的错过块率告警需接 slashing signing-info 查询，列入后续阶段。

## 1. 节点健康

```yaml
- name: node_health
  rules:
  - alert: NodeDown          # up{job=~"mcchain-(cometbft|app)"} == 0, 2m, critical
  - alert: BlockStalled      # delta(mcchain_latest_block_height[5m]) <= 0, 1m, critical
                             # 4s/块正常 5 分钟应推进 ~75 块；crisis halt 也由此兜底
  - alert: NodeFallingBehind # cometbft 高度落后全网最大值 30 块, 2m, warning
  - alert: PeerShortage      # cometbft_p2p_peers < 2, 10m, warning
```

## 2. 资源

```yaml
- name: resources
  rules:
  - alert: DiskFillingUp          # 根分区可用 <20%, 10m, warning
  - alert: DiskFillingUpCritical  # <10%, 5m, critical
  - alert: HighMemoryUsage        # 可用内存 <10%, 10m, warning
  - alert: HighCPUUsage           # >90% 持续 15m, warning
```

## 3. 认证 / 预言机链路

```yaml
- name: oracle
  rules:
  - alert: OracleAttestationStale # time() - mcchain_oracle_last_attestation_timestamp > 1800, 5m, warning
                                  # 设备认证是「贡献即挖矿」的入口，停滞=设备无法参与挖矿
```

## 4. 经济不变量（链级）

```yaml
- name: chain_invariants
  rules:
  - alert: MintedSupplyApproachingCap # minted/cap > 1.0, 5m, critical
                                      # 零通胀链创世即铸满，正常恒等——一旦超过即审计事件
  - alert: BankSupplyAboveMinted      # bank_total_supply - minted > 1e12 umc, 30m, warning
                                      # 与链上创世校验同口径：五池外注资检测
                                      # （devnet 因验证人注资天然偏高，部署时静默或调阈值）
```

## 5. 业务健康（第一阶段核心链路）

```yaml
- name: business
  rules:
  - alert: CheatDetectedSpike  # increase(edgeai_cheat_detected_count[1h]) > 10, 5m, warning
  - alert: OfflineSlashSpike   # increase(phonenode_offline_slash[1h]) > 50, 5m, warning
                               # 大规模设备掉线或心跳链路故障
```

## 6. 部署建议

- **前置条件**：`deploy/init.sh` 已启用 telemetry（`[telemetry] enabled=true` +
  `prometheus-retention-time>0`）并打开 `[api]`（监听 `0.0.0.0:1317`）——SDK v0.47 的
  应用指标经 API server `/metrics` 暴露，**不存在独立的 26661 端口**。
- 把 `deploy/mcchain_alerts.yml` 拷到 Prometheus `rule_files` 所指路径（默认
  `/etc/prometheus/rules/mcchain_alerts.yml`），`deploy/prometheus-mcchain.yml`
  已完成抓取与 alertmanager 对接，重载即生效。
- 通知渠道：PagerDuty / 钉钉 / 企业微信，按 severity 分级。
- **告警静默窗口**：治理升级窗口（升级前 30 分钟）静默所有 warning 级，避免噪音。
- **告警演练**：每季度做一次「故意触发」演练（如停掉一台 sentry 的 26660），
  确认值班人能按手册跑完。
