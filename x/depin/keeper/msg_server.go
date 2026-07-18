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

	// 预言机离线验证设备 attestation，将结果写入链上
	passed, reason := k.Keeper.VerifyDeviceAttestation(ctx, msg.DeviceId, msg.AttestationProof, msg.Signature)

	// 存储验证结果
	result := types.NewAttestationResult(msg.DeviceId, passed, reason, msg.OracleAddress)
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
