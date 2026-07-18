# MobileChain（MC）

> 一条把全节点装进每一部手机的公链  
> **A Public Chain That Puts a Full Node in Every Phone**

[![Cosmos SDK](https://img.shields.io/badge/Cosmos_SDK-v0.47.14-blue?logo=cosmos)](https://github.com/cosmos/cosmos-sdk)
[![CometBFT](https://img.shields.io/badge/CometBFT-v0.37.6-purple)](https://github.com/cometbft/cometbft)
[![Go](https://img.shields.io/badge/Go-1.22.5-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-green)](./LICENSE)

MC 是基于 **Cosmos SDK + CometBFT** 构建的 DePIN + 边缘 AI 公链。核心创新是让智能手机以「轻全节点」方式参与共识与贡献，解决当前公链节点集中化问题。链上经济由 5 个自定义模块驱动，通证固定总量 10 亿 MC、零通胀。

**开源可审计 · 参数写代码 · 链上求真 · 共识共生**

---

## 架构

```
┌─────────────────────────────────────────────────────┐
│                    app (应用装配层)                   │
├──────────┬──────────┬──────────┬──────────┬─────────┤
│ mcchain  │tokenomics│  depin   │phonenode │ edgeai  │
│ (参数)   │(铸币总账) │(设备激励)│(移动节点)│(AI市场) │
├──────────┴──────────┴──────────┴──────────┴─────────┤
│              Cosmos SDK 标准模块                      │
│  (bank / staking / gov / ibc / auth / crisis...)     │
├─────────────────────────────────────────────────────┤
│                 CometBFT 共识引擎                     │
└─────────────────────────────────────────────────────┘
```

## 自定义模块

| 模块 | 职责 | 关键特性 |
|------|------|---------|
| `x/tokenomics` | 代币发行与分配总账 | 唯一 Minter，固化总量 10 亿 MC，三池分配（团队 15% / 社区 35% / 生态 50%） |
| `x/depin` | 设备贡献激励引擎 | 设备注册、贡献计量、奖励拨付闸口 |
| `x/phonenode` | 移动全节点管理 | 硬件 attestation、心跳检测、离线 slash |
| `x/edgeai` | 边缘 AI 任务市场 | 任务创建/提交/争议仲裁/贡献即挖矿 |
| `x/mcchain` | 链级参数管理 | 系统配置、查询入口 |
| `x/dex` | 原生 AMM 交易所 | 常量积做市商 (x×y=k)，pool/swap/liquidity |

## 配套工具

| 项目 | 说明 |
|------|------|
| `mc-miner/` | Android 挖矿 App，WebView + CosmJS，本地助记词生成 |
| `cosmjs-bundle/` | 前端 CosmJS v0.32.4 UMD Bundle |
| `cosmos/` | Cosmos SDK 离线测试模块 |
| `mc_wp/` | 白皮书构建管线（Markdown → HTML） |
| `mainnet-launch/` | 一键主网启动脚本 |

## 关键参数

| 参数 | 值 |
|------|-----|
| 链 ID | `mcchain-mainnet-1` |
| 主币 | MC（最小单位 umc，1 MC = 10⁶ umc，精度 6） |
| 总量 | 10 亿 MC（10¹⁵ umc） |
| 通胀 | 零（总量永久锁定） |
| 共识 | CometBFT BFT |
| IBC | ibc-go v7.1.0 |

## 快速开始

```bash
# 依赖：Go 1.22+
git clone https://github.com/keliang4344-star/mcchain.git
cd mcchain
make build           # go build ./...
make install         # 安装 mcchaind

# 本地单节点
mcchaind init mynode --chain-id mcchain-1
mcchaind keys add alice --keyring-backend test
# ... 配置创世后启动
mcchaind start
```

> 详细开发环境搭建见 [DEVELOPMENT.md](./DEVELOPMENT.md)。

## 文档

| 文档 | 说明 |
|------|------|
| [白皮书](./docs/WHITEPAPER.md) | MC 公链完整技术与理念 |
| [通证分配](./docs/TOKEN_ALLOCATION.md) | 总量、分配池、解锁规则 |
| [模块白皮书](./docs/MODULE_WHITEPAPER.md) | 各模块完成度与改进路线 |
| [系统设计](./docs/system_design.md) | 架构、数据流、接口 |
| [审计清单](./docs/audit_checklist.md) | 安全审计范围与标准 |
| [主网 Runbook](./docs/MAINNET_RUNBOOK.md) | 上线部署操作手册 |
| [DAO 路线图](./docs/dao_roadmap.md) | 去中心化治理分阶段计划 |
| [新手部署指南](./BEGINNER_GUIDE.md) | 云服务商一键启动教程 |

## 测试

```bash
go test ./...
```

模块测试覆盖：depin (14) · phonenode (7) · tokenomics (~7) · edgeai (17) · mcchain (5) · dex (开发中)

关键模块目标覆盖率 ≥ 70%（CI 门禁见 `.github/workflows/ci.yml`）。

## 社区

- **Twitter / X**: [@MC_MobileChain](https://twitter.com/MC_MobileChain) (placeholder)
- **Discord**: [discord.gg/mcchain](https://discord.gg/mcchain) (placeholder)
- **GitHub Issues**: [github.com/keliang4344-star/mcchain/issues](https://github.com/keliang4344-star/mcchain/issues)

## 贡献

欢迎顶级工程师与社区共同参与 MC 公链建设。参与前请阅读：

- [治理框架 GOVERNANCE.md](./GOVERNANCE.md) — 核心区/开放区、merge 权限、共识层改动流程
- [贡献指南 CONTRIBUTING.md](./CONTRIBUTING.md) — 代码规范与提交流程
- [路线图 ROADMAP.md](./ROADMAP.md) — 阶段里程碑与「Help Wanted」求援模块
- [安全政策 SECURITY.md](./SECURITY.md) — 漏洞私下披露流程
- [行为准则 CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)
- [贡献者协议 CONTRIBUTOR_LICENSE_AGREEMENT.md](./CONTRIBUTOR_LICENSE_AGREEMENT.md)（CLA）
- [审计清单](./docs/audit_checklist.md) — 安全标准

## 许可证

[Apache License 2.0](./LICENSE)
