package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/types"
)

// AttestDevice 提交设备认证。
//
// 处理器只认 msg.Address（未经认证的普通字段）时，任何第三方都能
// 为他人设备提交 attestation——既可重放截获的认证材料，也可在软预言机模式下
// 直接把他人设备置为已认证。设备认证必须由设备账户本人签名提交。
//
// 修复（重放 + 过期）：
//  1. challenge 一次性消费——同一对 (challenge, signature) 只能生效一次。
//     只验签不记消费时，截获/复用旧材料即可无限次把 Attested 置真。
//  2. 认证结果带明确有效期（AttestedUntil）——不再是永不过期的布尔位。
//  3. challenge 长度在无状态层与状态层双重设限，避免超长输入撑大键与历史。
func (k msgServer) AttestDevice(goCtx context.Context, msg *types.MsgAttestDevice) (*types.MsgAttestDeviceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// 签名者身份即设备身份
	deviceAddr, err := resolveSelfAddress(msg.Creator, msg.Address, "device")
	if err != nil {
		return nil, err
	}

	// challenge 边界闸门（状态层，防绕过 ValidateBasic 的路径）
	if msg.Challenge == "" || len(msg.Challenge) > types.MaxAttestChallengeLength {
		return nil, types.ErrInvalidAttestation.Wrapf(
			"challenge length out of range (expect 1..%d bytes)", types.MaxAttestChallengeLength)
	}

	st, err := k.Keeper.GetDevice(ctx, deviceAddr)
	if err != nil {
		return nil, err
	}

	// 先查消费标记再验签——重放请求在最早的位置被拒绝，
	// 不消耗验签算力，也避免「验签通过但消费失败」的中间态。
	if !k.Keeper.ConsumeAttestChallenge(ctx, deviceAddr, msg.Challenge) {
		return nil, types.ErrInvalidAttestation.Wrap("attestation challenge already consumed (replay rejected)")
	}

	// T2 可插拔预言机：attestation 校验交由 DefaultOracle（默认 SoftOracle，
	// 行为与历史一致；生产可 SetOracle(NewTeeOracle(pk)) 切换真实验签）。
	if err := types.DefaultOracle.VerifyDeviceAttestation(ctx, deviceAddr, msg.Challenge, msg.Signature); err != nil {
		return nil, err
	}

	// 验签通过后把结果回写到 phonenode 的验证层级（TierOracle），
	// 让「预言机已背书」这一事实在链上有唯一存放处，而不是 depin 自己另记一份。
	// 仅当该设备同时是 phonenode 移动节点时才回写；回写失败属一致性故障，不得静默吞掉。
	if k.phonenodeKeeper != nil && k.phonenodeKeeper.HasNode(ctx, deviceAddr) {
		if err := k.phonenodeKeeper.MarkOracleVerified(ctx, deviceAddr, msg.Challenge); err != nil {
			return nil, err
		}
	}

	st.Attested = true
	// 认证结果带明确有效期，到期需重新认证。
	st.AttestedUntil = ctx.BlockTime().Unix() + types.AttestationValiditySeconds
	if err := k.Keeper.SetDevice(ctx, st); err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"depin.DeviceAttested",
		sdk.NewAttribute("device", deviceAddr),
		sdk.NewAttribute("attested_until", fmt.Sprintf("%d", st.AttestedUntil)),
	))

	return &types.MsgAttestDeviceResponse{}, nil
}
