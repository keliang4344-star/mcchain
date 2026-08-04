# MobileChain (MC) Whitepaper

**A Public Chain That Puts a Full Node in Every Phone**

> Chain ID `mcchain-mainnet-1` · Native token **MC** (base unit `umc`, 6 decimals) · Fixed supply 1,000,000,000 MC · Zero inflation

**Author:** MC Chain Team
**Version:** 1.0 (Final) · 2026

---

## 1. Abstract

MobileChain (MC) is a DePIN (Decentralized Physical Infrastructure Network) and Edge-AI public chain built on Cosmos SDK and CometBFT. Its core proposition is to let ordinary smartphones participate in consensus and network contribution as lightweight full nodes, addressing the validator-centralization problem common to legacy public chains.

Every economic parameter in this document corresponds to a constant in the source code. All numbers — total supply, distribution ratios, slash fractions, attestation validity — are verifiable on-chain and in code. Where a capability is not yet live on the mainnet client, it is explicitly marked **[Planned]**; this document explains the code, it does not promise what the code has not yet shipped.

The chain has a fixed supply of 1 billion MC and zero inflation. Value is distributed through six on-chain addresses: five allocation pools and a protocol treasury. Returns flow to real contributors — device operators, stakers, and settlement participants — rather than to a pre-mining clique.

---

## 2. Problem and Vision

The platform era monetized user-generated data while excluding users from ownership of that data. MobileChain inverts this: it makes "produce value → be priced → be rewarded" a property of the protocol. A phone that performs real work (attestation, inference, settlement forwarding) earns MC; a holder who stakes secures the network and receives the security drip.

The design goal is to lower the cost of participating in a public chain — from "need a miner, a server, or 100,000 MC of collateral" to "need a phone" — and to lock that lowered cost into code so the resulting power cannot be silently re-centralized.

---

## 3. Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      app (Application Layer)              │
├──────────┬──────────┬──────────┬──────────┬──────────────┤
│ tokenomics│  depin  │ phonenode│  edgeai  │     dex      │
│ (issuance)│(incentive)│(mobile) │(AI mkt)  │  (swap)      │
├──────────┴──────────┴──────────┴──────────┴──────────────┤
│            Cosmos SDK Standard Modules                    │
│      (bank / staking / gov / ibc / auth / crisis)         │
├─────────────────────────────────────────────────────────┤
│                 CometBFT Consensus Engine                 │
└─────────────────────────────────────────────────────────┘
```

Consensus: CometBFT (Tendermint) BFT, ~4-second block time.

| Module | Responsibility |
|--------|---------------|
| `x/tokenomics` | Sole minter; fixed 1B cap; five-pool allocation ledger; staking-security drip; gas rebate/burn; enterprise settlement fee |
| `x/depin` | Device contribution incentive engine: registration, contribution metering, reward distribution |
| `x/phonenode` | Mobile full-node management: hardware attestation, heartbeat, offline/equivocation slashing |
| `x/edgeai` | Edge-AI task marketplace: task creation/submission, dispute arbitration |
| `x/dex` | On-chain swap with a deflationary fee |

---

## 4. The Token

- **Name:** MobileChain, ticker **MC**.
- **Base unit:** `umc` (1 MC = 1,000,000 umc).
- **Total supply cap:** 1,000,000,000 MC (1e15 umc), enforced at the `x/tokenomics` minter. Minting beyond the cap panics.
- **Inflation:** Zero. No block subsidy, no ongoing emission.
- **Scarcity levers:** gas burn (7%), DEX swap-fee burn (50%), slash burn (40%), and enterprise-settlement burn (40%) are deflationary.

---

## 5. Token Distribution — Five Pools plus the Protocol Treasury

The fixed supply is allocated at genesis across five pools. A sixth on-chain address — the **Protocol Treasury** — is created empty (genesis balance zero) and funded only through protocol operations described in §7 and §8.

| # | Pool | Share | Amount (MC) | Custody |
|---|------|-------|-------------|---------|
| 1 | Device Incentive | 55% | 550,000,000 | `depin` module account; released per verified task |
| 2 | Staking-Security | 15% | 150,000,000 | `staking_security` module account (code-unspendable) |
| 3 | Team | 12% | 120,000,000 | 3-of-5 multisig vesting account |
| 4 | Foundation | 13% | 130,000,000 | 50,000,000 T0 unlock + 80,000,000 over 2-year linear vesting |
| 5 | Early Development | 5% | 50,000,000 | Operational multisig/vesting address |
| 6 | **Protocol Treasury** | — | **0 at genesis** | `protocol_treasury` module account (governance multisig + timelock) |

The five-pool sum is enforced to 100% by genesis validation. The Protocol Treasury is the **6th independent address**, physically separated from the staking-security pool and logically linked: it starts at zero and is filled only by (a) enterprise settlement fee回流 and (b) staking-security drip renewal when pool A is exhausted.

---

## 6. Staking-Security Drip (12-Year Floor)

The 150,000,000 MC Staking-Security pool continuously drips to validators and delegators, creating a "stake → secure → be rewarded" loop.

**Mechanism.** Each drip interval (every 100 blocks ≈ 6.7 min):

```
D_t = min( 5% × Staked ,  balance / (IntervalsPerYear × 12) )
```

- Target rate = 5% of total bonded MC per year, amortized per interval.
- The drip is capped so the source cannot be exhausted before the **12-year floor**.
- `IntervalsPerYear ≈ 7,884,000 blocks / 100 = 78,840`.

**Two-address renewal (A → B).** Pool A = `staking_security` (code-unspendable, 150M MC) is used first. If A is exhausted before the 12-year floor, the **Protocol Treasury (B)** continues the drip at the renewal floor APR of **1%–2%** (`RenewalFloorAPR`), guaranteeing the 12-year floor regardless of staking participation.

**Liquidity source.** Gas fees rebate 10% of each interval's collected fees into the staking-security pool, replenishing A.

---

## 7. Fee and Burn Mechanics

### 7.1 Gas — 7% Burn + 10% Rebate
Every 100 blocks, collected gas fees are split:
- **7% burned to the blackhole** (deflation, `GasBurnRatioBps = 700`).
- **10% rebated to the staking-security pool** (feeds the drip loop).

### 7.2 DEX Swap Fee
Swap fee = 0.30% of trade volume.
- **50% burned** (permanent deflation).
- **50% to LP providers** (stays in the pool reserve).

### 7.3 Enterprise Settlement Fee **[Live in code; call sites finalized per module]**
Enterprises and institutions pay MC when they consume on-chain services — oracle data, device settlement, and Edge-AI inference. A **1.5% settlement fee** (`EnterpriseSettlementFeeBps = 150`) is charged and split:
- **40% burned** (`EnterpriseFeeBurnRatioBps = 4000`).
- **60% routed to the Protocol Treasury** (`EnterpriseFeeTreasuryRatioBps = 6000`).

This gives the treasury a continuous, non-dilutive revenue stream and reinforces deflation.

---

## 8. Slashing and Security

### 8.1 Slash Split — 40% Burn / 60% Treasury
When a bonded validator is slashed, the slashed amount (which lands in the fee-collector) is split:
- **40% burned** (`SlashBurnRatioBps = 4000`).
- **60% routed to the Protocol Treasury** (`SlashTreasuryRatioBps = 6000`).

This turns penalties into protocol value rather than pure redistribution.

### 8.2 Double-Sign — Permanent Tombstone
Equivocation (signing two different blocks at the same height) is permanently tombstoned: the validator can **never** re-enter the active set, even after re-bonding. This is a hard, non-negotiable on-chain rule (`DoubleSignPermanentTombstone = true`).

### 8.3 Offline / Fraud Slashing (phonenode)
| Offense | Penalty |
|---------|---------|
| Offline (past grace) | 5% (`OfflineSlashBps = 500`) |
| Fraudulent contribution | 10% (`ContribSlashBps = 1000`) |
| Forged attestation | 20% (`AttestSlashBps = 2000`) |
| Re-attest cooldown | 43,200 blocks (~12h) |

Attestation validity is **30 days**; expiry pauses device-pool rewards and higher-tier commissions for that identity but does **not** reset the referral tree or downstream earnings.

---

## 9. DePIN and Device Incentives

The 55% Device Incentive pool rewards real phone work:
- **Registration & attestation:** a phone proves it is a real device (hardware attestation) and joins as a DePIN node.
- **Contribution metering:** tasks are scored; rewards are paid per verified task. A minimum contribution score (threshold = 30) is required; below it, no reward is issued but the attempt is recorded.
- **Node allowance:** a per-node daily allowance from the device pool is defined as a protocol parameter (device-pool construction reward), paid to attested, active nodes.
- **Attestation validity:** 30 days; renewal keeps the node eligible for device-pool rewards and higher-tier referral commissions.

---

## 10. Referral System

The referral rewards are drawn from an **independent 82,500,000 MC sub-budget** carved out as 15% of the Device Incentive pool. It is paid **entirely to the referrer (upline)**; the referred party's own earnings (device rewards, staking yield, allowances) are **100% retained** — nothing is deducted from the downline.

- **Depth:** 3 levels (`MaxReferralDepth = 3`).
- **Downline 100% self-retained:** the 23.5% often cited is the *network acquisition-cost cap* paid from the independent budget to the upline, never a slice taken from the downline.
- **Eligibility (4 conditions):** contribution tier 0–2000 ×1.0, 2000–5000 ×0.6, >5000 ×0.3; and a minimum referral quality ratio R ≥ 0.5.
- **Release schedule (3 phases):** 30% / 36% / 34% over the vesting horizon.
- **Entry:** dual entry — invite code **and** pre-filled invite link — with a 72-hour grace period and irreversible binding.

---

## 11. Consensus and Node Tiers

MC uses three node tiers:
1. **Consensus Validator** — runs the CometBFT consensus; requires ≥ 30,000 MC self-bond, meets uptime, and is always-on. **A cloud/server validator is mandatory for mainnet** because phones cannot provide 24/7 liveness.
2. **Economic Staking Node** — delegates or self-bonds to a validator; earns the security drip.
3. **Device DePIN Node** — a phone providing real attestation and tasks; earns the device pool.

**Delegation** is native via `x/staking` (MsgDelegate) and is enabled at mainnet.

**Path C — Phone–Cloud Co-sign [Planned enhancement].** The cloud validator (consensus) is bound to a real phone device identity (attestation) so one node earns both consensus and device rewards, strengthening the "real device secures the network" narrative. The cloud validator is **required**; the co-sign binding is an **enhancement** layered on top, so mainnet can launch with independent cloud validators and device nodes, then iterate to co-sign.

**Liquid Staking [Planned].** A liquid-staking module (`mcStMC`) is committed for mainnet, allowing staked MC to remain liquid while securing the network. Module integration is the closing engineering step.

---

## 12. EdgeAI Marketplace

`x/edgeai` is a marketplace where requesters post inference tasks, providers submit results, and disputes are arbitrated. Settlement uses the enterprise settlement fee (§7.3), with 40% burned and 60% to the treasury.

---

## 13. DEX

`x/dex` provides on-chain swaps with the 0.30% fee split 50% burn / 50% LP (§7.2).

---

## 14. Governance and the Treasury

The Protocol Treasury is spent only through governance: a multisig plus a timelock. Treasury inflows come from enterprise settlement fees (§7.3) and, if needed, staking-security drip renewal (§6). This keeps the treasury accountable and separates it from the code-unspendable staking-security pool.

---

## 15. CosmWasm Smart Contracts [Planned for mainnet launch]

CosmWasm (WebAssembly) smart-contract support is **committed for mainnet launch**. The wasm module integration into the mainnet client is the final engineering step; once live, developers can deploy composable contracts (DeFi, dApps) on MC from day one. Until the module is wired and tested, on-chain programmability is provided by the native modules above.

---

## 16. Roadmap

- **Mainnet launch:** five-pool genesis, staking-security drip (12-yr floor + A→B renewal), gas 7% burn, slash 40/60 split, dual-sign permanent tombstone, enterprise settlement fee, native delegation, DePIN + EdgeAI + DEX.
- **Shortly after:** Protocol Treasury governance (multisig + timelock) live operations; Path C phone–cloud co-sign.
- **Integration finalization:** CosmWasm smart contracts; liquid staking (`mcStMC`).

---

*MC Chain Team — parameters are in code, the ledger is public, and ownership returns to the device in your pocket.*
