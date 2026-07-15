package types

import sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

var (
	ErrPoolNotFound     = sdkerrors.Register(ModuleName, 2, "pool not found")
	ErrInvalidDenom     = sdkerrors.Register(ModuleName, 3, "invalid denom")
	ErrDuplicateDenom   = sdkerrors.Register(ModuleName, 4, "duplicate denoms in pool")
	ErrInvalidFeeRate   = sdkerrors.Register(ModuleName, 5, "invalid fee rate")
	ErrInvalidMaxPools  = sdkerrors.Register(ModuleName, 6, "invalid max pools")
	ErrMaxPoolsReached  = sdkerrors.Register(ModuleName, 7, "max pools reached")
	ErrInsufficientFunds = sdkerrors.Register(ModuleName, 8, "insufficient funds")
	ErrSlippageExceeded = sdkerrors.Register(ModuleName, 9, "slippage exceeded")
	ErrZeroAmount       = sdkerrors.Register(ModuleName, 10, "amount must be positive")
	ErrPoolEmpty        = sdkerrors.Register(ModuleName, 11, "pool is empty")
	ErrInsufficientLiquidity = sdkerrors.Register(ModuleName, 12, "insufficient liquidity")
	ErrInvalidPoolID    = sdkerrors.Register(ModuleName, 13, "invalid pool id")
	ErrSwapSameDenom    = sdkerrors.Register(ModuleName, 14, "cannot swap same denom")
	ErrInvalidTokenPair = sdkerrors.Register(ModuleName, 15, "invalid token pair for pool")
	ErrDenomSortRequired = sdkerrors.Register(ModuleName, 16, "denoms must be sorted alphabetically")
)
