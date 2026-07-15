package types

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	ErrTaskNotFound      = sdkerrors.Register(ModuleName, 1200, "task not found")
	ErrTaskNotOpen       = sdkerrors.Register(ModuleName, 1201, "task is not open for assignment")
	ErrTaskAlreadyDone   = sdkerrors.Register(ModuleName, 1202, "task already completed")
	ErrDuplicateResult   = sdkerrors.Register(ModuleName, 1203, "result already submitted for this task by this node")
	ErrDisputeExists     = sdkerrors.Register(ModuleName, 1204, "dispute already open for this task")
	ErrInvalidResultHash = sdkerrors.Register(ModuleName, 1205, "invalid result: hash required")
	ErrNotAssigned       = sdkerrors.Register(ModuleName, 1206, "task not assigned to this node")
	ErrArbitratorNotSet  = sdkerrors.Register(ModuleName, 1207, "edgeai arbitrator not configured; set params.arbitrator before resolving disputes")
	ErrInvalidResolution = sdkerrors.Register(ModuleName, 1208, "resolution must be 'honest' or 'cheat'")
	ErrDisputeNotOpen    = sdkerrors.Register(ModuleName, 1209, "dispute is not open for this task")
)
