package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/phonenode/types"
)

// RegisterNode 注册手机节点。
//
// AUTH-1 修复（阻断级授权缺陷）：GetSigners() 只认 Creator，而此前处理器用的是
// 未经认证的 msg.Address。Keeper.RegisterNode 对已存在地址返回 ErrNodeExists，
// 因此攻击者可批量抢注他人地址的节点槽位，令其永久无法入网、无法领取节点资本津贴。
// 节点身份必须与交易签名者严格同一。
func (k msgServer) RegisterNode(goCtx context.Context, msg *types.MsgRegisterNode) (*types.MsgRegisterNodeResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 签名者身份即节点身份（AUTH-1）
	nodeAddr, err := resolveSelfAddress(msg.Creator, msg.Address, "node")
	if err != nil {
		return nil, err
	}

	if _, err := k.Keeper.RegisterNode(ctx, nodeAddr, msg.Model, msg.Os, msg.Role); err != nil {
		return nil, err
	}

	return &types.MsgRegisterNodeResponse{}, nil
}

// resolveSelfAddress 校验「目标地址 == 交易签名者」并返回规范化后的地址。
// 语义与 x/depin 中同名函数一致（AUTH-1）：target 为空时取 creator，
// 非空时必须与 creator 完全相等，杜绝代他人创建链上身份记录。
func resolveSelfAddress(creator, target, what string) (string, error) {
	if _, err := sdk.AccAddressFromBech32(creator); err != nil {
		return "", sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	if target == "" {
		return creator, nil
	}
	if _, err := sdk.AccAddressFromBech32(target); err != nil {
		return "", sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid %s address (%s)", what, err)
	}
	if target != creator {
		return "", sdkerrors.Wrapf(
			sdkerrors.ErrUnauthorized,
			"%s address %s must equal the transaction signer %s", what, target, creator,
		)
	}
	return target, nil
}
