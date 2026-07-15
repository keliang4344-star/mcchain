# MobileChain B3 批次 · EdgeAI（attested execution + 任务/结果/anti-cheat，关联 depin）· 增量 PRD

**文档类型**：增量 PRD（基于现有 `mcchain` 代码，仅描述变更，不含实现代码）
**批次**：B3（EdgeAI）— 归属路线图「阶段四 经济模型与安全」延伸的移动端贡献即挖矿
**作者**：许清楚（Xu），Product Manager
**语言**：简体中文
**验收总原则**：沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令；验收一律以用户本机 `ignite chain build` + `go test` + 链上观测（`mcchaind q ...`、事件监听）为准。

**产品目标（一句话）**：新增 `x/edgeai` 模块，将移动端「执行 AI 任务」固化为链上 attested execution——任务可发布、结果带硬件证明、作弊可检测/可罚，并与 B1 经济模型、B2 安全、depin 生态池打通。

## 0. 背景现状（已侦察，采信）
- B1 经济模型（`x/tokenomics` cap/分配/释放/查询）已落地；B2 安全（attestation/slashing）提供反女巫与惩罚底座。
- `x/depin` 已具备 DePIN 池拨付与 phonenode 关联；`x/phonenode` 移动节点（B2 加 attestation）。
- 当前无 `x/edgeai` 模块（需新增）；移动端「贡献即挖矿」尚无任务/结果/anti-cheat 的链上表达。
- 路线图书面方向：主流 DePIN 方向 + 移动端贡献即挖矿。

## 1. 增量范围说明（基于现有 mcchain，仅列变更）
- **新增模块**：`x/edgeai`（tasks / results / attestation 校验 / anti-cheat / 争议）承载 attested execution 全流程。
- **关联 depin**：通过 depin keeper 触发生态池拨付（B1 记账、受 cap 约束）。
- **关联 phonenode/B2**：结果提交须查询 phonenode attestation 状态（B2）作为前置。
- **genesis 调整**：edgeai 模块初始化（空任务集、参数：争议期、anti-cheat 阈值）。
- **查询/事件**：`mcchaind q edgeai task|result|dispute`；任务/结果/争议事件上链。
- **不变**：B1 经济模型语义、B2 安全机制；客户端/SDK/浏览器链下部分（B5）不在本批次实现。

## 2. 需求池（P0–P2）

### R1 · 任务生命周期（P0 / Must have）
- **需求描述**：任务发布（算力/数据需求描述 + 奖励额度）、节点领取（assign）、执行、结果提交、完成/争议状态机。
- **验收标准**：
  - `mcchaind q edgeai task <id>` 可见状态（open/assigned/done/disputed）；状态转换合法；重复领取被拒。
  - 任务奖励额度受 B1 生态池约束（不从 cap 外增发）。
- **关键约束**：任务与 B3 edgeai 业务强相关；奖励经 depin 拨付，复用 B1 单一发行入口。

### R2 · Attested execution 证明（P0 / Must have）
- **需求描述**：结果提交须附 B2 硬件 attestation + 执行证明（证明任务在真实移动设备执行）；链上校验证明根/nonce 通过且对应 phonenode 已 attest。
- **验收标准**：
  - 无有效 attestation 的结果提交被拒；`mcchaind q edgeai result <id>` 显示证明状态；与 phonenode attestation 状态联动。
- **关键约束**：链上只存证明根/哈希；复用 B2 attestation 状态，不重复实现验证。

### R3 · Anti-cheat 验证（P0 / Must have）
- **需求描述**：检测重复提交、伪造结果、女巫批量作弊；可疑结果进入 dispute/挑战期，由其他节点复算或仲裁；确认作弊触发 slash（联动 B2）并拒付。
- **验收标准**：
  - 作弊提交被标记/拒付/触发 slash 事件；诚实节点结果正常通过；争议期结束后定稿。
  - anti-cheat 阈值参数化（可由 B2 最小治理调整）；与 B1 cap 不冲突。
- **关键约束**：争议/挑战机制须与 B2 slash 衔接——作恶挑战者被反 slash，诚实挑战者获奖励。

### R4 · 奖励联动 depin（P1 / Should have）
- **需求描述**：通过验证（且过争议期）的任务结果触发 depin 生态池拨付给执行节点，受 B1 tokenomics cap 约束。
- **验收标准**：
  - 拨付计入生态池、`minted_supply` 累计不超 cap；`mcchaind q tokenomics allocations` 可见生态池变化。
- **关键约束**：复用 B1 单一发行入口与记账；edgeai 不直接 mint，经 depin 拨付函数。

### R5 · 可观测与 SDK 接口（P2 / Nice to have）
- **需求描述**：任务/结果/争议事件上链，供 B5 SDK/浏览器消费；暴露聚合查询。
- **验收标准**：事件可订阅；查询覆盖生态工具所需字段。

## 3. 关键设计建议（给架构师）
1. **模块边界**：`x/edgeai` 独立模块，依赖 `phonenode`（attestation 查询）、`depin`（奖励拨付）、`tokenomics`（cap 记账，经 depin 间接）。
2. **验证模型**：链上存 attestation 根 + 结果哈希；执行重算/验证采用 optimistic——结果先接受，争议期内可被挑战复算；挑战由质押节点进行（联动 B2 slash 作恶挑战者）。
3. **与 B2 衔接**：结果提交路径查询 `phonenodeKeeper.IsAttested(addr)`；attestation 无效直接拒。
4. **与 B1 衔接**：奖励经 `depinKeeper` 的拨付函数（已存在），该函数内部受 tokenomics 记账；edgeai 不直接 mint。
5. **genesis 顺序**：`x/edgeai` 在 `depin`、`phonenode` 之后 InitGenesis（依赖两者 keeper）。
6. **防作弊阈值**：anti-cheat 参数放入 edgeai params，可由 B2 治理调整。

## 4. 文件清单（新增/修改）
- 新增：`x/edgeai/`（types: params/genesis/task/result/msg/query；keeper: 任务机/attestation 校验/anti-cheat/dispute + 依赖 phonenode/depin keeper；genesis；msg_server；module；client/cli + query；proto）。
- 修改：`app/app.go`（ModuleBasics、store key、keeper 装配含 phonenode/depin 依赖、mm 模块列表、Begin/End/InitGenesis 顺序、subspace）、`x/depin`（暴露/复用拨付函数）、`x/phonenode`（暴露 `IsAttested` 查询）、`config.yml`/genesis（edgeai 参数）。
- 文档：经济模型/edgeai 接入说明（供 B5）。

## 5. 验收总原则
- 沙箱无法运行 `go`/`protoc`，本 PRD 不写代码、不执行命令。
- 验收一律以用户本机 `ignite chain build` + `go test`（任务机/anti-cheat 模拟测试）+ 链上观测（`mcchaind q edgeai ...`、`mcchaind q tokenomics ...`、事件监听）为准。
- 硬验收：奖励拨付累计 `minted_supply <= total_supply_cap`；无 attestation 的结果必被拒；作弊必触发 slash 事件。
