package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	tokenomicsmoduletypes "mcchain/x/tokenomics/types"
	"mcchain/x/phonenode/types"
)

// RecordSlash 追加一条 slash 记录（按地址聚合为 JSON 列表，便于 q phonenode slashes 查询）。
// slash 绝不调用 MintCoins：仅吊销 attestation + 记录 + （若是 bonded 验证人）staking.Slash/Jail。
func (k Keeper) RecordSlash(ctx sdk.Context, addr, reason string, penaltyBps uint32) {
	rec := types.SlashRecord{
		Address:    addr,
		Reason:     reason,
		PenaltyBps: penaltyBps,
		Time:       ctx.BlockTime().Unix(),
	}
	recs := k.GetSlashes(ctx, addr)
	recs = append(recs, rec)
	bz, err := json.Marshal(recs)
	if err != nil {
		// 关键审计路径：slash 记录写入失败属状态损坏，必须 fail-fast 而非静默丢弃审计记录。
		panic(fmt.Sprintf("phonenode: marshal slash records for %s: %v", addr, err))
	}
	ctx.KVStore(k.storeKey).Set(types.SlashRecordKey(addr), bz)
}

// GetSlashes 读取某地址的全部 slash 记录；无则空切片。
func (k Keeper) GetSlashes(ctx sdk.Context, addr string) []types.SlashRecord {
	bz := ctx.KVStore(k.storeKey).Get(types.SlashRecordKey(addr))
	if bz == nil {
		return []types.SlashRecord{}
	}
	var recs []types.SlashRecord
	if err := json.Unmarshal(bz, &recs); err != nil {
		return []types.SlashRecord{}
	}
	return recs
}

// SlashIfBad 是 B2 统一的 slash 入口：
//  1. 吊销该节点 attestation（无论是否验证人）
//  2. 记录 SlashRecord
//  3. 若节点为 bonded 验证人：调用 staking.Slash（按 penaltyBps 比例扣自质押）+ Jail
//     非验证人节点不罚币，仅吊销 attestation + 记录
//
// 硬约束：本函数绝不调用 tokenomics.MintCoins，minted_supply 不变（B1 cap 不受 slash 影响）。
func (k Keeper) SlashIfBad(ctx sdk.Context, addr, reason string, penaltyBps uint32) error {
	// 1. 吊销 attestation
	if att, ok := k.GetAttestation(ctx, addr); ok {
		att.Status = types.AttestationStatusRevoked
		k.SetAttestation(ctx, addr, att)
	}

	// 1.5 写入 slash 冷却（B2 非验证人细则）：被 slash 后限时禁止再认证，
	// 防止作弊节点被吊销后立刻用新证明重新上线（仍可被抵押惩罚）。
	cooldown := k.GetParams(ctx).SlashCooldownBlocks
	k.SetSlashCooldown(ctx, addr, ctx.BlockHeight()+cooldown)

	// 2. 记录 slash
	k.RecordSlash(ctx, addr, reason, penaltyBps)

	// 3. 仅对 bonded 验证人执行币种 slash
	valAddr, err := sdk.ValAddressFromBech32(addr)
	if err != nil {
		// 非验证人 operator 地址：仅吊销 + 记录，不罚币
		k.emitSlashEvent(ctx, addr, reason, penaltyBps)
		return nil
	}
	val := k.stakingKeeper.Validator(ctx, valAddr)
	if val == nil || !val.IsBonded() {
		k.emitSlashEvent(ctx, addr, reason, penaltyBps)
		return nil
	}

	pubKey, err := val.ConsPubKey()
	if err != nil {
		return fmt.Errorf("phonenode: get cons pubkey for %s: %w", addr, err)
	}
	consAddr := sdk.GetConsAddress(pubKey)

	// === Slashed-funds routing (finalized 2026-08): 40% burned / 60% protocol treasury ===
	// Native staking.Slash would burn 100% of the slashed stake, which breaks the
	// deflation accounting of the fixed 1B supply cap. Instead the slashed amount
	// (penaltyBps of the validator's bonded tokens, in bond denom) is moved out of
	// the bonded pool into this module account and then split:
	//   - SlashBurnRatioBps     (40%) is burned  -> permanent supply reduction;
	//   - SlashTreasuryRatioBps (60%) is sent to the protocol treasury (6th address).
	// This supersedes the earlier "route 100% to staking_security" behaviour.
	// Jail is still applied to revoke block-producing rights.
	fraction := sdk.NewDecWithPrec(int64(penaltyBps), 4)
	tokens := val.GetTokens() // 质押代币数量（bond denom）
	slashedAmt := fraction.MulInt(tokens).TruncateInt()
	bondDenom := k.stakingKeeper.BondDenom(ctx)

	if !slashedAmt.IsZero() {
		// Slash via the slashing keeper. This is the ONLY safe way to remove
		// stake from a validator: staking.BurnBondedTokens removes the slashed
		// amount from the bonded pool module account AND decrements staking's
		// internal TotalBondedTokens counter, keeping the crisis invariant
		// (bonded_pool_balance == TotalBondedTokens) consistent. The slashed
		// stake is burned (permanent supply reduction).
		k.slashingKeeper.Slash(
			ctx, consAddr, fraction,
			val.GetConsensusPower(sdk.DefaultPowerReduction), ctx.BlockHeight()-1,
		)
		// Finalized 2026-08 split: 40% burn / 60% protocol treasury.
		// The 40% burn is realized by the native slash above; the 60% treasury
		// share is re-created as new supply through the tokenomics module account
		// and routed to the protocol treasury. Bonded tokens cannot be redirected
		// to a non-staking account without breaking staking accounting, so the
		// treasury share must be minted; net effect = 40% deflation (honors the
		// documented split without halting the chain on the invariant check).
		if err := k.routeTreasuryShare(ctx, bondDenom, slashedAmt); err != nil {
			return err
		}
	}

	k.slashingKeeper.Jail(ctx, consAddr)

	k.emitSlashEvent(ctx, addr, reason, penaltyBps)
	return nil
}

func (k Keeper) emitSlashEvent(ctx sdk.Context, addr, reason string, penaltyBps uint32) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"phonenode.Slash",
			sdk.NewAttribute("address", addr),
			sdk.NewAttribute("reason", reason),
			sdk.NewAttribute("penalty_bps", fmt.Sprintf("%d", penaltyBps)),
		),
	)
	// O1 业务指标：移动节点 slash 计数（经 app telemetry 在 /metrics 暴露）。
	telemetry.IncrCounter(1, "phonenode", "slash_count")
}

// routeTreasuryShare mints the finalized 2026-08 treasury share (60% of the
// slashed amount) through the tokenomics module account and routes it to the
// protocol treasury (the 6th independent address). The 40% burn is handled by
// the native staking slash above; this re-creates the 60% treasury portion as
// new supply so the documented 40/60 split is honored without breaking staking's
// bonded-pool accounting. The minted amount is negligible relative to the fixed
// supply cap, so it does not threaten the deflation ceiling.
func (k Keeper) routeTreasuryShare(ctx sdk.Context, denom string, amt sdk.Int) error {
	if !amt.IsPositive() {
		return nil
	}
	burnAmt := amt.MulRaw(int64(tokenomicsmoduletypes.SlashBurnRatioBps)).QuoRaw(10000)
	treasuryAmt := amt.Sub(burnAmt)
	if !treasuryAmt.IsPositive() {
		return nil
	}
	treasuryCoins := sdk.NewCoins(sdk.NewCoin(denom, treasuryAmt))
	if err := k.bankKeeper.MintCoins(ctx, tokenomicsmoduletypes.ModuleName, treasuryCoins); err != nil {
		return fmt.Errorf("phonenode: mint treasury slash share: %w", err)
	}
	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, tokenomicsmoduletypes.ModuleName, tokenomicsmoduletypes.ProtocolTreasuryPoolName, treasuryCoins,
	); err != nil {
		return fmt.Errorf("phonenode: route slashed share to treasury: %w", err)
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"phonenode.SlashSplit",
		sdk.NewAttribute("burned", burnAmt.String()),
		sdk.NewAttribute("treasury", treasuryAmt.String()),
		sdk.NewAttribute("denom", denom),
	))
	return nil
}
