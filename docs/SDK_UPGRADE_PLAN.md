# Cosmos SDK 升级规划（0.47 → 0.50 LTS）

> 状态：规划中（未排期实施）。本文档是独立大版本工程的路线图，与主网治理提案解耦。
> 升级属于**跨版本状态机迁移**，必须在测试网完整验证后放行，禁止直接在主网原地升级。

## 1. 目标版本矩阵

| 组件 | 当前 | 目标 | 说明 |
|---|---|---|---|
| Cosmos SDK | v0.47.14 | v0.50.x（LTS 分支） | 0.50 为 LTS，长期维护 |
| CometBFT | v0.37.6 | v0.38.x | SDK 0.50 配套 |
| IBC-Go | v7.3.0 | v8.x | SDK 0.50 配套 |
| wasmd | v0.45.0 | 按 SDK 0.50 兼容版本 | 若保留 CosmWasm 集成 |
| Go | 1.22.5 | 按 SDK 0.50 要求（通常 ≥1.21） | 保持 CI/本地一致 |

> 版本漂移由 `docs/VERSIONS.md` 作为单一权威来源登记，升级后必须同步更新。

## 2. 为什么要升级

- 0.47 已进入维护期，新特性与安全修复集中在 0.50 LTS。
- 0.50 引入更清晰的模块 API（`appmodule`、`module.HasGenesis` 等）、更好的 gas 记账、
  baseapp 结构简化，长期降低维护成本。
- 生态依赖（IBC、wasmd、第三方模块）逐步以 0.50 为基线。

## 3. 主要变更点（已知影响面）

| 变更域 | 影响 | 应对 |
|---|---|---|
| `app.go` 结构 | 模块管理方式调整（`module.Manager` 生命周期 API 变化） | 按 0.50 迁移指南重写 app 组装，保持模块清单等价 |
| proto 生成 | `buf` 生成产物与 `google.golang.org/protobuf` 版本对齐 | 重新跑 `make proto-gen`，提交生成物 diff 审查 |
| 状态存储迁移 | 部分模块 store key / codec 注册变化 | 走 `StoreUpgrades`，测试网从创世高度做升级演练 |
| 查询/事件接口 | REST/gRPC 路由注解与事件命名可能微调 | 以 `docs/static/openapi.yml` 与 `docs/sdk_event_contract.md` 为基准回归 |
| 依赖版本 | ibc-go v8、cometbft 0.38 API 破坏性变化 | 逐模块编译修复 + 单测回归 |

## 4. 升级步骤（建议顺序）

1. **分支与基线**：从当前 `main` 切 `release/sdk-v0.50`，锁定 go.mod 目标版本。
2. **依赖升级**：先升级 CometBFT 0.38 → IBC-Go v8 → SDK 0.50，逐层 `go build` 验证。
3. **app 重构**：按 SDK 0.50 迁移指南重构 `app/app.go` 模块组装与 lifecycle。
4. **proto 重生成**：`make proto-gen`，提交生成物，diff 审查事件/路由契约。
5. **状态迁移**：实现 `StoreUpgrades`；在 devnet 上从 0.47 高度快照 → 升级 → 校验状态哈希。
6. **测试矩阵**：全模块单测 + `tests/e2e/smoke.sh` 冒烟 + 既有 `mainnet_readiness.sh` 门禁。
7. **文档对齐**：更新 `docs/VERSIONS.md`、README、DEPLOYMENT_GUIDE、sdk_event_contract、
   mobile_sdk_integration（由 `scripts/check_docs_consistency.py` 强制校验漂移）。
8. **放行**：测试网完整运行 ≥1 周（覆盖跨版本升级与长稳），治理提案确认主网升级高度。

## 5. 验收门禁（完成定义）

- [ ] `go build ./...` 零错误
- [ ] 全模块单测通过（含升级迁移测试）
- [ ] `tests/e2e/smoke.sh` 全绿（init/出块/CLI/staking/REST/gRPC/转账）
- [ ] `scripts/check_docs_consistency.py` 通过
- [ ] devnet 完成一次 0.47 → 0.50 状态迁移演练
- [ ] 事件文档（`docs/generated/events.md`）与线上行为一致

## 6. 风险与回退

- **最大风险**：状态迁移或行为差异引入分叉。缓解：升级前做高度级状态哈希比对，
  测试网长稳 ≥1 周。
- **回退**：保留 0.47 二进制与快照，若升级后 48h 内出现未预期行为，回滚到升级前快照
  并重新评估。升级窗口应选择低活跃期，并提前公告验证人升级高度。
