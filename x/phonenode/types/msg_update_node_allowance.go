package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// MsgUpdateNodeAllowance 由治理（gov 模块账户）调整节点资本津贴配置。
//
// 本结构体由 protoc 生成的 tx.pb.go 提供；此文件仅承载 sdk.Msg 业务逻辑方法。
// authority 必须是治理模块账户地址，MsgServer 会二次校验，保证仅经链上治理提案
// 可变更「津贴是否启用 / 每节点每日发放量」，落实白皮书 §32「津贴是治理参数，
// 不是固定权利」的可治理承诺。

const TypeMsgUpdateNodeAllowance = "update_node_allowance"

var _ sdk.Msg = &MsgUpdateNodeAllowance{}

// NewMsgUpdateNodeAllowance 构造节点资本津贴配置更新消息。
// authority 应传入治理模块账户地址（govtypes.ModuleName 派生）。
func NewMsgUpdateNodeAllowance(authority string, enabled bool, perDay uint64) *MsgUpdateNodeAllowance {
	return &MsgUpdateNodeAllowance{
		Authority: authority,
		Enabled:   enabled,
		PerDay:    perDay,
	}
}

func (msg *MsgUpdateNodeAllowance) Route() string { return RouterKey }
func (msg *MsgUpdateNodeAllowance) Type() string  { return TypeMsgUpdateNodeAllowance }

// GetSigners 返回治理模块账户（与消息执行时的授权主体一致）。
func (msg *MsgUpdateNodeAllowance) GetSigners() []sdk.AccAddress {
	addr, err := sdk.AccAddressFromBech32(msg.Authority)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{addr}
}

func (msg *MsgUpdateNodeAllowance) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgUpdateNodeAllowance) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(msg.Authority); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid authority address (%s)", err)
	}
	// per_day == 0 合法，表示暂停发放；上限不在此校验，由治理提案讨论决定。
	return nil
}
