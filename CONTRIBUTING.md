# 贡献指南

感谢你对 MobileChain (MC) 的关注。本文件说明如何参与代码贡献、提 Issue 和 Pull Request。

## 行为准则

- 对事不对人，讨论聚焦于技术和代码
- 尊重不同意见，不人身攻击
- 保持中文或英文，不混用
- 所有贡献需遵循 Apache 2.0 许可证

## 我如何参与？

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

## 许可证

所有贡献在 Apache License 2.0 下授权。提交代码即表示你同意此条款。
