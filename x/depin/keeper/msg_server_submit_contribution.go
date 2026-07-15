package keeper

import (
	"context"
	"strconv"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"mcchain/x/depin/types"
)

// SubmitContribution records a verified contribution from a device and, when the
// computed reward is positive, pays it out from the DePIN reward pool (minted at
// genesis). Payout requires the device address to first be registered as a
// phonenode (association key: SubmitContribution.Creator == phonenode node
// Address), enforced below as the "发币闸口".
func (k msgServer) SubmitContribution(goCtx context.Context, msg *types.MsgSubmitContribution) (*types.MsgSubmitContributionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// score 在消息层为 string，需解析为 int
	score, err := strconv.Atoi(msg.Score)
	if err != nil {
		return nil, sdkerrors.Wrapf(types.ErrInvalidScore, "score %q is not an integer", msg.Score)
	}

	// 贡献设备身份 = 提交者（Creator）。必须已注册且通过 attestation（防女巫）。
	deviceAddr := msg.Creator
	if _, err := sdk.AccAddressFromBech32(deviceAddr); err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	st, err := k.Keeper.GetDevice(ctx, deviceAddr)
	if err != nil {
		return nil, err
	}
	if !st.Attested {
		return nil, types.ErrDeviceNotAttested
	}

	// 落盘 + 计奖（不在此处发币）
	reward, err := k.Keeper.SubmitAndReward(ctx, msg.TaskId, deviceAddr, msg.TaskType, score)
	if err != nil {
		return nil, err
	}

	// 仅当奖励 > 0 时，从 DePIN 模块账户（方案 A 池）向设备拨付。
	if reward > 0 {
		// P2/Q5/Q6：发币闸口前置关联校验——设备地址必须先在 phonenode 注册为节点，
		// 否则拒绝发币（不铸、不拨）。关联键 = SubmitContribution.Creator == 节点 Address。
		if !k.Keeper.phonenodeKeeper.HasNode(ctx, deviceAddr) {
			return nil, types.ErrPhonenodeNotRegistered
		}

		// B2 反女巫闸口：设备地址必须持有有效 attestation，否则拒绝拨付（与 depin 自身
		// st.Attested 叠加）。未 attest 节点即使注册也不发币，强制硬件 attestation。
		if !k.Keeper.phonenodeKeeper.IsAttested(ctx, deviceAddr) {
			return nil, types.ErrDeviceNotAttested
		}

		toAddr, err := sdk.AccAddressFromBech32(deviceAddr)
		if err != nil {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid device address (%s)", err)
		}
		// P1/Q4: reward denom is a module param (default "umc"), no longer a const.
		denom := k.Keeper.GetParams(ctx).RewardDenom
		amt := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(int64(reward))))
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, toAddr, amt); err != nil {
			return nil, sdkerrors.Wrapf(err, "failed to pay reward from depin pool")
		}

		// B5/R4：发币事件 —— 移动端 SDK 据此监听「贡献即挖矿」到账通知。
		ctx.EventManager().EmitEvent(
			sdk.NewEvent("depin.RewardPaid",
				sdk.NewAttribute("task_id", msg.TaskId),
				sdk.NewAttribute("device", deviceAddr),
				sdk.NewAttribute("task_type", msg.TaskType),
				sdk.NewAttribute("score", msg.Score),
				sdk.NewAttribute("reward", strconv.FormatUint(uint64(reward), 10)),
				sdk.NewAttribute("denom", denom),
			),
		)

		// O1 业务指标：depin 奖励拨付计数与累计金额（经 app telemetry 在 /metrics 暴露）。
		telemetry.IncrCounter(1, "depin", "reward_paid_count")
		telemetry.IncrCounter(float32(reward), "depin", "reward_paid_amount")
	}

	return &types.MsgSubmitContributionResponse{}, nil
}
