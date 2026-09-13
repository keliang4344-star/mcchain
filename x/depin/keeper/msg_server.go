package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// SubmitAttestation 处理预言机提交的设备 attestation 验证结果。
func (k msgServer) SubmitAttestation(goCtx context.Context, msg *types.MsgSubmitAttestation) (*types.MsgSubmitAttestationResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// [2]/[7] 预言机白名单闸门：生产链强制，开发/测试链豁免（兼容 SoftOracle）。
	// 交易签名者即 msg.OracleAddress（GetSigners 已将其作为唯一签名者，由 ante 验签），
	// 此处校验其是否在被授权白名单内；生产链白名单为空直接 fail-closed，杜绝任意地址冒充预言机。
	if isProductionChainID(ctx.ChainID()) {
		wl := k.Keeper.GetOracleWhitelist(ctx)
		if len(wl) == 0 {
			return nil, types.ErrOracleNotConfigured
		}
		if !oracleInWhitelist(msg.OracleAddress, wl) {
			return nil, types.ErrUnauthorizedOracle
		}
	}

	// 预言机离线验证设备 attestation，将结果写入链上
	passed, reason := k.Keeper.VerifyDeviceAttestation(ctx, msg.DeviceId, msg.AttestationProof, msg.Signature)

	// 存储验证结果
	result := types.NewAttestationResult(msg.DeviceId, passed, reason, msg.OracleAddress, ctx.BlockTime().Unix())
	if err := k.Keeper.StoreAttestationResult(ctx, msg.DeviceId, result); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"depin.AttestationResult",
			sdk.NewAttribute("device_id", msg.DeviceId),
			sdk.NewAttribute("passed", fmtBool(passed)),
			sdk.NewAttribute("reason", reason),
			sdk.NewAttribute("oracle", msg.OracleAddress),
		),
	)

	return &types.MsgSubmitAttestationResponse{
		Passed: passed,
		Reason: reason,
	}, nil
}

func fmtBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
