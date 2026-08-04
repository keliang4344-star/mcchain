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

	// === Slashed-funds routing (R1 铁律：任何模块均不得新印 MC) ===
	// 原生 staking.Slash 把被罚的自质押（penaltyBps 比例）100% 烧毁，实现通缩；
	// 被罚币绝不重新生成。国库那 60% 份额不新印，而是由质押安全池（既有储备账户）
	// 转账进入协议国库（routeTreasuryShare）。净效果：被罚币 100% 通缩 +
	// 安全池按 60% 划给国库，全链无任何新增 MC。Jail 仍照常执行以吊销出块权。
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
		// 40% 烧毁由上方原生 staking.Slash 完成（100% 烧毁）。
		// 国库 60% 份额从安全池转账进入，绝不调用 MintCoins（R1 铁律）。
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

// routeTreasuryShare routes the documented 60% slash treasury share to the
// protocol treasury (the 6th independent address) WITHOUT minting any new MC.
// Per the R1 iron rule, the treasury share is drawn as a transfer from the
// staking-security pool (a pre-funded reserve), not created by MintCoins. The
// 40% burn portion is already realized by the native staking.Slash call above
// (which burns the full slashed fraction). If the security pool has less than
// the full 60% share available, only the available amount is transferred, so a
// single slash can never exhaust the pool and halt the chain.
func (k Keeper) routeTreasuryShare(ctx sdk.Context, denom string, amt sdk.Int) error {
	if !amt.IsPositive() {
		return nil
	}
	treasuryRatio := sdk.NewDecWithPrec(int64(tokenomicsmoduletypes.SlashTreasuryRatioBps), 4) // 0.60
	treasuryAmt := treasuryRatio.MulInt(amt).TruncateInt()
	if !treasuryAmt.IsPositive() {
		return nil
	}

	// R1 铁律：任何模块都不得新印 MC。国库份额由既有安全池转账进入，不调用 MintCoins。
	srcAddr := tokenomicsmoduletypes.StakingSecurityPoolAddress()
	avail := k.bankKeeper.GetBalance(ctx, srcAddr, denom).Amount
	transferAmt := treasuryAmt
	if transferAmt.GT(avail) {
		transferAmt = avail
	}
	if !transferAmt.IsPositive() {
		return nil
	}
	treasuryCoins := sdk.NewCoins(sdk.NewCoin(denom, transferAmt))
	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, tokenomicsmoduletypes.StakingSecurityPoolName, tokenomicsmoduletypes.ProtocolTreasuryPoolName, treasuryCoins,
	); err != nil {
		return fmt.Errorf("phonenode: route slashed share from security pool to treasury: %w", err)
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"phonenode.SlashSplit",
		sdk.NewAttribute("burned", amt.String()),
		sdk.NewAttribute("treasury", transferAmt.String()),
		sdk.NewAttribute("treasury_source", tokenomicsmoduletypes.StakingSecurityPoolName),
		sdk.NewAttribute("denom", denom),
	))
	return nil
}
