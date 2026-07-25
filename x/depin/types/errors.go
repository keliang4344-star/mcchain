package types

// DONTCOVER

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/depin module sentinel errors
var (
	ErrDeviceExists       = sdkerrors.Register(ModuleName, 1100, "device already registered")
	ErrDeviceNotFound     = sdkerrors.Register(ModuleName, 1101, "device not found")
	ErrTaskExists         = sdkerrors.Register(ModuleName, 1102, "contribution task id already exists")
	ErrInvalidScore       = sdkerrors.Register(ModuleName, 1103, "score out of range [0,100]")
	ErrUnsupportedType    = sdkerrors.Register(ModuleName, 1104, "unsupported task type")
	ErrDeviceNotAttested  = sdkerrors.Register(ModuleName, 1105, "device not attested")
	ErrInvalidAttestation = sdkerrors.Register(ModuleName, 1106, "invalid attestation: challenge and signature required")
	// ErrPhonenodeNotRegistered is returned when a contribution is submitted by a
	// device that has not first registered as a phonenode. The association key is
	// the node Address == depin device address (SubmitContribution.Creator).
	ErrPhonenodeNotRegistered = sdkerrors.Register(ModuleName, 1107, "creator not registered in phonenode")
	ErrAttestationFailed      = sdkerrors.Register(ModuleName, 1108, "device attestation verification failed")
)
