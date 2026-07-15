package types

import (
	"fmt"

	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	"gopkg.in/yaml.v2"
)

var _ paramtypes.ParamSet = (*Params)(nil)

// Parameter store keys (B2 安全参数)。
var (
	KeyAttestationRequired = []byte("AttestationRequired")
	KeyAttestationValidity = []byte("AttestationValidity")
	KeySybilDeviceBinding  = []byte("SybilDeviceBinding")
	KeyOfflineGraceBlocks  = []byte("OfflineGraceBlocks")
	KeyOfflineSlashBps     = []byte("OfflineSlashBps")
	KeyContribSlashBps     = []byte("ContribSlashBps")
	KeyAttestSlashBps      = []byte("AttestSlashBps")
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
		AttestationRequired: true,
		AttestationValidity: int64(86400 * 30), // 30 天
		SybilDeviceBinding:  true,
		OfflineGraceBlocks:  100, // ~100 区块宽限
		OfflineSlashBps:     500, // 离线 slash 5%
		ContribSlashBps:     1000, // 作弊贡献 slash 10%
		AttestSlashBps:      2000, // 伪造 attestation slash 20%
	}
}

// DefaultSlashCooldownBlocks 是 B2 非验证人细则：节点被 slash 后再认证冷却的区块数。
// 硬编码安全常量（~12h @ 4s 出块），避免改动 proto 参数表；后续如需治理可调可提升为 param。
const DefaultSlashCooldownBlocks int64 = 43200

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
