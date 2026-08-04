package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Message type assertions — struct definitions live in tx.pb.go (proto-generated).
var (
	_ sdk.Msg = &MsgCreatePool{}
	_ sdk.Msg = &MsgAddLiquidity{}
	_ sdk.Msg = &MsgRemoveLiquidity{}
	_ sdk.Msg = &MsgSwapExactIn{}
)

func (m *MsgCreatePool) Route() string { return RouterKey }
func (m *MsgCreatePool) Type() string  { return "create_pool" }
func (m *MsgCreatePool) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}
func (m *MsgCreatePool) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgCreatePool) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator: %s", err)
	}
	if m.DenomA == "" || m.DenomB == "" {
		return ErrInvalidDenom
	}
	return nil
}

func (m *MsgAddLiquidity) Route() string { return RouterKey }
func (m *MsgAddLiquidity) Type() string  { return "add_liquidity" }
func (m *MsgAddLiquidity) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}
func (m *MsgAddLiquidity) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgAddLiquidity) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator: %s", err)
	}
	if m.PoolId == 0 {
		return ErrInvalidPoolID
	}
	return nil
}

func (m *MsgRemoveLiquidity) Route() string { return RouterKey }
func (m *MsgRemoveLiquidity) Type() string  { return "remove_liquidity" }
func (m *MsgRemoveLiquidity) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}
func (m *MsgRemoveLiquidity) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgRemoveLiquidity) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator: %s", err)
	}
	if m.PoolId == 0 {
		return ErrInvalidPoolID
	}
	return nil
}

func (m *MsgSwapExactIn) Route() string { return RouterKey }
func (m *MsgSwapExactIn) Type() string  { return "swap_exact_in" }
func (m *MsgSwapExactIn) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}
func (m *MsgSwapExactIn) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgSwapExactIn) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator: %s", err)
	}
	if m.DenomIn == "" || m.DenomOut == "" {
		return ErrInvalidDenom
	}
	return nil
}

// ---------------------------------------------------------------------------
// 离链高频微结算批处理消息（白皮书 §18）
// ---------------------------------------------------------------------------

var (
	_ sdk.Msg = &MsgSubmitSettlementBatch{}
	_ sdk.Msg = &MsgFinalizeSettlementBatch{}
)

func (m *MsgSubmitSettlementBatch) Route() string { return RouterKey }
func (m *MsgSubmitSettlementBatch) Type() string  { return "submit_settlement_batch" }
func (m *MsgSubmitSettlementBatch) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}
func (m *MsgSubmitSettlementBatch) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgSubmitSettlementBatch) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator: %s", err)
	}
	if m.BatchId == "" || m.MerkleRoot == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "batch_id and merkle_root are required")
	}
	if len(m.Entries) == 0 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "batch must contain at least one entry")
	}
	for _, e := range m.Entries {
		if e == nil {
			return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "nil settlement entry")
		}
		if _, err := sdk.AccAddressFromBech32(e.Recipient); err != nil {
			return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid recipient %s: %s", e.Recipient, err)
		}
		if e.AmountUmc == 0 {
			return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "entry amount must be positive")
		}
	}
	return nil
}

func (m *MsgFinalizeSettlementBatch) Route() string { return RouterKey }
func (m *MsgFinalizeSettlementBatch) Type() string  { return "finalize_settlement_batch" }
func (m *MsgFinalizeSettlementBatch) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Creator)
	return []sdk.AccAddress{addr}
}
func (m *MsgFinalizeSettlementBatch) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}
func (m *MsgFinalizeSettlementBatch) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Creator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator: %s", err)
	}
	if m.BatchId == "" {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "batch_id is required")
	}
	return nil
}
