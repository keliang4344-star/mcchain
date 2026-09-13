package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Message type assertions — struct definitions live in tx.pb.go (proto-generated).
var (
	_ sdk.Msg = &MsgLiquidStake{}
	_ sdk.Msg = &MsgLiquidUnstake{}
	_ sdk.Msg = &MsgClaimMatured{}
)

// ----------------------------------------------------------------------------
// MsgLiquidStake
// ----------------------------------------------------------------------------

func (m *MsgLiquidStake) Route() string { return RouterKey }
func (m *MsgLiquidStake) Type() string  { return "liquid_stake" }

func (m *MsgLiquidStake) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Delegator)
	return []sdk.AccAddress{addr}
}

func (m *MsgLiquidStake) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

func (m *MsgLiquidStake) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Delegator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid delegator: %s", err)
	}
	if _, err := sdk.ValAddressFromBech32(m.Validator); err != nil {
		return sdkerrors.Wrapf(ErrInvalidAddress, "invalid validator: %s", err)
	}
	if m.AmountUmc == 0 {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "amount must be positive")
	}
	return nil
}

// ----------------------------------------------------------------------------
// MsgLiquidUnstake
// ----------------------------------------------------------------------------

func (m *MsgLiquidUnstake) Route() string { return RouterKey }
func (m *MsgLiquidUnstake) Type() string  { return "liquid_unstake" }

func (m *MsgLiquidUnstake) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Delegator)
	return []sdk.AccAddress{addr}
}

func (m *MsgLiquidUnstake) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

func (m *MsgLiquidUnstake) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Delegator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid delegator: %s", err)
	}
	// An empty validator is allowed: the module then unbonds from the validator
	// holding the largest bond on behalf of the pool.
	if m.Validator != "" {
		if _, err := sdk.ValAddressFromBech32(m.Validator); err != nil {
			return sdkerrors.Wrapf(ErrInvalidAddress, "invalid validator: %s", err)
		}
	}
	if m.SharesUlmc == 0 {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "shares must be positive")
	}
	return nil
}

// ----------------------------------------------------------------------------
// MsgClaimMatured
// ----------------------------------------------------------------------------

func (m *MsgClaimMatured) Route() string { return RouterKey }
func (m *MsgClaimMatured) Type() string  { return "liquid_claim" }

func (m *MsgClaimMatured) GetSigners() []sdk.AccAddress {
	addr, _ := sdk.AccAddressFromBech32(m.Delegator)
	return []sdk.AccAddress{addr}
}

func (m *MsgClaimMatured) GetSignBytes() []byte {
	return sdk.MustSortJSON(ModuleCdc.MustMarshalJSON(m))
}

func (m *MsgClaimMatured) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Delegator); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid delegator: %s", err)
	}
	return nil
}
