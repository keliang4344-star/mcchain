# MobileChain (MC)

> A Public Chain That Puts a Full Node in Every Phone  
> **一条把全节点装进每一部手机的公链**

[![Cosmos SDK](https://img.shields.io/badge/Cosmos_SDK-v0.47.14-blue?logo=cosmos)](https://github.com/cosmos/cosmos-sdk)
[![CometBFT](https://img.shields.io/badge/CometBFT-v0.37.6-purple)](https://github.com/cometbft/cometbft)
[![Go](https://img.shields.io/badge/Go-1.22.5-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-green)](./LICENSE)

MC is a DePIN + Edge AI public chain built on **Cosmos SDK + CometBFT**. Its core innovation is enabling smartphones to participate in consensus and contribution as "light full nodes," addressing the node centralization problem in current public chains. On-chain economics are driven by 6 custom modules, with a fixed total supply of 1 billion MC and zero inflation.

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
| `x/tokenomics` | Token issuance & allocation ledger | Sole Minter, locked 1B MC supply, three-pool allocation (Team 15% / Community 35% / Ecosystem 50%) |
| `x/depin` | Device contribution incentive engine | Device registration, contribution metering, reward distribution gateway |
| `x/phonenode` | Mobile full node management | Hardware attestation, heartbeat detection, offline slashing |
| `x/edgeai` | Edge AI task marketplace | Task creation/submission, dispute arbitration, contribute-to-mine |
| `x/mcchain` | Chain-level parameter management | System configuration, query interface |
| `x/dex` | Native AMM exchange | Constant product market maker (x×y=k), pool/swap/liquidity |

## Supporting Tools

| Project | Description |
|---------|-------------|
| `mc-miner/` | Android mining App, WebView + CosmJS, local mnemonic generation |
| `cosmjs-bundle/` | Frontend CosmJS v0.32.4 UMD Bundle |
| `cosmos/` | Cosmos SDK offline test modules |
| `mc_wp/` | Whitepaper build pipeline (Markdown → HTML) |
| `mainnet-launch/` | One-click mainnet launch scripts |

## Key Parameters

| Parameter | Value |
|-----------|-------|
| Chain ID | `mcchain-mainnet-1` |
| Native Token | MC (smallest unit: umc, 1 MC = 10⁶ umc, precision 6) |
| Total Supply | 1 billion MC (10¹⁵ umc) |
| Inflation | Zero (supply permanently locked) |
| Consensus | CometBFT BFT |
| IBC | ibc-go v7.1.0 |

## Quick Start

```bash
# Prerequisites: Go 1.22+
git clone https://github.com/keliang4344-star/mcchain.git
cd mcchain
make build           # go build ./...
make install         # install mcchaind

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
| [Whitepaper](./docs/WHITEPAPER.md) | Full technical exposition and core philosophy of MC |
| [Token Allocation](./docs/TOKEN_ALLOCATION.md) | Total supply, pools, unlocking rules |
| [Module Whitepaper](./docs/MODULE_WHITEPAPER.md) | Module completion status and improvement roadmap |
| [System Design](./docs/system_design.md) | Architecture, data flow, interfaces |
| [Audit Checklist](./docs/audit_checklist.md) | Security audit scope and standards |
| [Mainnet Runbook](./docs/MAINNET_RUNBOOK.md) | Complete launch operation manual |
| [DAO Roadmap](./docs/dao_roadmap.md) | Phased decentralized governance plan |
| [Beginner Guide](./BEGINNER_GUIDE.md) | Cloud server one-click setup tutorial |

## Testing

```bash
go test ./...
```

Module test coverage: depin (14) · phonenode (7) · tokenomics (~7) · edgeai (17) · mcchain (5) · dex (in development)

Key modules target ≥ 70% coverage (CI gate: `.github/workflows/ci.yml`).

## Community

- **Twitter / X**: [@MC_MobileChain](https://twitter.com/MC_MobileChain) (placeholder)
- **Discord**: [discord.gg/mcchain](https://discord.gg/mcchain) (placeholder)
- **GitHub Issues**: [github.com/keliang4344-star/mcchain/issues](https://github.com/keliang4344-star/mcchain/issues)

## Contributing

Issues and Pull Requests are welcome. Please read before participating:

- [Audit Checklist](./docs/audit_checklist.md) — Security standards
- [CONTRIBUTING.md](./CONTRIBUTING.md) — Code standards and submission process

## License

[Apache License 2.0](./LICENSE)
