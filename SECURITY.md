# 安全政策（Security Policy）

MC 公链是承载价值的公链，**安全漏洞请私下披露，勿在公开 Issue 讨论**。

## 报告方式

- **GitHub Private Vulnerability Reporting**：进入仓库 `Security` → `Report a vulnerability`
- 或邮件联系核心团队（地址见 [CONTRIBUTING.md](./CONTRIBUTING.md)）

## 响应时效

- 首次确认：72 小时内
- 修复 / 缓解：按严重级别排期，高危优先

## 覆盖范围

- 共识层与出块逻辑
- 代币经济（`x/tokenomics`）、铸币与分配
- 签名 / 密钥管理、硬件 attestation（`x/phonenode`）
- 跨链（IBC）、预言机、前端 `web/`
- 依赖供应链（`go.mod`）

## 奖励

- 目前未设公开 bug bounty；重大漏洞报告者将在治理记录中获得公开致谢。

## 禁止

- 勿在公开渠道泄露未修复漏洞的利用细节。
- 勿对主网进行未授权攻击测试。
