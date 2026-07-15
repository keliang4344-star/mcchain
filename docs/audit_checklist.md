# MobileChain 第三方审计清单（B6-R4）

> 必由第三方审计机构执行。本文档仅列审计维度、交付物与通过标准，供审计方使用。
> 沙箱不参与审计；验收以第三方报告为准。

## 维度一 · 经济模型审计
- [ ] 总量不变量：`minted_supply ≤ total_supply_cap`（1e15 umc）恒成立，crisis 每轮校验。
- [ ] 三大池分配占比 = team 1500 / community 3500 / ecosystem 5000（合计 10000），拨付额与余额一致。
- [ ] DePIN 初始池 1e14 umc 由 tokenomics 生态池 genesis 一次性拨付，运行期仅拨付不增发。
- [ ] 团队池 vesting 释放曲线（1y+3y）正确，无提前释放。
- [ ] 贡献即挖矿奖励公式 `reward = score × rate`（capped 500，threshold 30）无整数溢出/除零。
- 交付物：经济模型形式化说明 + 数值测试报告。通过标准：上述不变量在边界输入下均成立。

## 维度二 · 安全审计（attestation / slashing / anti-cheat）
- [ ] `attest-device` 软认证边界：主网须替换为真实设备证明（T2 预言机），当前软认证不得上主网信任核心。
- [ ] `DetectOffline` 宽限逻辑：确认 `LastProofBlock` 初始化与 100 区块宽限正确，无瞬时误 slash（已修 bug 复测）。
- [ ] `SlashIfBad` 仅离线/作恶触发，金额计算正确，无负余额。
- [ ] `min_self_delegation` 下限 3e10 umc 在 ante + InitChainer 双重强制。
- [ ] 模块账户权限（Minter 仅 tokenomics，depin 无 Minter）正确。
- 交付物：威胁模型 + 攻击面清单 + 修复确认。通过标准：高危/中危 0 未决。

## 维度三 · 代码审计
- [ ] 依赖扫描（go mod + 已知 CVE）无高危。
- [ ] 所有 externaI 入口（msg_server）做权限/参数校验，无越权（如任意账户代扣）。
- [ ] 整数/溢出、panic recovery、gas 计量合理。
- [ ] 升级（BeginBlock/EndBlock）无状态破坏，升级高度 halt 正确。
- 交付物：静态分析 + 人工 review 报告。通过标准：严重/高危 0，中危有修复计划。

## 维度四 · 渗透与运维审计
- [ ] RPC/API/p2p 端口暴露面评估，默认最小开放。
- [ ] 密钥管理（keyring / HSM / tmkms）流程审计。
- [ ] 部署镜像（Dockerfile）无硬编码密钥、非 root 运行建议。
- [ ] 监控/告警/快照/灾备流程评审。
- 交付物：渗透测试报告 + 运维基线确认。通过标准：无未授权访问路径。

## 维度五 · 文档评审
- [ ] PRD B1–B6 与实现一致。
- [ ] 本 runbook / 部署方案可照做复现主网。
- [ ] 经济参数与 genesis 一致。
- 通过标准：文档与代码偏差 0。
