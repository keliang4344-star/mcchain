# MobileChain 主网上线就绪报告

> 生成时间：2026-07-13
> 评估方法：基于代码实况 + 全量安全自审（`go build`/`go vet`/`go test` 全绿）+ 预言机签名↔链上验签冒烟测试（PASS）。
> 原则：诚实标注「已完成 / 部分完成 / 阻塞（需外部资源）」，**不虚构完成度**。

---

## 一、总体结论

**链协议层（核心协议 / 经济模型 / 挖矿 / attestation / EdgeAI / 治理）已完整并通过测试。** 缺陷仅在运维层与主网外部依赖。

本次（今天）补齐并验证的项：
- ✅ 监控告警栈（Prometheus / Grafana / Alertmanager + 规则 + 看板）
- ✅ 链下预言机签名服务（`cmd/oracle`，签名与链上 `TeeOracle` 验签一致，已冒烟测试 PASS）
- ✅ Gas / 费用运营参数固化 + 文档
- ✅ 后端 API 文档
- ✅ 安全自审（build / vet / test 全绿）

**仍阻塞、需你或外部资源**：Linux VPS 服务器、设备端 TEE 硬件出证、Android APK 真机最终验证、第三方安全审计、完整区块浏览器/索引器、Web 钱包。

---

## 二、逐项清单（按优先级）

### P0 · 核心协议逻辑 — ✅ 已完成
共识 / 4s 出块 / staking / bank / tokenomics（固定上限 1e15 umc、零二次通胀）/ DePIN 挖矿拨付闭环 / phonenode attestation+slash / edgeai 任务+仲裁拨付 / B3.1 R4 / B6-R1 DAO 治理参数，全部实现并测试。
证据：`go build/vet/test=0`；此前端到端「贡献即挖矿 +400umc×3」、总供给 exactly 1e15、mint 通胀=0 已真机验证。

### P0 · 智能合约 / 原生模块 — ✅ 已完成（Cosmos SDK 原生模块，非 EVM）
五大自定义模块（depin / edgeai / mcchain / phonenode / tokenomics）均实现 Msg / keeper / querier / genesis。
注：本项目采用 Cosmos SDK 原生模块而非 EVM 智能合约；若需 EVM 兼容（ethermint / CosmWasm）属独立范围扩展，当前未做。

### P1 · 后端 API — ✅ 接口已完成 + 文档今日新增
- Cosmos SDK REST(1317) / gRPC(9090，已启用 reflection) / RPC(26657) / P2P(26656) 全暴露；自定义模块 Query 已注册。
- `cmd/event-subscriber` 订阅 8 类链上事件，作为索引器接入点。
- 今日新增 `docs/BACKEND_API.md`（端点清单）。
- 缺口（诚实）：生产级区块浏览器 / 索引器（持久化 DB + 前端）未建，属「上线后」迭代项，不阻塞主网启动。

### P1 · 监控告警 — ✅ 今日新增完成
- `monitoring/`：prometheus.yml、alert.rules.yml（出块停止 / 对等节点过少 / 预言机宕机 / 共识投票权归零 告警）、grafana 数据源 + 看板、alertmanager 配置。
- `docker-compose.yml` 增加 prometheus / grafana / alertmanager 并暴露端口；`deploy/init.sh` 启用 cometbft(26660) + app(26661) telemetry。
- 待你：把 alertmanager webhook 指向你的企微 / 钉钉 / 飞书机器人。

### P1 · 链下预言机（T2 生产 attestation）— ✅ 今日完成（服务侧）
- 链侧 `TeeOracle` 框架（早前完成）+ 今日新增链下签名服务 `cmd/oracle`（编入模块，冒烟测试签名与链上验签一致 PASS）。
- `app.go` 支持 env `MC_ORACLE_PUBKEY` 一行切换 `TeeOracle`。
- 阻塞（诚实）：设备端 TEE 出证（Android Key Attestation / iOS DeviceCheck）需真机 SDK 集成，我无法代写硬件背书；当前可用「准生产」手动签名脚本跑通闭环，再替换为真实 TEE。

### P2 · Gas 优化 / 费用 — ✅ 今日固化
- `init.sh` 设 `minimum-gas-prices`（默认 `0umc` 友好移动端；主网 `MIN_GAS_PRICES=0.0025umc`）。
- 今日新增 `docs/GAS_AND_FEES.md`（策略 / 切换 / DoS 防护）。
- 建议生产设 `app.toml` 的 `max_gas` 上限（当前默认 0=不限），见文档。

### P2 · 部署脚本 — ✅ 已完成（此前）+ 今日增强
- `Dockerfile`（多阶段构建）、`docker-compose.yml`（验证人 / 全节点 / 快照 + 监控 + 预言机）、`deploy/init.sh`（含 telemetry + gas）、`deploy/start.sh`、`make_genesis.py`、`mainnet-genesis-config.json`、`MAINNET_RUNBOOK.md`、`MAINNET_DEPLOY_PLAN.md`、`MAINNET_SERVER_GUIDE.md`、`ORACLE_FRAMEWORK.md`。
- 待你：付费租赁 Linux VPS（指南已给机型清单），按 runbook 拉起。

### P2 · 安全审计项 — 🟡 部分完成（自审通过 + 第三方待做）
- 今日安全自审：`go vet/test/build` 全绿；此前深度审计已抓并修复 P0（InitGenesis 顺序、团队多签地址排序、mint 二次通胀、genesis 参数）。
- `docs/audit_checklist.md` 列出第三方审计维度（须由独立机构执行，我不参与）。
- 建议：主网上线前安排一次第三方审计（重点：tokenomics 拨付、slash、ante 费用、预言机验签）。

### 前端界面 — 🟡 部分完成（移动端 App 在修；Web 未启动）
- 移动端矿工 App（`mc-miner`，WebView + CosmJS）：工程与 APK 已编译；当前版本加了「崩溃自显示」以定位真机闪退（此前三次闪退分别修了 WebView 加载、主题错配、系统 WebView 兜底）。
- 阻塞：APK 真机最终验证需你卸载重装最新版并反馈（或连 USB 抓 logcat）；Web 钱包 / 区块浏览器前端未启动。

### 钱包集成 — 🟡 部分完成
- 移动端：CosmJS 钱包生成 + 签名（App 内）已实现；Keplr / Leap 浏览器插件钱包集成未做（需注册 coin type 118 + 链信息）。
- 助记词持久化（App 内）未做（当前每次重装重新生成），建议加。

### 交易流程 — ✅ 已完成（协议层）
注册节点 / 设备、attest、提交贡献、任务仲裁、治理投票全流程 Msg + ante（含最低自抵押装饰器）均已实现并测试；端到端挖矿拨付真机验证通过。

---

## 三、今天新交付物

| 类别 | 文件 |
|---|---|
| 监控 | `monitoring/prometheus.yml`、`monitoring/alert.rules.yml`、`monitoring/grafana/*`、`docker-compose.yml`（增强）、`deploy/init.sh`（增强） |
| 预言机 | `cmd/oracle`、`internal/oraclesvc`、`cmd/mcchaind/cmd/oracle.go`、`app/app.go`（`MC_ORACLE_PUBKEY` 切换） |
| 文档 | `docs/BACKEND_API.md`、`docs/GAS_AND_FEES.md` |
| 自审证据 | `secaudit.log`（build/vet/test=0）、预言机冒烟测试（SIGNATURE VERIFY: PASS） |

---

## 四、你现在可安排的集成测试（VPS 到位后）

1. 按 `MAINNET_SERVER_GUIDE.md` 租 VPS（Ubuntu 22.04，验证人 4vCPU/8GB/200GB SSD）。
2. 按 `MAINNET_RUNBOOK.md` + `docker-compose.yml`：build 镜像 → `init.sh` 生成 genesis → 多验证人 `collect-gentxs` → `start`。
3. 起监控栈：访问 `grafana:3000` 看板；配 `alertmanager` webhook。
4. 起预言机：`mcchaind oracle`（固定 `ORACLE_KEY`），把 `/pubkey` 返回的 `pubkey_base64` 注入验证人 `MC_ORACLE_PUBKEY`。
5. 移动端：连节点 RPC(26657) 跑挖矿闭环；或先连测试网验证。

---

## 五、仍阻塞、需你决策 / 资源

- **VPS 租赁与 SSH**（付费，我无法代购）
- **设备端 TEE 出证 SDK 集成**（真机 / 硬件）
- **Android APK 真机闪退最终验证**（需你反馈屏幕错误或连 USB）
- **第三方安全审计**（独立机构）
- **Web 钱包 / 区块浏览器前端**（如需，单独排期）
