package types

// DONTCOVER

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/tokenomics 模块 sentinel errors。
var (
	// ErrInvalidGenesis 表示创世状态校验失败（R1/R2 约束违反）。
	ErrInvalidGenesis = sdkerrors.Register(ModuleName, 1101, "invalid genesis state")
	// ErrCapExceeded 表示铸造将导致累计 minted_supply 超过总量上限（R1）。
	ErrCapExceeded = sdkerrors.Register(ModuleName, 1102, "mint would exceed total supply cap")
)
