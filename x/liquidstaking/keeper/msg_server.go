package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/liquidstaking/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns the Msg service implementation backed by the keeper.
func NewMsgServerImpl(k Keeper) types.MsgServer {
	return &msgServer{Keeper: k}
}

var _ types.MsgServer = msgServer{}

// LiquidStake bonds MC through the module account and mints receipt shares.
func (m msgServer) LiquidStake(goCtx context.Context, msg *types.MsgLiquidStake) (*types.MsgLiquidStakeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	delegator, err := sdk.AccAddressFromBech32(msg.Delegator)
	if err != nil {
		return nil, types.ErrInvalidAddress
	}
	valAddr, err := sdk.ValAddressFromBech32(msg.Validator)
	if err != nil {
		return nil, types.ErrInvalidAddress
	}

	shares, err := m.Keeper.LiquidStake(ctx, delegator, valAddr, msg.AmountUmc)
	if err != nil {
		return nil, err
	}
	return &types.MsgLiquidStakeResponse{SharesMintedUlmc: shares}, nil
}

// LiquidUnstake burns receipt shares and starts the unbonding period.
func (m msgServer) LiquidUnstake(goCtx context.Context, msg *types.MsgLiquidUnstake) (*types.MsgLiquidUnstakeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	delegator, err := sdk.AccAddressFromBech32(msg.Delegator)
	if err != nil {
		return nil, types.ErrInvalidAddress
	}

	entry, err := m.Keeper.LiquidUnstake(ctx, delegator, msg.Validator, msg.SharesUlmc)
	if err != nil {
		return nil, err
	}
	return &types.MsgLiquidUnstakeResponse{
		UnbondingId:        entry.ID,
		AmountUmc:          entry.AmountUmc,
		CompletionUnixTime: entry.CompletionUnixTime,
	}, nil
}

// ClaimMatured pays out every unbonding entry that has completed.
func (m msgServer) ClaimMatured(goCtx context.Context, msg *types.MsgClaimMatured) (*types.MsgClaimMaturedResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	delegator, err := sdk.AccAddressFromBech32(msg.Delegator)
	if err != nil {
		return nil, types.ErrInvalidAddress
	}

	before := m.Keeper.GetDelegatorUnbondings(ctx, msg.Delegator)
	paid, err := m.Keeper.ClaimMatured(ctx, delegator)
	if err != nil {
		return nil, err
	}
	after := m.Keeper.GetDelegatorUnbondings(ctx, msg.Delegator)

	settled := uint64(0)
	if len(before) > len(after) {
		settled = uint64(len(before) - len(after))
	}
	return &types.MsgClaimMaturedResponse{AmountUmc: paid, EntriesClaimed: settled}, nil
}
