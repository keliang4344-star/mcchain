package keeper_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/mcchain/keeper"
	"mcchain/x/mcchain/types"
)

// TestMsgServerHandoverHappyPath 完整链上移交流程：
// 当前治理主体发起 → 时间锁未到期拒绝 → 到期后执行 → 治理主体更新为新地址。
func TestMsgServerHandoverHappyPath(t *testing.T) {
	k, ctx := setupHandoverKeeper(t)
	ms := keeper.NewMsgServerImpl(*k)

	governor := newGovernorAddr()
	newGov := newGovernorAddr()

	cfg := k.GetGovernanceHandoverConfig(ctx)
	cfg.Enabled = true
	cfg.CurrentGovernor = governor
	cfg.TimelockBlocks = 100
	k.SetGovernanceHandoverConfig(ctx, cfg)

	startCtx := ctx.WithBlockHeight(10)
	initRes, err := ms.InitiateHandover(sdk.WrapSDKContext(startCtx), types.NewMsgInitiateHandover(governor, newGov))
	require.NoError(t, err)
	require.Equal(t, int64(110), initRes.ActivationHeight)

	// 时间锁未到期 → 拒绝执行
	_, err = ms.CompleteHandover(sdk.WrapSDKContext(ctx.WithBlockHeight(109)), types.NewMsgCompleteHandover(governor))
	require.Error(t, err)
	require.Contains(t, err.Error(), "timelock not elapsed")

	// 到期后执行成功
	doneRes, err := ms.CompleteHandover(sdk.WrapSDKContext(ctx.WithBlockHeight(110)), types.NewMsgCompleteHandover(governor))
	require.NoError(t, err)
	require.Equal(t, newGov, doneRes.NewGovernor)

	after := k.GetGovernanceHandoverConfig(ctx)
	require.True(t, after.Executed)
	require.Equal(t, newGov, after.CurrentGovernor, "移交后治理主体应更新为新地址")

	// 终态：不可重复执行
	_, err = ms.CompleteHandover(sdk.WrapSDKContext(ctx.WithBlockHeight(200)), types.NewMsgCompleteHandover(newGov))
	require.Error(t, err)
	require.Contains(t, err.Error(), "already executed")
}

// TestMsgServerHandoverRejectsUnauthorized 非当前治理主体不得发起或执行移交。
func TestMsgServerHandoverRejectsUnauthorized(t *testing.T) {
	k, ctx := setupHandoverKeeper(t)
	ms := keeper.NewMsgServerImpl(*k)

	governor := newGovernorAddr()
	attacker := newGovernorAddr()
	newGov := newGovernorAddr()

	cfg := k.GetGovernanceHandoverConfig(ctx)
	cfg.Enabled = true
	cfg.CurrentGovernor = governor
	k.SetGovernanceHandoverConfig(ctx, cfg)

	_, err := ms.InitiateHandover(sdk.WrapSDKContext(ctx), types.NewMsgInitiateHandover(attacker, newGov))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")

	_, err = ms.CompleteHandover(sdk.WrapSDKContext(ctx), types.NewMsgCompleteHandover(attacker))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unauthorized")
}

// TestMsgServerHandoverRejectsUnconfiguredGovernor 未配置治理主体时，链上移交消息一律拒绝。
func TestMsgServerHandoverRejectsUnconfiguredGovernor(t *testing.T) {
	k, ctx := setupHandoverKeeper(t)
	ms := keeper.NewMsgServerImpl(*k)

	cfg := k.GetGovernanceHandoverConfig(ctx)
	cfg.Enabled = true
	k.SetGovernanceHandoverConfig(ctx, cfg)

	_, err := ms.InitiateHandover(sdk.WrapSDKContext(ctx), types.NewMsgInitiateHandover(newGovernorAddr(), newGovernorAddr()))
	require.Error(t, err)
	require.Contains(t, err.Error(), "current governor is not configured")
}

// TestHandoverMsgValidateBasic 消息层面基础校验。
func TestHandoverMsgValidateBasic(t *testing.T) {
	good := newGovernorAddr()
	other := newGovernorAddr()

	require.NoError(t, types.NewMsgInitiateHandover(good, other).ValidateBasic())
	require.Error(t, types.NewMsgInitiateHandover("not-an-address", other).ValidateBasic())
	require.Error(t, types.NewMsgInitiateHandover(good, "not-an-address").ValidateBasic())
	require.Error(t, types.NewMsgInitiateHandover(good, good).ValidateBasic(), "新治理主体不得与当前主体相同")

	require.NoError(t, types.NewMsgCompleteHandover(good).ValidateBasic())
	require.Error(t, types.NewMsgCompleteHandover("").ValidateBasic())

	signers := types.NewMsgCompleteHandover(good).GetSigners()
	require.Len(t, signers, 1)
	require.Equal(t, good, signers[0].String())
}
