package keeper

import (
	"context"
	"fmt"
	"math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"mcchain/x/edgeai/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

func (k msgServer) CreateTask(goCtx context.Context, msg *types.MsgCreateTask) (*types.MsgCreateTaskResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	creatorAddr, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err)
	}

	// 需求方付费（escrow）：创建任务即由 creator 向 edgeai 模块账户托管 reward，
	// 结算时由该托管金拨付 submitter。reward=0 视为无奖励任务，跳过托管。
	if msg.Reward > 0 {
		// 上界检查（B7）：uint64 → int64 转换在超过 math.MaxInt64 时会溢出，
		// 必须先校验，避免恶意超大 reward 导致托管金额符号翻转。
		if msg.Reward > uint64(math.MaxInt64) {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "reward %d exceeds int64 range", msg.Reward)
		}
		rewardCoins := sdk.NewCoins(sdk.NewInt64Coin(types.EdgeAIDenom, int64(msg.Reward)))
		if k.bankKeeper.SpendableCoins(ctx, creatorAddr).AmountOf(types.EdgeAIDenom).LT(rewardCoins.AmountOf(types.EdgeAIDenom)) {
			return nil, sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, "creator balance insufficient to escrow reward %d umc", msg.Reward)
		}
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, creatorAddr, types.ModuleName, rewardCoins); err != nil {
			return nil, fmt.Errorf("edgeai: escrow reward failed: %w", err)
		}
		// Enterprise settlement fee (finalized 2026-08): the demand side pays
		// EnterpriseSettlementFeeBps (1.50%) on top of the escrowed reward.
		// The fee is split 40% to nodes (fee_collector) / 60% to the protocol treasury.
		// Charging the demand side keeps the 85/15 payout split for the supply
		// side untouched. A creator without the fee balance is not blocked from
		// posting the task; the fee is skipped and recorded as waived.
		if err := k.chargeEnterpriseSettlementFee(ctx, creatorAddr, msg.Reward); err != nil {
			return nil, err
		}
	}

	id := k.nextTaskID(ctx)
	t := &Task{
		Id:             id,
		Creator:        msg.Creator,
		Description:    msg.Description,
		Reward:         msg.Reward,
		Status:         types.TaskStatusOpen,
		CreatedAt:      ctx.BlockTime().Unix(),
		CreatedAtBlock: ctx.BlockHeight(),
	}
	if err := k.Keeper.SetTask(ctx, t); err != nil {
		return nil, err
	}

	// 可选绑定验证脚本哈希（白皮书行 490-493）：若 msg 携带 ScriptHash，
	// 则建立 task→script 绑定，提交结果时校验脚本是否已注册。
	// 注：MsgCreateTask 当前未含 ScriptHash 字段（proto 未重新生成），
	// 此处保留扩展点，实际绑定通过 RegisterScriptSpec + SetTaskScriptHash 完成。
	// TODO: 待 proto 重新生成后从 msg.ScriptHash 读取。

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.TaskCreated",
			sdk.NewAttribute("task_id", id),
			sdk.NewAttribute("creator", msg.Creator),
		),
	)
	return &types.MsgCreateTaskResponse{}, nil
}

// chargeEnterpriseSettlementFee applies the enterprise settlement fee policy to
// an EdgeAI task escrow. Fee = reward * EnterpriseSettlementFeeBps / 10000.
// Split: EnterpriseFeeNodeRatioBps (40%) routed to the fee collector for node
// operators, the remainder (60%) routed to the protocol treasury, the 6th
// independent address. Nothing is burned and no coin is minted on this path:
// enterprise revenue is redistributed, never destroyed.
func (k msgServer) chargeEnterpriseSettlementFee(ctx sdk.Context, payer sdk.AccAddress, reward uint64) error {
	feeAmt := sdk.NewIntFromUint64(reward).
		MulRaw(int64(tokenomicstypes.EnterpriseSettlementFeeBps)).
		QuoRaw(10000)
	if !feeAmt.IsPositive() {
		return nil
	}
	feeCoins := sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, feeAmt))

	// The fee is best-effort: an underfunded creator still gets the task posted.
	if k.bankKeeper.SpendableCoins(ctx, payer).AmountOf(types.EdgeAIDenom).LT(feeAmt) {
		ctx.EventManager().EmitEvent(sdk.NewEvent(
			"edgeai.EnterpriseFeeWaived",
			sdk.NewAttribute("payer", payer.String()),
			sdk.NewAttribute("amount", feeAmt.String()),
		))
		return nil
	}
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, payer, types.ModuleName, feeCoins); err != nil {
		return fmt.Errorf("edgeai: collect enterprise settlement fee: %w", err)
	}

	// 企业付费分账（2026-08 定稿）：40% 分给节点（fee_collector，随出块奖励分配给
	// 验证人与委托人），60% 进国库（protocol_treasury，储备用于社区与生态建设）。
	// 国库不由任何模块凭空生成，只能由这类真实业务收入转入（零新印铁律）。
	nodeAmt := feeAmt.MulRaw(int64(tokenomicstypes.EnterpriseFeeNodeRatioBps)).QuoRaw(10000)
	treasuryAmt := feeAmt.Sub(nodeAmt)

	if nodeAmt.IsPositive() {
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			ctx, types.ModuleName, authtypes.FeeCollectorName,
			sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, nodeAmt)),
		); err != nil {
			return fmt.Errorf("edgeai: route enterprise fee to nodes: %w", err)
		}
	}
	if treasuryAmt.IsPositive() {
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			ctx, types.ModuleName, tokenomicstypes.ProtocolTreasuryPoolName,
			sdk.NewCoins(sdk.NewCoin(types.EdgeAIDenom, treasuryAmt)),
		); err != nil {
			return fmt.Errorf("edgeai: route enterprise fee to treasury: %w", err)
		}
	}

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"edgeai.EnterpriseSettlementFee",
		sdk.NewAttribute("payer", payer.String()),
		sdk.NewAttribute("fee", feeAmt.String()),
		sdk.NewAttribute("to_nodes", nodeAmt.String()),
		sdk.NewAttribute("treasury", treasuryAmt.String()),
	))
	return nil
}
