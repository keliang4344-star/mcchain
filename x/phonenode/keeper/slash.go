package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

	fraction := sdk.NewDecWithPrec(int64(penaltyBps), 4)
	power := val.GetTokens().Int64()
	k.slashingKeeper.Slash(ctx, consAddr, fraction, power, ctx.BlockHeight()-1)
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
