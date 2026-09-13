package types

import (
	"math"
	"sort"

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

// requirePositiveAmount parses a decimal amount string and requires it to be a
// well-formed positive integer. Amounts travel over the wire as strings because
// they are 256-bit; an unparseable or non-positive value must be rejected in
// ValidateBasic so it never reaches the keeper.
func requirePositiveAmount(field, raw string) error {
	v, ok := sdk.NewIntFromString(raw)
	if !ok {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "%s: %q is not a valid integer", field, raw)
	}
	if !v.IsPositive() {
		return sdkerrors.Wrapf(ErrZeroAmount, "%s must be positive, got %s", field, v.String())
	}
	return nil
}

// requireNonNegativeAmount allows an empty string (treated as zero by the msg
// server) but rejects a malformed or negative bound.
func requireNonNegativeAmount(field, raw string) error {
	if raw == "" {
		return nil
	}
	v, ok := sdk.NewIntFromString(raw)
	if !ok {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "%s: %q is not a valid integer", field, raw)
	}
	if v.IsNegative() {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "%s must not be negative, got %s", field, v.String())
	}
	return nil
}

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
	if err := sdk.ValidateDenom(m.DenomA); err != nil {
		return sdkerrors.Wrapf(ErrInvalidDenom, "denom_a: %s", err)
	}
	if err := sdk.ValidateDenom(m.DenomB); err != nil {
		return sdkerrors.Wrapf(ErrInvalidDenom, "denom_b: %s", err)
	}
	if m.DenomA == m.DenomB {
		return ErrDuplicateDenom
	}
	// The AMM stores reserves positionally and relies on the pair being sorted;
	// reject an unsorted pair up front instead of failing deep in the keeper.
	if !sort.StringsAreSorted([]string{m.DenomA, m.DenomB}) {
		return ErrDenomSortRequired
	}
	// A fee rate above MaxFeeRateBps wraps `MaxFeeRateBps - feeRateBps` on a
	// uint32 and corrupts AMM pricing; MaxPoolFeeRateBps additionally stops a
	// pool owner from setting a confiscatory rate. 0 means "use the module
	// default" and is accepted.
	if m.FeeRateBps > MaxPoolFeeRateBps {
		return sdkerrors.Wrapf(ErrInvalidFeeRate,
			"fee_rate_bps must be <= %d (0 = module default), got %d", MaxPoolFeeRateBps, m.FeeRateBps)
	}
	if err := requirePositiveAmount("amount_a", m.AmountA); err != nil {
		return err
	}
	if err := requirePositiveAmount("amount_b", m.AmountB); err != nil {
		return err
	}
	if m.PoolId > MaxPoolID {
		return sdkerrors.Wrapf(ErrInvalidPoolID, "pool_id must be <= %d, got %d", MaxPoolID, m.PoolId)
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
	if err := requirePositiveAmount("amount_a_max", m.AmountAMax); err != nil {
		return err
	}
	if err := requirePositiveAmount("amount_b_max", m.AmountBMax); err != nil {
		return err
	}
	return requireNonNegativeAmount("min_lp_out", m.MinLpOut)
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
	if err := requirePositiveAmount("lp_amount", m.LpAmount); err != nil {
		return err
	}
	if err := requireNonNegativeAmount("min_a_out", m.MinAOut); err != nil {
		return err
	}
	return requireNonNegativeAmount("min_b_out", m.MinBOut)
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
	if m.PoolId == 0 {
		return ErrInvalidPoolID
	}
	if m.DenomIn == "" || m.DenomOut == "" {
		return ErrInvalidDenom
	}
	if err := sdk.ValidateDenom(m.DenomIn); err != nil {
		return sdkerrors.Wrapf(ErrInvalidDenom, "denom_in: %s", err)
	}
	if err := sdk.ValidateDenom(m.DenomOut); err != nil {
		return sdkerrors.Wrapf(ErrInvalidDenom, "denom_out: %s", err)
	}
	if m.DenomIn == m.DenomOut {
		return ErrSwapSameDenom
	}
	if err := requirePositiveAmount("amount_in", m.AmountIn); err != nil {
		return err
	}
	return requireNonNegativeAmount("min_amount_out", m.MinAmountOut)
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
	if len(m.Entries) > MaxSettlementEntries {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest,
			"batch may contain at most %d entries, got %d", MaxSettlementEntries, len(m.Entries))
	}
	var total uint64
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
		// The batch total is a uint64: without this guard a crafted batch could
		// wrap it around and report a total far below what is actually paid out.
		if total > math.MaxUint64-e.AmountUmc {
			return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "batch total overflows uint64")
		}
		total += e.AmountUmc
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
