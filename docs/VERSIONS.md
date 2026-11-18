# MobileChain 版本与依赖矩阵（单一权威来源）

> 本文档是全仓库版本信息的**唯一权威来源**。所有文档中的版本描述（DEVELOPMENT.md、sdk_event_contract.md、mobile_sdk_integration.md 等）必须与此一致；
> 由 `scripts/check_docs_consistency.py` 在 CI 中比对，任何漂移会直接导致 CI 失败。

## 1. 链标识

| 项 | 值 | 说明 |
|---|---|---|
| 主网链 ID | `mcchain-mainnet-1` | 创世 `chain_id` 固定 |
| 测试网链 ID | `mcchain-testnet-1` | devnet 脚本默认 |
| denom | `umc` | 链上唯一计价单位，`1 MC = 1e6 umc` |
| 地址前缀 | `mc` | bech32 前缀 |

## 2. 依赖版本（与 go.mod 严格一致）

| 依赖 | 版本 | 说明 |
|---|---|---|
| Go | 1.22.5（最低 1.22.0） | `go.mod` 声明最低版本，CI 固定 1.22.5 |
| Cosmos SDK | v0.47.14 | `github.com/cosmos/cosmos-sdk` |
| CometBFT | v0.37.6 | cometbft |
| IBC-Go | v7.3.0 | `github.com/cosmos/ibc-go/v7` |
| wasmd | v0.45.0 | 仅在 CGO + libwasmvm 可链接的构建中启用（见下） |
| gogoproto | v1.7.0 | proto 代码生成 |
| cosmos-proto | v1.0.0-beta.5 | google.api/http 注解依赖 |

> 构建矩阵：`wasm` 模块通过 `wasm_setup_{cgo,nocgo}.go` 条件编译。默认 `CGO_ENABLED=0`
> 的构建**不含** CosmWasm；发布 wasm-enabled 版本必须使用 Linux + CGO + libwasmvm，
> 并在 release 说明中标注，两种二进制不可混用。
>
> 升级依赖时：1) 改 `go.mod`；2) 同步更新本文档；3) 用 `bash scripts/generate_proto.sh <module>` 重新生成 proto（唯一入口）。

## 3. 关键协议常量（速查）

| 常量 | 值 | 权威位置 |
|---|---|---|
| 总供应上限 | 1e15 umc = 10 亿 MC | `x/tokenomics/types/keys.go` |
| 五池分配 | 55 / 15 / 12 / 13 / 5（%bps 5500/1500/1200/1300/500） | `x/tokenomics/types/keys.go` + `docs/TOKEN_ALLOCATION.md` |
| 质押滴灌 | 年化 5%（500 bps） | `x/tokenomics/types/keys.go` |
| DEX 费率 | 0.30%（LP 0.15% + 黑洞 0.15%） | `x/dex/types/keys.go` |
| Gas 黑洞 | 7% | `x/tokenomics/types/keys.go` |
| 治理参数 | 押金 10 MC / 投票 48h / quorum 33.4% / threshold 50% / veto 33.4% | `x/gov` params + `docs/PROTOCOL_PARAMS.md` |

## 4. 文档↔版本映射

| 文档 | 应引用版本 |
|---|---|
| DEVELOPMENT.md（proto 生成） | v0.47.14 / v1.7.0 / beta.5 |
| sdk_event_contract.md / mobile_sdk_integration.md | v0.47.14 / v0.37.6 |
| README.md / README_EN.md | IBC v7.3.0 |
