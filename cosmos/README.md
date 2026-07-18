# MobileChain 生产链（B 线 / CometBFT）模块层

本目录是 MobileChain **生产级公链**的逻辑层（B 线）。A 线（教学链原型）
是从零手写的 Go 教学链（Phase 1–7 已验证），本目录在其之上抽取出**可直接上 CometBFT / Cosmos SDK
的生产模块**，并设计为：

- **零网络依赖**：当前用纯 Go 标准库 + 内存存储实现，`go build` / `go test` 在本机离线即可跑通；
- **接口对齐 Cosmos SDK**：每个模块的 Keeper 方法签名与 Cosmos SDK `Keeper` 一致，
  未来只需把底层 store 换成 `collections.Store` / `KVStore`，即可无缝升级为真实链上模块；
- **复用已验证经济逻辑**：通过 `go.mod` 的 `replace` 直接引用 `mcchain-staging`
  （DePIN 奖励引擎 + 国库），不重复造轮子、不做假。

## 目录结构

```
cosmos/
├── go.mod                 # 模块 mcchain-cosmos，replace 指向 ../mcchain_staging
├── x/
│   ├── depin/             # DePIN + 边缘 AI 模块（x/depin 蓝本）
│   │   ├── types.go       # 类型与错误定义
│   │   ├── keeper.go      # Keeper：设备注册 / 贡献验证 / 奖励发放
│   │   └── keeper_test.go # 单元测试（奖励引擎 / 阈值 / 封顶 / 错误路径）
│   └── phonenode/         # 手机轻节点模块（x/phonenode 蓝本）
│       ├── types.go       # 节点角色（light/full）/ 状态
│       ├── keeper.go      # 节点注册 / Merkle 证明验证 / 状态剪枝
│       └── keeper_test.go # Merkle 证明正确性 / 篡改检测 / 轻节点生命周期
└── app/
    ├── app.go             # 逻辑总装：持有 Keeper，模拟出块（交易池→Commit→mint）
    └── app_test.go        # 集成测试：DePIN 出块闭环 + 轻节点证明
```

## 核心设计

### 1. DePIN 模块（`x/depin`）
- 复用 `mcchain-staging/depin` 的奖励引擎：`inference 5x / data_label 3x / bandwidth 1x`，
  质量分阈值 30，单任务奖励封顶 500 MC；
- `SubmitAndReward(taskID, addr, taskType, score)`：校验 → 计算 → 入账，返回实际奖励；
- 拒绝路径明确：非法类型、越界分数、重复 taskID、未知设备均返回对应错误，不静默吞掉。

### 2. 手机轻节点模块（`x/phonenode`）
这是 MobileChain 的差异化核心——手机**不存全状态**，而是作为轻节点：
- `RegisterNode(addr, model, os, role)`：角色为 `light`（主流）或 `full`；
- `SubmitStateProof(addr, root, leaf, index, proof)`：手机提交 Merkle 证明，
  向全节点验证“我的余额/贡献记录确实在链上状态根里”；验证失败返回 `ErrBadProof`
  并累计失败次数（用于女巫/作恶风控）；
- `MarkPruned(addr)`：手机对本地已同步状态做剪枝，省存储——这是手机能当节点的关键；
- 内置 `VerifyMerkleProof` / `BuildMerkleRoot` / `LeafHash`，与 CometBFT 状态证明语义一致。

### 3. 逻辑总装（`app`）
`App` 持有所有 Keeper，模拟最小出块周期：
- `SubmitContribution(...)` 把贡献放入交易池；
- `Commit()` 模拟出块：处理交易池全部贡献 → 计算奖励 → 累计 `Minted` 总量 → 链高 +1；
- 失败贡献进入失败收据并跳过（不中断出块，与真实链一致）；
- `SubmitLightProof(...)` 让轻节点提交状态证明。

## 运行测试

```bash
# 需要本机 Go（go_root 已内置在项目中）

cd cosmos
go build ./...
go test ./...      # app + depin + phonenode 全部通过
go vet ./...       # 静态检查无告警
```

## 升级为真实 CometBFT 链（有网络时）

本逻辑层是“可上链”的蓝本。获得网络后，用 Ignite CLI 生成真实应用骨架，
再把本目录的 Keeper 逻辑搬进 Cosmos SDK 模块：

```bash
# 1) 用 Ignite 生成链骨架（需网络拉取 cosmos-sdk）
ignite scaffold chain mcchain --address-prefix mc

# 2) 生成自定义模块（对应本目录的 x/depin、x/phonenode）
ignite scaffold module depin --dep module/token
ignite scaffold module phonenode

# 3) 生成消息类型（示例）
ignite scaffold message register-device address model os
ignite scaffold message submit-contribution task-id task-type score
ignite scaffold message submit-state-proof root leaf index proof

# 4) 把 cosmos/x/depin/keeper.go 的业务方法迁移进
#    x/depin/keeper/keeper.go，底层 store 换成 collections.Store
#    经济常量/奖励引擎直接复用 mcchain-staging（已验证，无需重写）
```

> 说明：当前沙箱无外网，`cosmos-sdk` 未缓存，故未执行 `ignite scaffold`
> （避免生成半成品）。本逻辑层已独立编译与测试通过，是真实上链前的
> “业务逻辑冻结版”——上链时只需换存储后端，经济与共识语义零改动。

## 红线（与 A 线一致）

- 不抄袭、代码全程公开可审计；
- 经济参数（奖励系数/阈值/封顶/国库上限）全部写在代码与文档中；
- 不做拉人头 / 私域引流 / 虚假宣传。
