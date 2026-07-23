package keeper

import (
	"encoding/binary"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

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

	// Cooldown check: find the most recent referral from this inviter
	referrals := k.GetReferralsByInviter(ctx, inviter)
	if len(referrals) > 0 {
		lastRef := referrals[len(referrals)-1]
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

	ctx.EventManager().EmitTypedEvent(&ReferralCreatedEvent{
		ReferralId: referralID,
		Inviter:    inviter,
		Invitee:    invitee,
	})

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

// ---------------------------------------------------------------------------
// 待领奖励
// ---------------------------------------------------------------------------

// TrackReward is called when an invitee earns a reward (e.g., from completing a DePIN task).
// It credits the inviter with a proportion (rewardRateBps) of the invitee's reward,
// sourced from the ecosystem module account.
//
// This should be called as a hook from the depin/edgeai reward distribution path,
// NOT from the invitee's own reward—the referral reward is additional and paid by
// the ecosystem fund.
func (k Keeper) TrackReward(ctx sdk.Context, invitee string, rewardAmount sdkmath.Int) error {
	params := k.GetParams(ctx)

	refID, found := k.getReferralIDByInvitee(ctx, invitee)
	if !found {
		// Invitee has no inviter; nothing to track.
		return nil
	}

	ref, found := k.GetReferral(ctx, refID)
	if !found || ref.Status != types.ReferralStatusActive {
		return nil
	}

	// Calculate referral bonus: rewardAmount * rewardRateBps / 10000
	bonus := rewardAmount.Mul(sdkmath.NewInt(int64(params.RewardRateBps))).Quo(sdkmath.NewInt(10000))
	if bonus.IsZero() {
		return nil
	}

	// Daily cap check (white paper lines 528-540)
	if err := k.CheckDailyCaps(ctx, ref.Inviter, bonus); err != nil {
		return err
	}

	current := k.getPendingRewards(ctx, ref.Inviter)
	newTotal := current.Add(bonus)
	k.setPendingRewards(ctx, ref.Inviter, newTotal)

	// Record daily cap usage
	k.RecordDailyCapUsage(ctx, ref.Inviter, bonus)

	ctx.EventManager().EmitTypedEvent(&ReferralRewardTrackedEvent{
		Inviter: ref.Inviter,
		Invitee: invitee,
		Amount:  bonus,
	})

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

	reward := sdk.NewCoins(sdk.NewCoin("umc", pending))
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.EcosystemModuleAccount, claimerAddr, reward); err != nil {
		return sdk.Coin{}, err
	}

	// Reset pending rewards
	k.setPendingRewards(ctx, claimer, sdkmath.ZeroInt())

	ctx.EventManager().EmitTypedEvent(&ReferralRewardClaimedEvent{
		Claimer: claimer,
		Amount:  pending,
	})

	return sdk.NewCoin("umc", pending), nil
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

func (k Keeper) setPendingRewards(ctx sdk.Context, inviter string, amount sdkmath.Int) {
	store := ctx.KVStore(k.storeKey)
	bz, err := amount.Marshal()
	if err != nil {
		panic(fmt.Sprintf("failed to marshal pending rewards: %v", err))
	}
	store.Set([]byte(types.PendingRewardKeyPrefix+inviter), bz)
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

// ---------------------------------------------------------------------------
// 事件类型（手动定义，proto 生成前使用）
// ---------------------------------------------------------------------------

type ReferralCreatedEvent struct {
	ReferralId uint64 `json:"referral_id"`
	Inviter    string `json:"inviter"`
	Invitee    string `json:"invitee"`
}

func (e *ReferralCreatedEvent) Reset()         {}
func (e *ReferralCreatedEvent) String() string { return "ReferralCreated" }
func (e *ReferralCreatedEvent) ProtoMessage()  {}

type ReferralRewardTrackedEvent struct {
	Inviter string        `json:"inviter"`
	Invitee string        `json:"invitee"`
	Amount  sdkmath.Int   `json:"amount"`
}

func (e *ReferralRewardTrackedEvent) Reset()         {}
func (e *ReferralRewardTrackedEvent) String() string { return "ReferralRewardTracked" }
func (e *ReferralRewardTrackedEvent) ProtoMessage()  {}

type ReferralRewardClaimedEvent struct {
	Claimer string      `json:"claimer"`
	Amount  sdkmath.Int `json:"amount"`
}

func (e *ReferralRewardClaimedEvent) Reset()         {}
func (e *ReferralRewardClaimedEvent) String() string { return "ReferralRewardClaimed" }
func (e *ReferralRewardClaimedEvent) ProtoMessage()  {}
