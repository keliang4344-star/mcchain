package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// =============================================================================
// Message types — mirrors proto/mcchain/dex/tx.proto
// =============================================================================

var (
	_ sdk.Msg = &MsgCreatePool{}
	_ sdk.Msg = &MsgAddLiquidity{}
	_ sdk.Msg = &MsgRemoveLiquidity{}
	_ sdk.Msg = &MsgSwapExactIn{}
)

// MsgCreatePool defines the message to create a new liquidity pool.
type MsgCreatePool struct {
	Creator    string `json:"creator"`
	DenomA     string `json:"denom_a"`
	DenomB     string `json:"denom_b"`
	AmountA    string `json:"amount_a"`
	AmountB    string `json:"amount_b"`
	FeeRateBps uint32 `json:"fee_rate_bps"`
	PoolId     uint64 `json:"pool_id"`
}

func (m *MsgCreatePool) Reset()               { *m = MsgCreatePool{} }
func (m *MsgCreatePool) String() string        { return "MsgCreatePool" }
func (m *MsgCreatePool) ProtoMessage()          {}
func (m *MsgCreatePool) Route() string          { return RouterKey }
func (m *MsgCreatePool) Type() string           { return "create_pool" }
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

type MsgCreatePoolResponse struct {
	PoolId uint64 `json:"pool_id"`
}
func (m *MsgCreatePoolResponse) Reset()        { *m = MsgCreatePoolResponse{} }
func (m *MsgCreatePoolResponse) String() string { return "MsgCreatePoolResponse" }
func (m *MsgCreatePoolResponse) ProtoMessage()   {}

// MsgAddLiquidity defines the message to add liquidity to a pool.
type MsgAddLiquidity struct {
	Creator    string `json:"creator"`
	PoolId     uint64 `json:"pool_id"`
	AmountAMax string `json:"amount_a_max"`
	AmountBMax string `json:"amount_b_max"`
	MinLpOut   string `json:"min_lp_out"`
}

func (m *MsgAddLiquidity) Reset()               { *m = MsgAddLiquidity{} }
func (m *MsgAddLiquidity) String() string        { return "MsgAddLiquidity" }
func (m *MsgAddLiquidity) ProtoMessage()          {}
func (m *MsgAddLiquidity) Route() string          { return RouterKey }
func (m *MsgAddLiquidity) Type() string           { return "add_liquidity" }
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

type MsgAddLiquidityResponse struct {
	LpMinted string `json:"lp_minted"`
	ActualA  string `json:"actual_a"`
	ActualB  string `json:"actual_b"`
}
func (m *MsgAddLiquidityResponse) Reset()         { *m = MsgAddLiquidityResponse{} }
func (m *MsgAddLiquidityResponse) String() string  { return "MsgAddLiquidityResponse" }
func (m *MsgAddLiquidityResponse) ProtoMessage()    {}

// MsgRemoveLiquidity defines the message to remove liquidity from a pool.
type MsgRemoveLiquidity struct {
	Creator  string `json:"creator"`
	PoolId   uint64 `json:"pool_id"`
	LpAmount string `json:"lp_amount"`
	MinAOut  string `json:"min_a_out"`
	MinBOut  string `json:"min_b_out"`
}

func (m *MsgRemoveLiquidity) Reset()               { *m = MsgRemoveLiquidity{} }
func (m *MsgRemoveLiquidity) String() string        { return "MsgRemoveLiquidity" }
func (m *MsgRemoveLiquidity) ProtoMessage()          {}
func (m *MsgRemoveLiquidity) Route() string          { return RouterKey }
func (m *MsgRemoveLiquidity) Type() string           { return "remove_liquidity" }
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

type MsgRemoveLiquidityResponse struct {
	AmountA string `json:"amount_a"`
	AmountB string `json:"amount_b"`
}
func (m *MsgRemoveLiquidityResponse) Reset()         { *m = MsgRemoveLiquidityResponse{} }
func (m *MsgRemoveLiquidityResponse) String() string  { return "MsgRemoveLiquidityResponse" }
func (m *MsgRemoveLiquidityResponse) ProtoMessage()    {}

// MsgSwapExactIn defines the message to perform an exact-input swap.
type MsgSwapExactIn struct {
	Creator       string `json:"creator"`
	PoolId        uint64 `json:"pool_id"`
	DenomIn       string `json:"denom_in"`
	AmountIn      string `json:"amount_in"`
	DenomOut      string `json:"denom_out"`
	MinAmountOut  string `json:"min_amount_out"`
}

func (m *MsgSwapExactIn) Reset()               { *m = MsgSwapExactIn{} }
func (m *MsgSwapExactIn) String() string        { return "MsgSwapExactIn" }
func (m *MsgSwapExactIn) ProtoMessage()          {}
func (m *MsgSwapExactIn) Route() string          { return RouterKey }
func (m *MsgSwapExactIn) Type() string           { return "swap_exact_in" }
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

type MsgSwapExactInResponse struct {
	AmountOut string `json:"amount_out"`
}
func (m *MsgSwapExactInResponse) Reset()         { *m = MsgSwapExactInResponse{} }
func (m *MsgSwapExactInResponse) String() string  { return "MsgSwapExactInResponse" }
func (m *MsgSwapExactInResponse) ProtoMessage()  {}
