package types

// DONTCOVER

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/phonenode module sentinel errors
var (
	ErrNodeExists           = sdkerrors.Register(ModuleName, 1100, "node already registered")
	ErrNodeNotFound         = sdkerrors.Register(ModuleName, 1101, "node not found")
	ErrInvalidProof         = sdkerrors.Register(ModuleName, 1102, "invalid state proof: root/leaf/index/proof required")
	ErrAttestationRequired  = sdkerrors.Register(ModuleName, 1103, "attestation required but node has none")
	ErrNonceReused          = sdkerrors.Register(ModuleName, 1104, "attestation nonce already used")
	ErrDeviceAlreadyBound   = sdkerrors.Register(ModuleName, 1105, "device_id_hash already bound to another node")
	ErrInvalidAttestation   = sdkerrors.Register(ModuleName, 1106, "invalid attestation: root_hash/nonce/device_id_hash required")
	ErrNotBondedValidator   = sdkerrors.Register(ModuleName, 1107, "node is not a bonded validator; only attestation revocation applies")
	ErrSlashCooldown        = sdkerrors.Register(ModuleName, 1108, "node is in slash cooldown; re-attestation blocked until cooldown passes")
	ErrInvalidVerifierStatus = sdkerrors.Register(ModuleName, 1109, "invalid verifier status; must be 'active' or 'suspended'")
	ErrInsufficientStake     = sdkerrors.Register(ModuleName, 1110, "insufficient stake for verifier role")
)
