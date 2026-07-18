# 贡献指南

感谢你对 MobileChain (MC) 的关注。本文件说明如何参与代码贡献、提 Issue 和 Pull Request。

## 行为准则

- 对事不对人，讨论聚焦于技术和代码
- 尊重不同意见，不人身攻击
- 保持中文或英文，不混用
- 所有贡献需遵循 Apache 2.0 许可证

## 我如何参与？

MC 欢迎外部工程师共同开发。重大或共识相关改动，请先阅读 [GOVERNANCE.md](./GOVERNANCE.md) 了解核心区 / 开放区边界与合并权限；想认领任务可看 [ROADMAP.md](./ROADMAP.md) 的「Help Wanted」一节。

### 提 Bug 或功能建议

在 [Issues](https://github.com/keliang4344-star/mcchain/issues) 页面提交，请包含：

- **环境信息**：OS / Go 版本 / 链版本
- **复现步骤**：完整描述，最好附带日志或截图
- **预期 vs 实际**：你期望发生什么，实际发生了什么

### 提交代码（Pull Request）

1. **Fork 本仓库**
2. **创建分支**：`git checkout -b feat/描述` 或 `fix/描述`
3. **写代码**：遵循下方代码规范
4. **写测试**：关键逻辑必须有单测，覆盖率不低于模块当前水平
5. **自检**：
   ```bash
   go test ./...
   go build ./...
   ```
6. **提交**：一个 commit 一件事，消息格式：
   ```
   feat(x/tokenomics): 新增锁定释放核查
   fix(x/depin): 修复奖励计算溢出
   ```
7. **推到你 Fork 的仓库**
8. **发起 PR** 到本仓库 `main` 分支

### PR 审查标准

- 代码逻辑清晰，不引入明显性能问题
- 测试覆盖新增逻辑
- 不破坏现有 API（如有变动需在 PR 描述中说明）
- 无安全风险（不暴露私钥、不引入未审计的依赖）
- 通过 CI 检查

## 代码规范

### Go

- 遵循 [Effective Go](https://go.dev/doc/effective_go) 和 Go 标准库风格
- 包名小写，无下划线
- 导出函数、类型、常量必须写注释
- 错误处理：不使用 `panic`（除 `init()` 外），错误信息小写开头
- 单元测试使用 `testing` 标准库 + `testify/require`（如已在模块中使用）
- 禁止 `go.sum` 手动编辑

### Proto

- `proto/mcchain/<module>/` 下按模块组织
- 字段命名 snake_case，消息 PascalCase
- 如有新增模块 proto，同步提供 `protoc` 生成脚本（见 `scripts/`）

### 文档

- 重大功能变动需同步更新 `docs/` 下对应文档
- 新模块需更新 `docs/MODULE_WHITEPAPER.md`

## 开发环境

见 [DEVELOPMENT.md](./DEVELOPMENT.md)。

## 安全审计

MC 公链遵循「开源可审计·参数写代码」原则。如果你发现安全漏洞，请**不要直接提公开 Issue**，通过 GitHub 的 [Private Vulnerability Reporting](https://github.com/keliang4344-star/mcchain/security/advisories/new) 或邮件联系核心团队。

## Bug Bounty / 漏洞赏金

MobileChain 鼓励安全研究员和白帽黑客参与审计，帮助我们发现并修复潜在安全漏洞。

### 报告渠道

- **GitHub Issues**: 低风险问题可在 [Issues](https://github.com/keliang4344-star/mcchain/issues) 页面公开提交
- **邮箱**: 高风险或敏感漏洞请发送至 **security@mcchain.org** (placeholder)，采用 PGP 加密通信
- **GitHub Security Advisory**: 也可通过 [Private Vulnerability Reporting](https://github.com/keliang4344-star/mcchain/security/advisories/new) 私密提交

### 严重性分级

| 等级 | 定义 | 示例 |
|---|---|---|
| **Critical** | 直接导致资金损失、链停摆或共识破坏 | 代币超额铸造、genesis 篡改、远程 RCE |
| **High** | 严重破坏安全性但利用条件较苛刻 | 签名绕过、状态机死锁、IBC 跨链攻击 |
| **Medium** | 局部安全缺陷，影响范围有限 | 模块间权限越界、未授权查询、DoS 向量 |
| **Low** | 非关键 Bug，不影响核心安全 | 日志泄露调试信息、minor 配置缺陷 |

### 奖励范围

奖励金额根据漏洞严重性和报告质量综合评定（具体金额视基金会预算确定，以下为参考范围）：

- **Critical**: 赏金从优，并以致谢公告形式公开表彰
- **High**: 中等赏金 + 致谢
- **Medium**: 小额赏金或社区认可
- **Low**: 社区致谢（Hall of Fame）

### 规则

1. **先报告后公开**：在修复补丁发布前不得公开漏洞细节
2. **禁止破坏性测试**：不要在主网或公共测试网上进行可能造成实际损失的测试
3. **提供可复现 PoC**：报告应包含环境信息、复现步骤、影响评估
4. **赏金由基金会决定**：最终奖励金额由 MC 基金会根据漏洞实际影响评估

---

## 许可证

所有贡献在 Apache License 2.0 下授权。正式贡献者协议见 [CONTRIBUTOR_LICENSE_AGREEMENT.md](./CONTRIBUTOR_LICENSE_AGREEMENT.md)（CLA）；提交代码即表示你同意该条款（也可用 DCO `Signed-off-by` 签署）。
