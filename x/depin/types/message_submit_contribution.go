package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgSubmitContribution = "submit_contribution"

var _ sdk.Msg = &MsgSubmitContribution{}

func NewMsgSubmitContribution(creator string, taskId string, taskType string, score string) *MsgSubmitContribution {
	return &MsgSubmitContribution{
		Creator:  creator,
		TaskId:   taskId,
		TaskType: taskType,
		Score:    score,
	}
}

func (msg *MsgSubmitContribution) Route() string {
	return RouterKey
}

func (msg *MsgSubmitContribution) Type() string {
	return TypeMsgSubmitContribution
}

func (msg *MsgSubmitContribution) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgSubmitContribution) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgSubmitContribution) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}
	// task_id 会直接拼成 KVStore 键（Contribution:<taskID>），
	// 必须在此做无状态长度闸门；keeper 侧另有一道同口径防线（消息可绕过 ValidateBasic 的路径）。
	if msg.TaskId == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "task_id must not be empty")
	}
	if len(msg.TaskId) > MaxContributionTaskIDLength {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest,
			"task_id too long: %d > %d", len(msg.TaskId), MaxContributionTaskIDLength)
	}
	return nil
}
