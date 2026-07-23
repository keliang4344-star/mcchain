package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// Parameter store keys (B2 安全参数)。
var (
	KeyAttestationRequired  = []byte("AttestationRequired")
	KeyAttestationValidity  = []byte("AttestationValidity")
	KeySybilDeviceBinding   = []byte("SybilDeviceBinding")
	KeyOfflineGraceBlocks   = []byte("OfflineGraceBlocks")
	KeyOfflineSlashBps      = []byte("OfflineSlashBps")
	KeyContribSlashBps      = []byte("ContribSlashBps")
	KeyAttestSlashBps       = []byte("AttestSlashBps")
	KeySlashCooldownBlocks  = []byte("SlashCooldownBlocks")
)

// ParamKeyTable the param key table for launch module
func ParamKeyTable() paramtypes.KeyTable {
	return paramtypes.NewKeyTable().RegisterParamSet(&Params{})
}

// NewParams creates a new Params instance
func NewParams() Params {
	return Params{}
}

// DefaultParams returns a default set of parameters (B2 安全默认值)。
func DefaultParams() Params {
	return Params{
		AttestationRequired:  true,
		AttestationValidity:  int64(86400 * 30), // 30 天
		SybilDeviceBinding:   true,
		OfflineGraceBlocks:   100,   // ~100 区块宽限
		OfflineSlashBps:      500,   // 离线 slash 5%
		ContribSlashBps:      1000,  // 作弊贡献 slash 10%
		AttestSlashBps:       2000,  // 伪造 attestation slash 20%
		SlashCooldownBlocks:  43200, // ~12h @ 4s 出块
	}
}

// SlashCooldownBlocks 默认值已提升为链上 param，默认 43200（~12h @ 4s 出块），可由治理调整。
// 具体值从 keeper.GetParams(ctx).SlashCooldownBlocks 读取。

func validateSlashCooldownBlocks(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v <= 0 {
		return fmt.Errorf("slash_cooldown_blocks must be positive: %d", v)
	}
	return nil
}

// ParamSetPairs get the params.ParamSet
func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs {
	return paramtypes.ParamSetPairs{
		paramtypes.NewParamSetPair(KeyAttestationRequired, &p.AttestationRequired, validateBool),
		paramtypes.NewParamSetPair(KeyAttestationValidity, &p.AttestationValidity, validateAttestationValidity),
		paramtypes.NewParamSetPair(KeySybilDeviceBinding, &p.SybilDeviceBinding, validateBool),
		paramtypes.NewParamSetPair(KeyOfflineGraceBlocks, &p.OfflineGraceBlocks, validateOfflineGraceBlocks),
		paramtypes.NewParamSetPair(KeyOfflineSlashBps, &p.OfflineSlashBps, validateBps),
		paramtypes.NewParamSetPair(KeyContribSlashBps, &p.ContribSlashBps, validateBps),
		paramtypes.NewParamSetPair(KeyAttestSlashBps, &p.AttestSlashBps, validateBps),
		paramtypes.NewParamSetPair(KeySlashCooldownBlocks, &p.SlashCooldownBlocks, validateSlashCooldownBlocks),
	}
}

func validateBool(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateAttestationValidity(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v <= 0 {
		return fmt.Errorf("attestation_validity must be positive: %d", v)
	}
	return nil
}

func validateOfflineGraceBlocks(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v <= 0 {
		return fmt.Errorf("offline_grace_blocks must be positive: %d", v)
	}
	return nil
}

func validateBps(i interface{}) error {
	v, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v > 10000 {
		return fmt.Errorf("slash bps must be <= 10000: %d", v)
	}
	return nil
}

// Validate validates the set of params
func (p Params) Validate() error {
	if p.AttestationValidity <= 0 {
		return fmt.Errorf("attestation_validity must be positive")
	}
	if p.OfflineGraceBlocks <= 0 {
		return fmt.Errorf("offline_grace_blocks must be positive")
	}
	if p.SlashCooldownBlocks <= 0 {
		return fmt.Errorf("slash_cooldown_blocks must be positive")
	}
	if p.OfflineSlashBps > 10000 || p.ContribSlashBps > 10000 || p.AttestSlashBps > 10000 {
		return fmt.Errorf("slash bps must be <= 10000")
	}
	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}
