# CosmWasm Integration Specification

Status: **Planned.** Not compiled into the mainnet client.

This document states exactly why CosmWasm is not yet in the binary, what the
integration consists of, and what has to be true before the status changes to
Live. It exists so that the gap between the roadmap and the repository is
written down rather than implied.

---

## 1. Why it is not shipped yet

CosmWasm execution depends on `libwasmvm`, a Rust library consumed through cgo.
Three facts govern the current state:

1. **`wasmvm` v1.5.0 ships no Windows link target.** The module distributes
   `libwasmvm.x86_64.so`, `libwasmvm.aarch64.so`, and `libwasmvm.dylib`. There
   is a `link_windows.go` declaring `-lwasmvm`, but no matching Windows import
   library or DLL is included, so the link step has nothing to resolve against.

2. **With cgo disabled, the keeper is a deliberate panic.** `wasmd` guards its
   constructor by build tag. `x/wasm/keeper/keeper_no_cgo.go` compiles cleanly
   and then calls `panic("not implemented, please build with cgo enabled")`.
   A `CGO_ENABLED=0` build therefore *compiles* and *fails at application
   construction*, which is worse than not wiring it at all: every node would
   abort on startup.

3. **The current build host has no C toolchain.** No `gcc` and no
   `x86_64-w64-mingw32-gcc` are present, so cgo cannot be enabled locally even
   if a Windows library existed.

A build that compiles but panics on boot is not an integration. The module is
left out of `app.go` and the whitepaper keeps the Planned label.

### Verification performed

| Check | Result |
| --- | --- |
| `go build github.com/CosmWasm/wasmd/x/wasm` under `CGO_ENABLED=0` | compiles |
| `type VM struct` build tag in `wasmvm/lib.go` | `//go:build cgo` |
| `x/wasm/keeper/keeper_no_cgo.go` body | `panic(...)` |
| Windows library present in `wasmvm/internal/api` | absent |
| `gcc` on build host | absent |

The dependency probe that produced these results was reverted. `go.mod` is back
at `ibc-go v7.1.0` and carries no `wasmd` or `wasmvm` requirement, so the
committed tree builds and tests clean.

---

## 2. Build requirements to lift the block

Integration is gated on the build environment, not on protocol design.

- Linux `amd64`/`arm64` or macOS host, or a Linux container for CI and release.
- `CGO_ENABLED=1` with a working C toolchain.
- For static release binaries, the muslc path: link against
  `libwasmvm_muslc.a` with the `muslc` build tag.
- Release images must pin one `wasmvm` version across every validator. A
  mismatched `libwasmvm` between nodes is a consensus fault, not a local bug.

Because the mainnet node image is Linux, this is a release-pipeline task rather
than a protocol change. It lands when the chain's official build moves to the
containerised Linux pipeline.

---

## 3. Integration specification

The wiring follows the standard `wasmd` pattern for Cosmos SDK v0.47. It is
recorded here so the change is reviewable before it is written.

### 3.1 Dependencies

```
github.com/CosmWasm/wasmd v0.45.0     // pairs with SDK v0.47.x
github.com/CosmWasm/wasmvm v1.5.0
github.com/cosmos/ibc-go/v7 v7.3.0    // required minimum for wasmd v0.45.0
```

Note the coupling: `wasmd v0.45.0` raises `ibc-go` from v7.1.0 to v7.3.0. That
bump must be validated against the existing transfer and interchain-accounts
wiring before it is merged, not alongside it.

### 3.2 Application changes

Registration in `app/app.go`:

- `ModuleBasics`: add `wasm.AppModuleBasic{}`.
- `maccPerms`: add `wasmtypes.ModuleName: {authtypes.Burner}`.
- Store keys: add `wasmtypes.StoreKey`.
- Keeper construction after `IBCKeeper`, `TransferKeeper`, `DistrKeeper`, and
  the scoped capability keeper for wasm, passing the message router, the gRPC
  query router, `homePath`, the parsed `wasmtypes.WasmConfig`, the capability
  string, and the gov module authority.
- IBC router: `AddRoute(wasmtypes.ModuleName, wasm.NewIBCHandler(...))`,
  registered before `SetRouter`.
- Module manager: append the wasm app module; add `wasmtypes.ModuleName` to the
  begin-block, end-block, and init-genesis orders, placing genesis after
  `ibctransfertypes` so contracts can bind ports.
- Ante handler: prepend `wasmkeeper.NewLimitSimulationGasDecorator` and
  `wasmkeeper.NewCountTXDecorator`.
- Snapshot extension: register `wasmkeeper.NewWasmSnapshotter` on the multistore
  so state-sync carries contract code.

### 3.3 Chain policy

- **Capabilities.** `iterator,staking,stargate,cosmwasm_1_1,cosmwasm_1_2`.
- **Upload permission.** Governance-gated at launch
  (`AccessTypeAnyOfAddresses` on the gov authority), relaxed by proposal once
  the audit surface is understood. An open upload policy on day one is a
  denial-of-service surface, not a feature.
- **Stargate allowlist.** Query and message allowlists are explicit. Native
  module messages are exposed one at a time, each with its own review.
- **Gas.** Contract gas is metered by the standard `wasmd` register. The 7% gas
  burn described in the whitepaper applies to contract execution with no
  exception path.
- **Supply cap.** Contracts have no mint authority. `wasmtypes.ModuleName`
  receives `Burner` only. The 1,000,000,000 MC ceiling is unaffected by
  contract deployment.

### 3.4 Acceptance criteria

The status moves to Live only when all of the following hold on a Linux cgo
build:

1. `go build ./...` and `go test ./...` pass with the wasm module wired in.
2. A node starts, produces blocks, and reports a non-empty
   `LibwasmvmVersion()`.
3. Store, instantiate, execute, and query succeed against a reference contract
   on a local devnet.
4. The tokenomics minted-supply invariant still holds after contract activity.
5. State-sync restores a node whose snapshot contains contract code.
6. The `ibc-go` v7.3.0 upgrade passes the existing transfer and
   interchain-accounts tests unchanged.

Until every line above is satisfied on a real build, programmability on
MobileChain is what the native modules provide.
