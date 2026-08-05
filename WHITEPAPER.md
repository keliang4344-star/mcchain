# MobileChain (MC)

### A Public Chain Where the Node Fits in a Pocket

**Chain ID** `mcchain-mainnet-1` · **Native token** MC · **Base unit** `umc` (6 decimals) · **Supply** 1,000,000,000 MC, fixed · **Inflation** zero

**Published by the MC Chain Team** · Version 1.0 · 2026

---

### How to read this document

Every economic number in this whitepaper maps to a named constant in the open-source client. Appendix A lists each parameter next to the file that defines it, so a reader can check the claim against the code rather than trust the prose.

Two status markers are used throughout:

| Marker | Meaning |
|--------|---------|
| **Live** | Implemented in the mainnet client and covered by tests |
| **Planned** | Committed on the roadmap, not yet in the client |

Where the code and this document disagree, the code governs.

---

## 1. Executive Summary

MobileChain is a Cosmos SDK application chain for decentralized physical infrastructure (DePIN) and edge AI. Its distinguishing choice is the participation floor: a consumer smartphone, with hardware attestation, is a first-class contributor to the network and earns from it.

Three properties define the economics.

**Fixed supply.** One billion MC, minted at genesis, never expanded. The minter panics above the cap. There is no block subsidy and no ongoing emission.

**Distribution weighted to contributors.** Fifty-five percent of supply sits in the device incentive pool and is released only against verified work. The team allocation is twelve percent behind a multisig with a one-year cliff and three-year linear vesting.

**Deflation from usage, not from promises.** Gas fees, swap fees, and penalties burn MC. Every burn is a permanent reduction against the fixed cap. Contributor earnings are never touched.

A sixth on-chain address, the Protocol Treasury, is created empty at genesis and funded only by protocol revenue. It is the mechanism that lets the network fund itself without diluting holders.

---

## 2. The Problem

Public chains have been consistent about decentralization as a value and inconsistent about it as a fact. Proof-of-work concentrated into industrial mining. Proof-of-stake concentrated into custodial staking desks and validators with six-figure capital requirements. In both cases the network was open in principle and closed in practice, because the cost of participating was set by hardware and capital rather than by protocol design.

Meanwhile, roughly five billion smartphones sit idle for most of every day. They hold real compute, real sensors, real network presence, and a hardware root of trust that can prove they are genuine devices. They are the largest under-used infrastructure in existence, and no major chain treats them as infrastructure.

The second problem is ownership of value. Platforms monetized user activity and returned none of the equity. A network that wants a different outcome cannot rely on policy, because policy is revocable. It has to write the distribution into consensus rules that no operator can quietly amend.

---

## 3. Design Principles

**Parameters live in code.** Ratios, thresholds, and caps are constants in the repository, not entries in an off-chain policy document.

**No minting after genesis.** Every payout draws from a pre-allocated pool. A module that cannot mint cannot inflate.

**Penalties reduce supply, they do not enrich an operator.** Slashed value is split between a burn and the staking-security pool, both of which are public. No individual receives the proceeds of another participant's penalty.

**Rewards follow verified work.** Attestation, contribution scoring, and dispute windows gate every payout. Presence alone earns nothing.

**Unfinished work is labeled unfinished.** A roadmap item is marked Planned until it ships.

---

## 4. Protocol Architecture

```
+-----------------------------------------------------------+
|                     Application Layer                      |
+------------+---------+-----------+----------+--------------+
| tokenomics |  depin  | phonenode |  edgeai  |     dex      |
|  issuance  | device  |  mobile   | AI task  |  native AMM  |
|   & fees   | rewards | security  |  market  |  & burn      |
+------------+---------+-----------+----------+--------------+
|              Cosmos SDK Standard Modules                   |
|      bank / staking / gov / auth / slashing / ibc          |
+-----------------------------------------------------------+
|                CometBFT Consensus Engine                   |
+-----------------------------------------------------------+
```

Consensus is CometBFT BFT with roughly four-second blocks and instant finality. The application layer is six custom modules.

| Module | Responsibility |
|--------|----------------|
| `x/tokenomics` | Sole holder of the supply cap; genesis allocation of the five pools; staking-security drip; gas burn and rebate; enterprise settlement fee policy |
| `x/depin` | Device contribution ledger: registration, contribution scoring, reward release from the device pool |
| `x/phonenode` | Mobile node security: hardware attestation, Sybil binding, liveness, tiered slashing |
| `x/edgeai` | Edge AI task market: escrow, optimistic settlement, dispute arbitration, verifier sampling |
| `x/dex` | Constant-product AMM with a deflationary fee |
| `x/referral` | Referral accounting funded from an independent budget, with per-user and network-wide circuit breakers |

---

## 5. The MC Token

MC is the native token. One MC equals 1,000,000 `umc`, and all on-chain arithmetic uses `umc`.

The supply cap of 1,000,000,000 MC is enforced inside `x/tokenomics`. The `x/mint` module is configured to zero, so no block produces new supply. Any attempt to mint past the cap halts the transaction rather than silently truncating.

Three mechanisms remove MC from circulation permanently:

| Source | Burn |
|--------|------|
| Gas fees | 7% of collected fees |
| DEX swaps | 50% of the 0.30% swap fee (0.15% of volume) |
| Slashing | 40% of the slashed amount |

Every burn source is either a protocol usage fee or a penalty for misbehavior. Contributor earnings are never burned: device task bounties, referral bonuses, and EdgeAI task payouts reach the participant in full, with nothing withheld on-chain.

Because supply is fixed, each burn raises the proportional claim of every remaining holder. Deflation here is a consequence of network usage, not a marketing schedule.

---

## 6. Allocation: Five Pools and the Protocol Treasury

The entire supply is allocated at genesis. Genesis validation rejects any configuration whose pool shares do not sum to one hundred percent.

| # | Allocation | Share | MC | Custody and release |
|---|-----------|-------|-----|---------------------|
| 1 | Device Incentive | 55% | 550,000,000 | `depin` module account; released per verified contribution |
| 2 | Staking Security | 15% | 150,000,000 | `staking_security` module account; unspendable except by the drip |
| 3 | Team | 12% | 120,000,000 | 3-of-5 multisig; 1-year cliff, then 3-year linear vesting |
| 4 | Foundation | 13% | 130,000,000 | 50,000,000 unlocked at genesis; 80,000,000 on 2-year linear vesting |
| 5 | Early Development | 5% | 50,000,000 | Operational multisig and vesting address |
| 6 | **Protocol Treasury** | — | **0 at genesis** | `protocol_treasury` module account; governance multisig with timelock |

Two details deserve emphasis.

The device pool carves out 82,500,000 MC — fifteen percent of itself — as the referral budget. The remaining 467,500,000 MC funds device rewards. Genesis asserts this arithmetic, so the two figures cannot drift apart.

The Protocol Treasury holds nothing at launch. It is the sixth independent address, physically separate from the staking-security pool and economically linked to it. Every coin it ever holds arrives from protocol revenue described in section 8, or from drip renewal described in section 7. There is no pre-mine and no reserved allocation.

---

## 7. Staking Security Drip

Staking secures the chain, and the staking-security pool pays for that security without printing money.

The pool releases value on a fixed cadence: once every 100 blocks, roughly every six and a half minutes. Each release is bounded by two constraints at once.

```
D_t = min( 5% x Staked / IntervalsPerYear ,  Balance / (IntervalsPerYear x 12) )
```

The first term targets a five percent annual return on all bonded MC. The second term is a floor guarantee: the pool may never pay out faster than a twelve-year runway allows. When staking participation is low, the first term binds and the pool lasts longer than twelve years. When participation is high, the second term binds and the yield falls rather than the pool draining early. `IntervalsPerYear` is 78,840, derived from 7,884,000 blocks per year at the 100-block cadence.

**Renewal across two addresses.** Pool A is the 150,000,000 MC staking-security allocation. It is spent first. If A is exhausted before the twelve-year floor is reached, the drip continues from the Protocol Treasury at a renewal rate of one to two percent APR. The twelve-year commitment therefore holds regardless of how staking participation evolves, and it holds without minting.

**Replenishment.** Ten percent of collected gas fees is rebated into the staking-security pool at each interval. Network usage extends the runway of the pool that pays for network security.

*Status: Live.*

---

## 8. Fees, Burns, and Treasury Inflows

### 8.1 Gas

Gas is collected in `umc` and processed every 100 blocks. Seven percent is burned. Ten percent is rebated to the staking-security pool. The remainder follows standard fee distribution to validators and delegators.

*Status: Live.*

### 8.2 Swap fee

The native AMM charges 0.30% per swap. Half of that fee is burned, which is 0.15% of trade volume permanently removed. The other half stays in the pool reserve as liquidity-provider return. No portion of the swap fee is diverted to the treasury; liquidity providers and the burn split it evenly.

*Status: Live.*

### 8.3 Enterprise settlement fee

Institutional consumers of the network — enterprises purchasing edge inference, device settlement, or oracle data — pay a 1.50% settlement fee. The fee is charged to the demand side, on top of the amount being escrowed, so it never reduces the payout to the contributor performing the work.

The fee splits 40% to node operators, distributed through the fee collector alongside block rewards, and 60% to the Protocol Treasury. The split is computed dust-free: the treasury receives the remainder after the node share, so the two legs always reconstruct the fee exactly. Nothing is burned and nothing is minted on this path — enterprise revenue is redistributed, never destroyed.

On the EdgeAI path the fee is collected when a task is escrowed. A requester who can fund the reward but not the fee still gets the task posted; the fee is waived and the waiver is recorded as an event rather than blocking the transaction.

*Status: Live on the EdgeAI settlement path. Additional settlement paths adopt the same policy engine as they launch.*

### 8.4 What the treasury is for

The treasury exists so the protocol can fund audits, integrations, liquidity, grants, and the drip renewal of section 7 out of revenue rather than out of holders' balances. It is spent only through governance, behind a multisig and a timelock, and its balance is public at every block.

---

## 9. Validator and Node Security

### 9.1 Slashing splits, it does not vanish

When a bonded validator is penalized, the slashed stake is moved out of the bonded pool and split: 40% is burned, 60% is routed to the staking-security pool, where it is drip-distributed to honest nodes and validators. Native Cosmos behavior would burn the full amount; MobileChain burns the deflationary share and turns the majority into a subsidy for the participants who kept the network honest. Nothing is minted on this path, and no operator receives the proceeds directly.

*Status: Live.*

### 9.2 Equivocation is permanent

Signing two different blocks at the same height is treated as an unrecoverable offense. The validator is tombstoned permanently and can never rejoin the active set, even by re-bonding with a new stake. This is not a parameter that governance is expected to soften; it is the boundary condition that makes the security model meaningful.

*Status: Live.*

### 9.3 Device-layer penalties

Mobile nodes are governed by a separate, gentler schedule, because a phone losing signal is not equivalent to a validator attacking consensus.

| Offense | Penalty |
|---------|---------|
| Offline beyond the grace window | 5% |
| Fraudulent contribution report | 10% |
| Forged attestation | 20% |

The offline grace window is 100 blocks. After any penalty, a node enters a 43,200-block cooldown — about twelve hours — during which it cannot re-attest. That cooldown exists so a caught node cannot immediately re-enter under a fresh proof.

### 9.4 Attestation and what it does not touch

Hardware attestation is valid for 30 days and must be renewed. An expired attestation pauses that identity's device-pool rewards and its eligibility for higher-tier referral commissions.

Expiry does not reset referral relationships. The binding between a referrer and a referred account is permanent and independent of attestation state. It does not reduce the earnings of anyone below that identity in the tree. This separation is deliberate: attestation proves a device is real right now, while the referral record is a historical fact that does not expire.

---

## 10. The Device Layer

The device incentive pool pays for verified work, and only for verified work.

A phone joins by registering and presenting hardware attestation. Sybil binding ties one identity to one device, so the same handset cannot multiply into a farm of accounts. Contribution is then scored per task; a submission below the quality threshold of 30 earns nothing, though the attempt is still recorded on-chain for auditability.

Attested, active nodes also receive a daily node allowance from the device pool. The allowance is a protocol parameter and is paid only while attestation is current.

`x/depin` has no minting authority. It distributes from a pre-funded balance of 467,500,000 MC and can never exceed it.

*Status: Live.*

---

## 11. Referral Program

Referral rewards are funded from an independent budget of 82,500,000 MC, carved out of the device pool at genesis.

The structural point matters more than the rates: **the reward is paid to the referrer out of the referral budget, and nothing is deducted from the referred account.** A referred participant keeps one hundred percent of their device rewards, staking yield, and allowances. The two flows come from different sources and never touch.

| Parameter | Value |
|-----------|-------|
| Depth | 3 levels |
| Level 1 | 10% |
| Level 2 | 5% |
| Level 3 | 2% |
| Daily cap per referrer | 500 MC |
| Daily cap network-wide | 20,600 MC |
| Minimum payout | 100 MC |
| Maximum referrals per account | 100 |
| Binding | Permanent and irreversible |

The percentages are network acquisition cost, expressed relative to the referred account's contribution and paid from the independent budget. They are a ceiling on what the network will spend to acquire a participant, not a share taken from that participant.

Two circuit breakers cap the program: a per-referrer daily limit and a network-wide daily limit. At the network limit the budget supports roughly eleven years of continuous maximum draw, and actual consumption tracks real activity well below that ceiling. The module cannot mint; when the budget is spent, the program ends.

*Status: Live. Governance decides whether levels two and three activate after mainnet, and may close the program at any time.*

---

## 12. Node Tiers, Delegation, and Liquid Staking

MobileChain runs three participation tiers rather than one.

**Consensus validators** produce blocks. They require a minimum self-delegation of 30,000 MC, enforced chain-wide by an ante decorator on every validator creation and edit, and they require continuous uptime. A cloud or server validator is mandatory for mainnet, because a phone cannot guarantee twenty-four-hour liveness and a consensus set that pretends otherwise would be fragile by construction.

**Economic staking nodes** delegate to a validator or self-bond, and earn the security drip described in section 7. Delegation uses native `x/staking` and is enabled from mainnet.

**Device nodes** are phones producing attestation and completing tasks. They earn from the device pool and require no capital.

**Phone-cloud co-signing** binds a cloud validator's consensus identity to an attested physical device, so a single operator earns from both consensus and device contribution. The cloud validator is the requirement; the co-signing binding is an enhancement layered on top. The two roles are independent at launch, with co-signing enabled as a post-launch upgrade.

*Status: Live. Validator threshold, delegation, the tier structure, and phone-cloud co-signing are Live.*

**Liquid staking** issues a transferable representation of bonded MC, so stake can secure the network without being immobilized. The module is live in the client.

*Status: Live.*

---

## 13. Edge AI Market

`x/edgeai` is a market for inference work. A requester posts a task and escrows the reward. Providers submit results. Settlement is optimistic: after the dispute window of 100 blocks passes without challenge, payment executes automatically.

The payout splits two ways. Eighty-five percent goes to the node that performed the work. Fifteen percent is reserved for verifiers, claimed when a verifier is sampled to inspect a completed task. Nothing is burned on this path: the reward a requester escrows reaches the participants who produced and checked the result, in full.

A dispute freezes settlement until arbitration resolves it. A ruling of fraud claws back the escrowed submitter share and routes it to the staking-security pool, where it is drip-distributed to honest nodes and validators, and the submitter is penalized separately. Single-task reward is capped at 1,000 MC, and the anti-cheat consensus threshold is fifty percent.

Verifier selection is reputation-weighted and samples completed tasks after the fact, so a provider cannot know in advance which submissions will be checked.

*Status: Live. On-chain recomputation serves as a second verification layer.*

---

## 14. Native Exchange

`x/dex` is a constant-product automated market maker supporting pool creation, swaps, and liquidity provision.

The initial MC/USDT pool is seeded with 5,000,000 MC from the foundation genesis unlock, paired with market-maker USDT, at an opening reference price of 0.02 USDT per MC. Swap fee is 0.30%, split evenly between the burn and liquidity providers. Liquidity positions carry a seven-day minimum lock.

The exchange is where a phone's earnings become liquid, and every trade that passes through it reduces total supply.

*Status: Live.*

---

## 15. Oracles and Off-Chain Services

Some facts the chain needs are not native to the chain: whether a device is genuinely online, whether an inference actually ran, whether an attestation is authentic.

The oracle service submits those facts under constraint. Access requires a bearer token, submissions are rate-limited, and transport encryption is available. Device attestation verification is fail-closed and checks multiple real hardware roots of trust; a signing request that cannot be verified is rejected rather than waved through. In production the oracle public key is mandatory, and a missing key halts startup instead of degrading to a permissive mode.

The boundary is explicit. The oracle reports; it does not decide. Reward release, penalties, and settlement are executed by on-chain modules following rules written in code.

*Status: Live.*

---

## 16. Governance

Governance uses the standard Cosmos governance module: proposal, deposit, voting, execution. Parameters that governance can reach are marked as such in Appendix A.

Three constraints bound it. The supply cap is not a governance parameter. Permanent tombstoning for equivocation is not a governance parameter. Treasury spending requires both a multisig and a timelock, so no single approval moves funds and every movement is visible before it executes.

The intended trajectory is a progressive handover: the team holds fewer operational keys over time, and more parameters move under on-chain vote. That transfer is measured by what has been transferred, not by what has been announced.

---

## 17. Programmability

CosmWasm smart-contract support is integrated via the `x/wasm` module (wasmd v0.45, wasmvm 1.5): developers deploy composable contracts directly on MobileChain and compose with the native modules described here. The module is wired into the application on CGO-enabled (Linux) builds, where the WebAssembly runtime is linked; contract execution is not available on non-CGO builds. Governance can adjust wasm parameters through the module's parameter space.

*Status: Live. The WebAssembly module is integrated and enabled on CGO-enabled builds; native modules provide programmability on all builds.*

---

## 18. Roadmap

The roadmap is stated as milestones, not dates. Progress is measured against on-chain state and the public repository.

**Delivered.** Eight native modules; five-pool genesis with end-to-end verification; zero-inflation enforcement; mobile node security including attestation, Sybil binding, offline grace, tiered slashing, and cooldown; liquid staking with delegated bonding; phone-cloud co-signing; the EdgeAI economic loop from posting through optimistic settlement and arbitration with on-chain recomputation as a second verification layer; the native AMM with its burn; the referral module with dual circuit breakers; the 30,000 MC validator floor; oracle hardening with fail-closed device verification; the twelve-year drip with treasury renewal; gas burn and rebate; the slash split; the enterprise settlement fee; the Protocol Treasury as the sixth address; progressive governance handover with timelock; off-chain settlement batching for high-frequency micro-payouts; IBC interoperability through interchain accounts and IBC transfer; and CosmWasm smart contracts via `x/wasm` (enabled on CGO-enabled builds).

**In progress.** Mainnet launch preparation: genesis ceremony, validator recruitment, final audit. Mobile SDK refinement. Dashboard and RPC configuration.

**Planned.** None outstanding at the protocol layer: CosmWasm was the final planned capability and is now delivered with the `x/wasm` integration (WebAssembly runtime active on CGO-enabled builds).

---

## Appendix A — Parameter Reference

Each row names the constant and the file that defines it. Where a value here disagrees with the code, the code is correct and this document is stale.

### A.1 Chain

| Parameter | Value | Source |
|-----------|-------|--------|
| Consensus engine | CometBFT v0.37.6 | `go.mod` |
| Application framework | Cosmos SDK v0.47.14 | `go.mod` |
| Language | Go 1.21 | `go.mod` |
| Mainnet chain ID | `mcchain-mainnet-1` | genesis |
| Address prefix | `mc` | `app/app.go` |
| Base denomination | `umc` | `x/tokenomics/types/keys.go` |
| Decimals | 6 | chain config |
| Inflation | 0 | `x/mint` genesis |
| Block time | ~4 s | CometBFT config |

### A.2 Supply and allocation

| Parameter | umc | MC | Constant |
|-----------|-----|-----|----------|
| Supply cap | 1,000,000,000,000,000 | 1,000,000,000 | `TotalSupplyCap` |
| Device incentive 55% | 550,000,000,000,000 | 550,000,000 | `DeviceIncentivePercentBps = 5500` |
| — referral budget | 82,500,000,000,000 | 82,500,000 | `ReferralEcosystemBudget` |
| — device rewards | 467,500,000,000,000 | 467,500,000 | `DepinInitialPoolSlice - ReferralEcosystemBudget` |
| Staking security 15% | 150,000,000,000,000 | 150,000,000 | `StakingSecurityPercentBps = 1500` |
| Team 12% | 120,000,000,000,000 | 120,000,000 | `TeamPercentBps = 1200` |
| Foundation 13% | 130,000,000,000,000 | 130,000,000 | `FoundationPercentBps = 1300` |
| — genesis unlock | 50,000,000,000,000 | 50,000,000 | `FoundationT0Unlock` |
| Early development 5% | 50,000,000,000,000 | 50,000,000 | `EarlyDevPercentBps = 500` |
| DEX initial liquidity | 5,000,000,000,000 | 5,000,000 | `DexInitialLiquidityMC` |
| Protocol treasury at genesis | 0 | 0 | `ProtocolTreasuryPoolName` |
| Team multisig threshold | 3-of-5 | — | `TeamMultisigThreshold` |
| Team vesting | 1-year cliff + 3-year linear | — | `x/tokenomics/keeper/genesis.go` |

### A.3 Drip and fees

| Parameter | Value | Constant |
|-----------|-------|----------|
| Drip interval | 100 blocks | `DripIntervalBlocks` |
| Drip target rate | 5% APR on bonded MC | `DripRatioBps = 500` |
| Blocks per year | 7,884,000 | `BlocksPerYear` |
| Intervals per year | 78,840 | `IntervalsPerYear` |
| Drip floor | 12 years | `DripFloorYears` |
| Renewal APR band | 1.00% – 2.00% | `RenewalFloorAPRBps`, `RenewalFloorAPRCeilBps` |
| Gas burn | 7.00% | `GasBurnRatioBps = 700` |
| Gas rebate to security pool | 10.00% | `GasRebateRatioBps = 1000` |
| Enterprise settlement fee | 1.50% | `EnterpriseSettlementFeeBps = 150` |
| — to node operators | 40.00% | `EnterpriseFeeNodeRatioBps = 4000` |
| — to treasury | 60.00% | `EnterpriseFeeTreasuryRatioBps = 6000` |
| Slash burn | 40.00% | `SlashBurnRatioBps = 4000` |
| Slash to staking-security pool | 60.00% | `SlashSecurityRatioBps = 6000` |
| Permanent tombstone on equivocation | true | `DoubleSignPermanentTombstone` |

### A.4 Device layer

| Parameter | Value | Source |
|-----------|-------|--------|
| Initial device vault | 467,500,000 MC | `DefaultInitialPool` |
| Reward denomination | `umc` | `DefaultRewardDenom` |
| Minting authority | none | `maccPerms` |
| Contribution quality threshold | 30 | `ContributionThreshold` |
| Genesis consistency assertion | `DepinInitialPoolSlice - ReferralEcosystemBudget == DefaultInitialPool` | `x/tokenomics/keeper/genesis.go` |

### A.5 Mobile node security

| Parameter | Value | Source |
|-----------|-------|--------|
| `AttestationRequired` | true | `x/phonenode/types/params.go` |
| `AttestationValidity` | 2,592,000 s (30 days) | same |
| `SybilDeviceBinding` | true | same |
| `OfflineGraceBlocks` | 100 | same |
| `OfflineSlashBps` | 500 (5%) | same |
| `ContribSlashBps` | 1000 (10%) | same |
| `AttestSlashBps` | 2000 (20%) | same |
| `SlashCooldownBlocks` | 43,200 (~12 h) | `DefaultSlashCooldownBlocks` |

### A.6 Edge AI

| Parameter | Value | Source |
|-----------|-------|--------|
| `MaxTaskReward` | 1,000,000,000 umc (1,000 MC) | `x/edgeai/types/params.go` |
| `DisputePeriodBlocks` | 100 | same |
| `AntiCheatThresholdBps` | 5000 (50%) | same |
| Payout to submitter | 85% | `EdgeAISubmitterRatioBps = 8500` |
| Verifier reserve | 15% | `EdgeAIVerifierReserveRatioBps = 1500` |
| Burn on settlement | none | — |
| Cheat clawback destination | staking-security pool | `x/edgeai/keeper/clawback.go` |
| Payment model | requester escrow, optimistic settlement | `x/edgeai/keeper` |

### A.7 Exchange and referral

| Parameter | Value | Source |
|-----------|-------|--------|
| Swap fee | 30 bps (0.30%) | `DefaultFeeRateBps` |
| Fee burned | 5000 bps of fee (50%) | `FeeBurnBps` |
| Fee to liquidity providers | 5000 bps of fee (50%) | `FeeLPBps` |
| Referral depth | 3 | `MaxReferralDepth` |
| Level 1 / 2 / 3 | 1000 / 500 / 200 bps | `DefaultLevel1..3RewardRateBps` |
| Daily cap per referrer | 500,000,000 umc (500 MC) | `DefaultDailyPerUserCap` |
| Daily cap network-wide | 20,600,000,000 umc (20,600 MC) | `DefaultDailyNetworkCap` |
| Minimum payout | 100,000,000 umc (100 MC) | `DefaultMinPayout` |
| Maximum referrals per account | 100 | `DefaultMaxReferralsPerUser` |

### A.8 Consensus threshold

| Parameter | Value | Source |
|-----------|-------|--------|
| `MinSelfDelegationLowerBound` | 30,000,000,000 umc (30,000 MC) | `app/ante.go` |
| Enforcement scope | every validator create and edit transaction | `MinSelfDelegationDecorator` |

### A.9 Code map

| Component | Path | Responsibility |
|-----------|------|----------------|
| mcchain | `x/mcchain` | System anchor module |
| tokenomics | `x/tokenomics` | Supply cap, pool allocation, drip, fee policy, treasury |
| depin | `x/depin` | Device vault and per-contribution release |
| phonenode | `x/phonenode` | Attestation, Sybil binding, tiered slashing, slash split |
| edgeai | `x/edgeai` | Task escrow, optimistic settlement, arbitration, enterprise fee |
| dex | `x/dex` | Constant-product AMM and swap-fee burn |
| referral | `x/referral` | Referral ledger and circuit breakers |
| ante decorator | `app/ante.go` | Validator minimum self-delegation |
| oracle service | `internal/oraclesvc` | Constrained submission of off-chain facts |
| dashboard | `web/` | Wallet, explorer, transaction tooling |

---

## Appendix B — How to Verify These Claims

1. Clone the repository and build the client. The build is reproducible from `go.mod`.
2. Open `x/tokenomics/types/keys.go`. Every allocation ratio and fee constant in Appendix A is declared there.
3. Run the test suite. Genesis allocation, supply cap enforcement, the fee splits, and the slashing paths are covered by tests that fail if the numbers move.
4. Query a running node for module account balances. The five pools and the treasury are addressable and their balances are public at every height.
5. Compare what you find against this document. If they differ, the code is authoritative.

---

*MobileChain — the parameters are in the code, the ledger is public, and the node fits in a pocket.*

*MC Chain Team*
