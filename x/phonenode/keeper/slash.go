package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"mcchain/x/phonenode/types"
	tokenomicsmoduletypes "mcchain/x/tokenomics/types"
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
//  3. 若节点为 bonded 验证人：按 penaltyBps 比例扣自质押，罚没资金 100% 转入
//     质押安全池，随后 Jail；非验证人节点不罚币，仅吊销 attestation + 记录
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

	// === 罚没资金去向（2026-08 定稿）===
	// 被罚没的自质押 100% 回流质押安全池，成为对诚实节点与验证人的补贴：
	// 作恶者的损失，变成诚实者的收益。
	//
	// 罚没不属于通缩销毁的四类来源（DePIN 赏金 5% / DEX 手续费 0.05% /
	// 推荐加成 1% / 治理押金 10%），因此这里既不烧币、也不新印，只做转移。
	//
	// 实现说明：Cosmos SDK v0.47 的 slashing.Slash 内部无条件调用
	// burnBondedTokens 烧毁全部被罚金额，没有任何开关可以改变去向
	//（BurnSlashTokens 参数要到 v0.50+ 才有），故此处以 slashToSecurityPool
	// 完成等价动作。Jail 仍照常执行以吊销出块权。
	fraction := sdk.NewDecWithPrec(int64(penaltyBps), 4)
	if err := k.slashToSecurityPool(ctx, valAddr, fraction, addr, reason); err != nil {
		return err
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

// slashToSecurityPool 扣减验证人被罚没的自质押，并把等额资金 100% 转入质押安全池。
//
// 这是原生 slashing.Slash 的等价替代（SDK v0.47 的 Slash 会强制烧毁被罚金额，
// 不满足「罚没回流安全池」的既定原则），按与 SDK 相同的顺序完成三件事：
//
//  1. 触发 distribution 的 BeforeValidatorSlashed 钩子。必须在扣减 tokens 之前调用，
//     否则委托人的历史奖励会按未罚没前的份额计算，导致超额提取。
//  2. RemoveValidatorTokens 扣减验证人 tokens 并刷新 power index。
//     只减 tokens 不减 shares，正是 slash 的语义：每一份委托的含金量下降。
//  3. 把等额资金从 staking bonded pool 转入质押安全池。
//
// 不变量安全性：v0.47 中 TotalBondedTokens 直接读取 bonded pool 模块账户余额，
// 并无独立计数器；staking 的 ModuleAccountInvariants 校验
//「bonded pool 余额 == 所有 bonded 验证人 tokens 之和」。上述第 2、3 步等额同向
// 变动，两侧始终一致，不变量恒成立。
//
// 已知边界：本实现只罚没验证人当前处于 bonded 状态的自质押，不追溯正在解绑
//（unbonding）或再委托（redelegation）中的份额——SDK 的对应处理函数未导出，
// 无法从模块外复用。检测到作恶后本链即时罚没，逃逸窗口仅为区块级，风险可控；
// 彻底覆盖需待升级到提供 BurnSlashTokens 开关的 SDK 版本。
func (k Keeper) slashToSecurityPool(
	ctx sdk.Context, valAddr sdk.ValAddress, fraction sdk.Dec, addr, reason string,
) error {
	if !fraction.IsPositive() {
		return nil
	}
	validator, found := k.stakingKeeper.GetValidator(ctx, valAddr)
	if !found {
		return nil
	}
	slashedAmt := fraction.MulInt(validator.Tokens).TruncateInt()
	if slashedAmt.GT(validator.Tokens) {
		slashedAmt = validator.Tokens
	}
	if !slashedAmt.IsPositive() {
		return nil
	}

	// 1. distribution 钩子：必须先于 tokens 扣减执行。
	if err := k.stakingKeeper.Hooks().BeforeValidatorSlashed(ctx, valAddr, fraction); err != nil {
		return fmt.Errorf("phonenode: before-validator-slashed hook for %s: %w", addr, err)
	}

	// 2. 扣减验证人 tokens（刷新 power index）。
	k.stakingKeeper.RemoveValidatorTokens(ctx, validator, slashedAmt)

	// 3. 等额资金由 bonded pool 转入质押安全池：不烧毁、不新印，全链总量不变。
	bondDenom := k.stakingKeeper.BondDenom(ctx)
	slashedCoins := sdk.NewCoins(sdk.NewCoin(bondDenom, slashedAmt))
	if err := k.bankKeeper.SendCoinsFromModuleToModule(
		ctx, stakingtypes.BondedPoolName, tokenomicsmoduletypes.StakingSecurityPoolName, slashedCoins,
	); err != nil {
		return fmt.Errorf("phonenode: route slashed stake to security pool: %w", err)
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"phonenode.SlashRouted",
		sdk.NewAttribute("address", addr),
		sdk.NewAttribute("reason", reason),
		sdk.NewAttribute("slashed", slashedAmt.String()),
		sdk.NewAttribute("destination", tokenomicsmoduletypes.StakingSecurityPoolName),
		sdk.NewAttribute("burned", "0"),
		sdk.NewAttribute("minted", "0"),
		sdk.NewAttribute("denom", bondDenom),
	))
	telemetry.IncrCounter(float32(slashedAmt.Int64()), "phonenode", "slashed_to_security_pool")
	return nil
}
