package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const TypeMsgCreateTask = "create_task"

// 自由文本/哈希字段必须设长度上限。
// 这些字段会被永久写入 KVStore，无上限时单笔交易即可塞入超大负载
// （受块 gas 限制但远超业务需要），数值上可无限次重复，状态持续膨胀。
const (
	MaxDescriptionLen = 512
	MaxTaskIDLen      = 64
	MaxHashLen        = 128 // hex 最长 64 字符，留余量
	MaxNonceLen       = 128
	MaxReasonLen      = 1024
)

var _ sdk.Msg = &MsgCreateTask{}

func NewMsgCreateTask(creator, description string, reward uint64) *MsgCreateTask {
	return &MsgCreateTask{Creator: creator, Description: description, Reward: reward}
}

func (msg *MsgCreateTask) Route() string { return RouterKey }
func (msg *MsgCreateTask) Type() string  { return TypeMsgCreateTask }
func (msg *MsgCreateTask) GetSigners() []sdk.AccAddress {
	creator, _ := sdk.AccAddressFromBech32(msg.Creator)
	return []sdk.AccAddress{creator}
}
func (msg *MsgCreateTask) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(msg))
}
func (msg *MsgCreateTask) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator (%s)", err)
	}
	if msg.Description == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "description required")
	}
	if len(msg.Description) > MaxDescriptionLen {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "description too long: %d > %d", len(msg.Description), MaxDescriptionLen)
	}
	return nil
}
