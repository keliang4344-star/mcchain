package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/dex/types"
)

type msgServer struct {
	Keeper
}

func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (m msgServer) CreatePool(goCtx context.Context, msg *types.MsgCreatePool) (*types.MsgCreatePoolResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	amountA, ok := sdk.NewIntFromString(msg.AmountA)
	if !ok || amountA.LTE(sdk.ZeroInt()) {
		return nil, types.ErrZeroAmount
	}
	amountB, ok := sdk.NewIntFromString(msg.AmountB)
	if !ok || amountB.LTE(sdk.ZeroInt()) {
		return nil, types.ErrZeroAmount
	}

	pool, err := m.Keeper.CreatePool(ctx, msg.DenomA, msg.DenomB, amountA, amountB, msg.FeeRateBps, msg.Creator, msg.PoolId)
	if err != nil {
		return nil, err
	}

	return &types.MsgCreatePoolResponse{PoolId: pool.Id}, nil
}

func (m msgServer) AddLiquidity(goCtx context.Context, msg *types.MsgAddLiquidity) (*types.MsgAddLiquidityResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	amountAMax, ok := sdk.NewIntFromString(msg.AmountAMax)
	if !ok {
		return nil, types.ErrZeroAmount
	}
	amountBMax, ok := sdk.NewIntFromString(msg.AmountBMax)
	if !ok {
		return nil, types.ErrZeroAmount
	}
	minLPOut, ok := sdk.NewIntFromString(msg.MinLpOut)
	if !ok {
		minLPOut = sdk.ZeroInt()
	}

	lpMinted, actualA, actualB, err := m.Keeper.AddLiquidity(ctx, msg.PoolId, amountAMax, amountBMax, minLPOut, msg.Creator)
	if err != nil {
		return nil, err
	}

	return &types.MsgAddLiquidityResponse{
		LpMinted: lpMinted.String(),
		ActualA:  actualA.String(),
		ActualB:  actualB.String(),
	}, nil
}

func (m msgServer) RemoveLiquidity(goCtx context.Context, msg *types.MsgRemoveLiquidity) (*types.MsgRemoveLiquidityResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	lpAmount, ok := sdk.NewIntFromString(msg.LpAmount)
	if !ok || lpAmount.LTE(sdk.ZeroInt()) {
		return nil, types.ErrZeroAmount
	}
	minAOut, ok := sdk.NewIntFromString(msg.MinAOut)
	if !ok {
		minAOut = sdk.ZeroInt()
	}
	minBOut, ok := sdk.NewIntFromString(msg.MinBOut)
	if !ok {
		minBOut = sdk.ZeroInt()
	}

	amountA, amountB, err := m.Keeper.RemoveLiquidity(ctx, msg.PoolId, lpAmount, minAOut, minBOut, msg.Creator)
	if err != nil {
		return nil, err
	}

	return &types.MsgRemoveLiquidityResponse{
		AmountA: amountA.String(),
		AmountB: amountB.String(),
	}, nil
}

func (m msgServer) SwapExactIn(goCtx context.Context, msg *types.MsgSwapExactIn) (*types.MsgSwapExactInResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	amountIn, ok := sdk.NewIntFromString(msg.AmountIn)
	if !ok || amountIn.LTE(sdk.ZeroInt()) {
		return nil, types.ErrZeroAmount
	}
	minAmountOut, ok := sdk.NewIntFromString(msg.MinAmountOut)
	if !ok {
		minAmountOut = sdk.ZeroInt()
	}

	amountOut, err := m.Keeper.SwapExactIn(ctx, msg.PoolId, msg.DenomIn, msg.DenomOut, amountIn, minAmountOut, msg.Creator)
	if err != nil {
		return nil, err
	}

	return &types.MsgSwapExactInResponse{AmountOut: amountOut.String()}, nil
}
