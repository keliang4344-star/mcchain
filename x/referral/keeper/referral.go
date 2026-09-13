package keeper

import (
	"encoding/binary"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/internal/safemath"
	"mcchain/x/referral/types"
)

// ---------------------------------------------------------------------------
// 推荐 CRUD
// ---------------------------------------------------------------------------

// CreateReferral creates a new referral record and returns its ID.
// Anti-sybil checks (invitee is phonenode, not already referred, not self-referral,
// cooldown, max referrals) are enforced in the message handler.
func (k Keeper) CreateReferral(
	ctx sdk.Context,
	inviter string,
	invitee string,
	inviteCode string,
) (uint64, error) {
	params := k.GetParams(ctx)

	// Anti-sybil: invitee must be a registered phonenode
	if !k.phonenodeKeeper.HasNode(ctx, invitee) {
		return 0, types.ErrInviteeNotRegistered
	}

	// Anti-sybil: invitee cannot already be referred
	if _, found := k.getReferralIDByInvitee(ctx, invitee); found {
		return 0, types.ErrInviteeAlreadyReferred
	}

	// Anti-sybil: cannot refer yourself
	if inviter == invitee {
		return 0, types.ErrSelfReferral
	}

	// Cycle guard: adding inviter -> invitee must not create a cycle in the
	// referral graph (e.g., A refers B, B refers C, C refers A). Walk the
	// existing parent chain upward from the new inviter up to MaxReferralDepth;
	// if the new invitee is already an ancestor, reject the edge.
	if k.wouldCreateCycle(ctx, inviter, invitee) {
		return 0, types.ErrReferralCycle
	}

	// Cooldown check: find the most recent referral from this inviter
	referrals := k.GetReferralsByInviter(ctx, inviter)
	if len(referrals) > 0 {
		lastRef := referrals[len(referrals)-1]
		// 先做方向守卫。若创世/迁移数据里 CreatedAt
		// 大于当前高度，uint64 减法会回绕成天文数字，冷却检查被静默绕过。
		if lastRef.CreatedAt > ctx.BlockHeight() {
			return 0, types.ErrCooldownNotElapsed
		}
		if uint64(ctx.BlockHeight())-uint64(lastRef.CreatedAt) < params.CooldownBlocks {
			return 0, types.ErrCooldownNotElapsed
		}
	}

	// Max referrals check
	if uint64(len(referrals)) >= params.MaxReferralsPerUser {
		return 0, types.ErrMaxReferralsReached
	}

	referralID := k.nextReferralID(ctx)

	referral := types.Referral{
		ReferralId: referralID,
		Inviter:    inviter,
		Invitee:    invitee,
		InviteCode: inviteCode,
		CreatedAt:  ctx.BlockHeight(),
		Status:     types.ReferralStatusActive,
	}

	k.setReferral(ctx, referral)
	k.setReferralByInviter(ctx, inviter, referralID)
	k.setReferralByInvitee(ctx, invitee, referralID)

	// 改用标准 EmitEvent。
	// 原手写 TypedEvent 结构体未注册 proto 描述符，SDK 的 TypedEventToEvent
	// 对未注册消息产出空类型 + 空属性 —— 链上事件流里根本没有可索引内容，
	// 下游监控与索引全部失效。普通 Event 携带显式属性，兼容所有事件消费端。
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"referral.created",
		sdk.NewAttribute("referral_id", fmt.Sprintf("%d", referralID)),
		sdk.NewAttribute("inviter", inviter),
		sdk.NewAttribute("invitee", invitee),
	))

	return referralID, nil
}

// GetReferral retrieves a referral by ID.
func (k Keeper) GetReferral(ctx sdk.Context, referralID uint64) (types.Referral, bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefix(types.ReferralKeyPrefix))

	bz := store.Get(uint64Key(referralID))
	if bz == nil {
		return types.Referral{}, false
	}

	var referral types.Referral
	k.cdc.MustUnmarshal(bz, &referral)
	return referral, true
}

// GetReferralsByInviter returns all referrals made by a given inviter.
func (k Keeper) GetReferralsByInviter(ctx sdk.Context, inviter string) []types.Referral {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefix(types.ReferralByInviterPrefix+inviter+"/"))

	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	var referrals []types.Referral
	for ; iterator.Valid(); iterator.Next() {
		refID := binary.BigEndian.Uint64(iterator.Value())
		ref, found := k.GetReferral(ctx, refID)
		if found {
			referrals = append(referrals, ref)
		}
	}
	return referrals
}

// wouldCreateCycle reports whether adding the edge inviter -> invitee would
// introduce a cycle in the referral graph. It walks the existing parent chain
// upward from inviter (following who referred inviter, then that node's
// referrer, ...) up to MaxReferralDepth levels. If any ancestor equals invitee,
// the new edge would close a loop and the function returns true.
func (k Keeper) wouldCreateCycle(ctx sdk.Context, inviter, invitee string) bool {
	current := inviter
	for depth := uint32(0); depth < types.MaxReferralDepth; depth++ {
		refID, found := k.getReferralIDByInvitee(ctx, current)
		if !found {
			return false
		}
		ref, found := k.GetReferral(ctx, refID)
		if !found || ref.Status != types.ReferralStatusActive {
			return false
		}
		if ref.Inviter == invitee {
			// invitee is already an ancestor of inviter -> cycle.
			return true
		}
		current = ref.Inviter
	}
	return false
}

// ---------------------------------------------------------------------------
// 待领奖励
// ---------------------------------------------------------------------------

// TrackReward is called when an invitee earns a reward (e.g., from completing a DePIN / EdgeAI task).
// It walks the referral chain up to 10 levels (whitepaper §25: 10 代分级推荐)
// and credits each ancestor with their proportional share, sourced from the ecosystem module account.
//
// 10 代分级：
//   - 权重表：一代 10% / 二代 5% / 三代 3% / 四代 2% / 五代 1% / 六~十代各 0.5%，
//     合计 23.5% = 网络为单一下线支付的获客成本上限（由 8250 万独立池拨付）；
//   - 挖矿节点（注册 + 认证有效 + 心跳在宽限内）可享受满 10 代收益；
//   - 非挖矿节点只享受前 5 代收益（第 6 代起该祖先不再分得）。
//
// Each level independently checks daily caps (per user and network-wide).
func (k Keeper) TrackReward(ctx sdk.Context, invitee string, rewardAmount sdkmath.Int) error {
	params := k.GetParams(ctx)

	currentInvitee := invitee
	// 10 级权重：前 3 档取治理参数（可调），第 4-10 档为链上常量。
	rates := [10]uint32{
		params.Level1RewardRateBps, params.Level2RewardRateBps, params.Level3RewardRateBps,
		types.DefaultLevel4RewardRateBps, types.DefaultLevel5RewardRateBps,
		types.DefaultLevel6RewardRateBps, types.DefaultLevel7RewardRateBps,
		types.DefaultLevel8RewardRateBps, types.DefaultLevel9RewardRateBps,
		types.DefaultLevel10RewardRateBps,
	}

	for level := 0; level < int(types.MaxReferralDepth); level++ {
		// 10 代分级：非节点祖先在第 6 代起不再获得收益（链条仍继续向上，
		// 上级若是节点仍可吃到更深层的贡献）。
		if level >= int(types.NonNodeMaxDepth) {
			if refID, found := k.getReferralIDByInvitee(ctx, currentInvitee); found {
				if ref, found := k.GetReferral(ctx, refID); found && ref.Status == types.ReferralStatusActive {
					if !k.phonenodeKeeper.IsActiveNode(ctx, ref.Inviter) {
						currentInvitee = ref.Inviter
						continue
					}
				}
			}
		}

		rate := rates[level]
		if rate == 0 {
			// 该级无费率不代表链终结——若直接 continue 而不推进
			// currentInvitee，下一级费率会错配到同一祖先。先沿链上移一级再继续。
			if refID, found := k.getReferralIDByInvitee(ctx, currentInvitee); found {
				if ref, found := k.GetReferral(ctx, refID); found && ref.Status == types.ReferralStatusActive {
					currentInvitee = ref.Inviter
					continue
				}
			}
			return nil
		}

		refID, found := k.getReferralIDByInvitee(ctx, currentInvitee)
		if !found {
			// No more ancestors in the chain.
			return nil
		}

		ref, found := k.GetReferral(ctx, refID)
		if !found || ref.Status != types.ReferralStatusActive {
			return nil
		}

		// Calculate bonus: rewardAmount * rate / 10000
		bonus := rewardAmount.Mul(sdkmath.NewInt(int64(rate))).Quo(sdkmath.NewInt(10000))
		if bonus.IsZero() {
			// Move up the chain: inviter becomes the next invitee.
			currentInvitee = ref.Inviter
			continue
		}

		// Daily cap check (whitepaper lines 528-540)
		if err := k.CheckDailyCaps(ctx, ref.Inviter, bonus); err != nil {
			// 不再静默丢弃。
			//
			// 旧行为是 `continue` —— 触顶的那一笔直接消失，不延后、不补发、不发事件。
			// 而日上限被击穿的时刻恰好就是增长最快的时刻，也就是最需要留住推广者的
			// 时刻；此时团队长看到「我拉的人在产出收益、我却一分钱没拿到」，且链上
			// 没有任何信息告诉他原因。静默才是这条设计真正的缺陷。
			//
			// 现改为记入延后账本，由 BeginBlock 的 DrainDeferred 按 FIFO 在后续有额度
			// 的日子补发，并发 RewardDeferred 事件。
			//
			// 唯一例外是预算已耗尽：此时继续延后只会让账本无界增长，所以放弃本笔并发出
			// BudgetExhausted 告警事件——这是必须被运维看见的终态，同样不是静默。
			if k.EcosystemBalance(ctx).IsPositive() {
				k.deferReward(ctx, ref.Inviter, bonus)
			} else {
				ctx.EventManager().EmitEvent(sdk.NewEvent(
					types.EventTypeReferralBudgetExhausted,
					sdk.NewAttribute(types.AttrInviter, ref.Inviter),
					sdk.NewAttribute(types.AttrAmount, bonus.String()),
					sdk.NewAttribute(types.AttrReason, types.DeferReasonDailyCap),
				))
			}
			// 单级触顶不应阻断整条链的其余层级：继续向上追溯。
			currentInvitee = ref.Inviter
			continue
		}

		current := k.getPendingRewards(ctx, ref.Inviter)
		newTotal := current.Add(bonus)
		k.setPendingRewards(ctx, ref.Inviter, newTotal)

		// Record daily cap usage
		k.RecordDailyCapUsage(ctx, ref.Inviter, bonus)

		// 改用标准 EmitEvent（原 TypedEvent 未注册
		// proto 描述符，链上事件为空，见上方 referral.created 注释）。
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"referral.reward_tracked",
			sdk.NewAttribute("inviter", ref.Inviter),
			sdk.NewAttribute("invitee", invitee),
			sdk.NewAttribute("amount", bonus.String()),
		))

		// Move up the chain: inviter becomes the next invitee.
		currentInvitee = ref.Inviter
	}

	return nil
}

// GetPendingRewards returns the total pending rewards for a given inviter.
func (k Keeper) GetPendingRewards(ctx sdk.Context, inviter string) sdk.Coin {
	amount := k.getPendingRewards(ctx, inviter)
	return sdk.NewCoin("umc", amount)
}

// ClaimRewards claims all pending rewards for the inviter.
// Rewards are paid from the ecosystem module account.
func (k Keeper) ClaimRewards(ctx sdk.Context, claimer string) (sdk.Coin, error) {
	params := k.GetParams(ctx)

	pending := k.getPendingRewards(ctx, claimer)
	if pending.IsZero() {
		return sdk.Coin{}, types.ErrNoPendingRewards
	}

	minPayout, ok := sdkmath.NewIntFromString(params.MinPayout)
	if !ok {
		return sdk.Coin{}, fmt.Errorf("invalid min_payout param: %s", params.MinPayout)
	}
	if pending.LT(minPayout) {
		return sdk.Coin{}, types.ErrBelowMinPayout
	}

	// Pay from ecosystem module account
	claimerAddr, err := sdk.AccAddressFromBech32(claimer)
	if err != nil {
		return sdk.Coin{}, fmt.Errorf("invalid claimer address: %w", err)
	}

	// 推荐奖励 100% 足额发放给推荐人（白皮书《优化定稿版》§24.6 否决清单）。
	// 早期版本曾在领取时抽取 1% 打入黑洞，该设计已撤销：推荐奖励来自设备池内
	// 专项子预算（额外拨付，不从被推荐人收益中扣减），属参与者应得，不承担通缩职能。
	// 全链通缩仅来自协议使用费（gas 7%、DEX 手续费的 50%）与作恶罚没（40%）。
	payoutAmt := pending
	payout := sdk.NewCoins(sdk.NewCoin(types.BaseDenom, payoutAmt))

	// 预算耗尽时给出可区分的错误码，而不是让下游撞到 bank 的通用
	// 「insufficient funds」。前者可被前端明确提示、可被监控按错误码告警；
	// 后者与「参数写错」「账户不存在」等情形混在一起，无法定性。
	if k.EcosystemBalance(ctx).LT(payoutAmt) {
		return sdk.Coin{}, types.ErrReferralBudgetExhausted.Wrapf(
			"ecosystem budget insufficient: remaining=%s, pending=%s",
			k.EcosystemBalance(ctx).String(), payoutAmt.String())
	}

	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.EcosystemModuleAccount, claimerAddr, payout); err != nil {
		return sdk.Coin{}, err
	}
	telemetry.IncrCounter(safemath.Float32(payoutAmt), "referral", "rewards_claimed")

	// Reset pending rewards
	k.setPendingRewards(ctx, claimer, sdkmath.ZeroInt())

	// 改用标准 EmitEvent（原 TypedEvent 未注册
	// proto 描述符，链上事件为空，见上方 referral.created 注释）。
	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"referral.reward_claimed",
		sdk.NewAttribute("claimer", claimer),
		sdk.NewAttribute("amount", payoutAmt.String()),
	))

	return sdk.NewCoin("umc", payoutAmt), nil
}

// ---------------------------------------------------------------------------
// 内部存储辅助
// ---------------------------------------------------------------------------

func (k Keeper) nextReferralID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.KeyPrefix(types.ReferralCountKeyPrefix))
	var count uint64
	if bz != nil {
		count = binary.BigEndian.Uint64(bz) + 1
	} else {
		count = 1
	}
	store.Set(types.KeyPrefix(types.ReferralCountKeyPrefix), uint64Bytes(count))
	return count
}

func (k Keeper) setReferral(ctx sdk.Context, referral types.Referral) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefix(types.ReferralKeyPrefix))
	bz := k.cdc.MustMarshal(&referral)
	store.Set(uint64Key(referral.ReferralId), bz)
}

func (k Keeper) setReferralByInviter(ctx sdk.Context, inviter string, referralID uint64) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefix(types.ReferralByInviterPrefix+inviter+"/"))
	store.Set(uint64Key(referralID), uint64Bytes(referralID))
}

func (k Keeper) setReferralByInvitee(ctx sdk.Context, invitee string, referralID uint64) {
	store := ctx.KVStore(k.storeKey)
	key := []byte(types.ReferralByInviteePrefix + invitee)
	store.Set(key, uint64Bytes(referralID))
}

func (k Keeper) getReferralIDByInvitee(ctx sdk.Context, invitee string) (uint64, bool) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get([]byte(types.ReferralByInviteePrefix + invitee))
	if bz == nil {
		return 0, false
	}
	return binary.BigEndian.Uint64(bz), true
}

func (k Keeper) getPendingRewards(ctx sdk.Context, inviter string) sdkmath.Int {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get([]byte(types.PendingRewardKeyPrefix + inviter))
	if bz == nil {
		return sdkmath.ZeroInt()
	}
	var amount sdkmath.Int
	if err := amount.Unmarshal(bz); err != nil {
		return sdkmath.ZeroInt()
	}
	return amount
}

// setPendingRewards 写入某 inviter 的待发奖励，并同步维护全局负债计数。
//
// 这是 pending 余额**唯一**的写入点，因此把全局计数的维护放在这里，
// 而不是散落到每个调用方 —— 单一收口点才不会有「某条路径忘了加/减」的漂移。
// 全局计数用于 EffectiveNetworkCap 的配速基数（余额 − 已承诺 − 延后），
// 保证「累计承诺额」永远不超过预算总额。
func (k Keeper) setPendingRewards(ctx sdk.Context, inviter string, amount sdkmath.Int) {
	if amount.IsNil() || amount.IsNegative() {
		// 负值不参与账务：pending 是负债，负负债没有经济含义。
		// 调用方以 ZeroInt 表示「清零」，走下面的正常路径。
		amount = sdkmath.ZeroInt()
	}
	old := k.getPendingRewards(ctx, inviter)
	delta := amount.Sub(old)

	store := ctx.KVStore(k.storeKey)
	bz, err := amount.Marshal()
	if err != nil {
		panic(fmt.Sprintf("failed to marshal pending rewards: %v", err))
	}
	store.Set([]byte(types.PendingRewardKeyPrefix+inviter), bz)

	if !delta.IsZero() {
		k.setPendingRewardTotal(ctx, k.getPendingRewardTotal(ctx).Add(delta))
	}
}

// getPendingRewardTotal 读取全局「已承诺但未领取」返佣总额。
//
// 缺失（旧状态、人工构造的创世文件）时返回 0 —— 这会略微放宽配速，
// 但不会导致超发：真正的硬约束仍是生态账户余额，ClaimRewards 付不出钱会失败。
func (k Keeper) getPendingRewardTotal(ctx sdk.Context) sdkmath.Int {
	bz := ctx.KVStore(k.storeKey).Get([]byte(types.PendingRewardTotalKey))
	if bz == nil {
		return sdkmath.ZeroInt()
	}
	if v, ok := sdkmath.NewIntFromString(string(bz)); ok && v.IsPositive() {
		return v
	}
	return sdkmath.ZeroInt()
}

func (k Keeper) setPendingRewardTotal(ctx sdk.Context, v sdkmath.Int) {
	if v.IsNil() || v.IsNegative() {
		v = sdkmath.ZeroInt()
	}
	ctx.KVStore(k.storeKey).Set([]byte(types.PendingRewardTotalKey), []byte(v.String()))
}

// PendingRewardTotal 暴露全局待发返佣总额（供查询、告警与配速计算使用）。
func (k Keeper) PendingRewardTotal(ctx sdk.Context) sdkmath.Int {
	return k.getPendingRewardTotal(ctx)
}

// ---------------------------------------------------------------------------
// 序列化辅助
// ---------------------------------------------------------------------------

func uint64Key(n uint64) []byte {
	return uint64Bytes(n)
}

func uint64Bytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}
