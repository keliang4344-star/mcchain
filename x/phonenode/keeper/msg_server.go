package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/phonenode/types"
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

// UpdateVerifierStatus 处理更新节点验证者状态的消息。
func (k msgServer) UpdateVerifierStatus(goCtx context.Context, msg *types.MsgUpdateVerifierStatus) (*types.MsgUpdateVerifierStatusResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.Keeper.UpdateVerifierStatus(ctx, msg.NodeId, msg.Status); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"phonenode.VerifierStatusUpdated",
			sdk.NewAttribute("node_id", msg.NodeId),
			sdk.NewAttribute("status", msg.Status),
		),
	)

	return &types.MsgUpdateVerifierStatusResponse{}, nil
}

// RegisterCloudSigner 处理「为节点绑定云端共签方公钥」的消息（Path C 手机-云端共签）。
// 签名者即为节点地址，防止第三方替他人绑定共签方。
func (k msgServer) RegisterCloudSigner(goCtx context.Context, msg *types.MsgRegisterCloudSigner) (*types.MsgRegisterCloudSignerResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.Keeper.RegisterCloudSigner(ctx, msg.Creator, msg.CloudPubKey); err != nil {
		return nil, err
	}

	return &types.MsgRegisterCloudSignerResponse{}, nil
}

// SubmitCosign 处理「提交云端共签」的消息：链上用已绑定的云端公钥验签，
// 通过则持久化共签证明并发出事件。
func (k msgServer) SubmitCosign(goCtx context.Context, msg *types.MsgSubmitCosign) (*types.MsgSubmitCosignResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	if err := k.Keeper.SubmitCosign(ctx, msg.Creator, msg.PayloadHash, msg.CloudSignature); err != nil {
		return nil, err
	}

	return &types.MsgSubmitCosignResponse{}, nil
}
