# MobileChain (MC)

> A Public Chain That Puts a Full Node in Every Phone  
> **一条把全节点装进每一部手机的公链**

[![Cosmos SDK](https://img.shields.io/badge/Cosmos_SDK-v0.47.14-blue?logo=cosmos)](https://github.com/cosmos/cosmos-sdk)
[![CometBFT](https://img.shields.io/badge/CometBFT-v0.37.6-purple)](https://github.com/cometbft/cometbft)
[![Go](https://img.shields.io/badge/Go-1.22.5-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-green)](./LICENSE)

MC is a DePIN + Edge AI public chain built on **Cosmos SDK + CometBFT**. Its core innovation is enabling smartphones to participate in consensus and contribution as "light full nodes," addressing the node centralization problem in current public chains. On-chain economics are driven by eight native modules, with a fixed total supply of 1 billion MC and zero inflation.

**Open Source & Auditable · Parameters in Code · Truth on Chain · Consensus Symbiosis**

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  app (Application Layer)             │
├──────────┬──────────┬──────────┬──────────┬─────────┤
│ mcchain  │tokenomics│  depin   │phonenode │ edgeai  │
│ (params) │ (mint)   │(incentive)│(mobile) │(AI mkt) │
├──────────┴──────────┴──────────┴──────────┴─────────┤
│           Cosmos SDK Standard Modules                │
│  (bank / staking / gov / ibc / auth / crisis...)     │
├─────────────────────────────────────────────────────┤
│               CometBFT Consensus Engine              │
└─────────────────────────────────────────────────────┘
```

## Custom Modules

| Module | Responsibility | Key Features |
|--------|---------------|--------------|
| `x/tokenomics` | Token issuance & allocation ledger | Sole Minter, locked 1B MC supply, five-pool allocation (Device 55% / Staking Security 15% / Team 12% / Foundation 13% / Early Dev 5%) |
| `x/depin` | Device contribution incentive engine | Device registration, contribution metering, reward distribution gateway |
| `x/phonenode` | Mobile full node management | Hardware attestation, heartbeat detection, offline slashing |
| `x/edgeai` | Edge AI task marketplace | Task creation/submission, dispute arbitration, contribute-to-mine |
| `x/mcchain` | Chain-level parameter management | System configuration, query interface |
| `x/dex` | Native AMM exchange | Constant product market maker (x×y=k), pool/swap/liquidity |
| `x/referral` | Referral incentive | On-chain referral graph, 10-generation rewards (first 3 tiers governable), paced budget release |
| `x/liquidstaking` | Liquid staking | Staking share tokenization, exchange rate distinguishes empty pool vs slashed-to-zero |

## Supporting Tools

| Project | Description |
|---------|-------------|
| `mc-miner/` | Android mining App, WebView + CosmJS, local mnemonic generation |
| `cosmjs-bundle/` | Frontend CosmJS v0.32.4 UMD Bundle |
| `web/` | Web console (miner dashboard / tx builder) |
| `cosmjs/` | Lightweight tx builder `mc-tx-builder.js` and type definitions |
| `typescript-sdk/` | TypeScript SDK (address / bech32 / client) |
| `monitoring/` | Prometheus + Alertmanager + Grafana configs |
| `deploy/` | systemd units / nginx / env templates / deploy scripts |
| `scripts/` | Genesis generation, validation, stress test, backup scripts |

## Key Parameters

| Parameter | Value |
|-----------|-------|
| Chain ID | `mcchain-mainnet-1` |
| Native Token | MC (smallest unit: umc, 1 MC = 10⁶ umc, precision 6) |
| Total Supply | 1 billion MC (10¹⁵ umc) |
| Inflation | Zero (supply permanently locked) |
| Consensus | CometBFT BFT |
| IBC | ibc-go v7.3.0 |
| Min. Validator Self-Delegation | 30,000 MC |

## Quick Start

```bash
# Prerequisites: Go 1.22+
git clone https://github.com/keliang4344-star/mcchain.git
cd mcchain
go build ./...                          # full build (no Makefile in repo)
go build -o build/mcchaind ./cmd/mcchaind   # build the node binary

# Local single node
mcchaind init mynode --chain-id mcchain-1
mcchaind keys add alice --keyring-backend test
# ... configure genesis then start
mcchaind start
```

> See [DEVELOPMENT.md](./DEVELOPMENT.md) for detailed dev environment setup.

## Documentation

| Document | Description |
|----------|-------------|
| [Whitepaper](./WHITEPAPER.md) | The English edition, corresponding to the Chinese edition chapter by chapter |
| [Whitepaper · Print Edition](./docs/MobileChain-Whitepaper-Collector-Edition.pdf) | English collector's edition, A4, with bookmarks |
| [Whitepaper · Chinese](./WHITEPAPER_CN.md) | The Chinese original, from which this translation is made |
| [Whitepaper · Chinese Print Edition](./docs/MobileChain白皮书_完整典藏版.pdf) | Chinese collector's edition, A4, with bookmarks |
| [Token Allocation](./docs/TOKEN_ALLOCATION.md) | Total supply, pools, unlocking rules |
| [Protocol Parameters](./docs/PROTOCOL_PARAMS.md) | Authoritative reference for block, economic and governance parameters |
| [DAO Roadmap](./docs/dao_roadmap.md) | Phased decentralized governance plan |
| [Beginner Guide](./BEGINNER_GUIDE.md) | Cloud server one-click setup tutorial |

## Testing

```bash
go test ./...
```

Module test cases: depin 36 · dex 48 · edgeai 81 · liquidstaking 18 · mcchain 15 · phonenode 48 · referral 30 · tokenomics 31 · app 10 · internal 22 — the total keeps growing with new tests (see CI run logs).

Key modules target ≥ 70% coverage (CI gate: `.github/workflows/ci.yml`).

## Community

- **Twitter / X**: [@MC_MobileChain](https://twitter.com/MC_MobileChain) (placeholder)
- **Discord**: [discord.gg/mcchain](https://discord.gg/mcchain) (placeholder)
- **GitHub Issues**: [Issues](https://github.com/keliang4344-star/mcchain/issues)

## Contributing

Issues and Pull Requests are welcome. Please read before participating:

- [CONTRIBUTING.md](./CONTRIBUTING.md) — Code standards and submission process

## License

[Apache License 2.0](./LICENSE)
