package types

import (
	"encoding/base64"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AttestationOracle 设备 attestation 验证抽象（T2 可插拔预言机框架）。
//
// 链侧只依赖此接口；具体实现可在「软认证（开发）」与「真实验证（生产）」之间切换，
// 无需改动 AttestDevice 消息处理逻辑。真实设备端（Android Key Attestation /
// iOS DeviceCheck 出证 + 链下预言机服务转发签名）由接入方实现，链上只负责验签。
type AttestationOracle interface {
	// VerifyDeviceAttestation 校验设备 attestation 是否通过。
	// challenge / signature 的语义由实现决定：
	//   - SoftOracle：仅校验两者非空（开发/测试）。
	//   - TeeOracle  ：校验 signature 为预言机私钥对 (deviceAddr|challenge) 的签名。
	VerifyDeviceAttestation(ctx sdk.Context, deviceAddr, challenge, signature string) error
}

// DefaultOracle 当前启用的预言机实现。默认 SoftOracle（与历史行为一致）。
// 生产环境调用 SetOracle(NewTeeOracle(pubKey)) 切换为真实验证。
var DefaultOracle AttestationOracle = &SoftOracle{}

// SetOracle 切换预言机实现（生产部署时调用，例如 app 初始化阶段）。
func SetOracle(o AttestationOracle) {
	if o != nil {
		DefaultOracle = o
	}
}

// SoftOracle 开发/测试用软认证：仅要求 challenge + signature 均非空。
// 对应原 msg_server 占位逻辑，保证测试网与本地挖矿流程不变。
type SoftOracle struct{}

func (SoftOracle) VerifyDeviceAttestation(_ sdk.Context, _ string, challenge, signature string) error {
	if challenge == "" || signature == "" {
		return ErrInvalidAttestation
	}
	return nil
}

// TeeOracle 生产用真实验证实现（T2 链侧骨架）。
//
// 设备端经 TEE（如 Android Key Attestation）出证后，由链下预言机服务用其
// secp256k1 私钥对 challenge 签名，signature 随 AttestDevice 上链；此处做链上
// 验签，确保 attestation 由受信任的预言机背书，杜绝自签名伪造。
//
// 注意：预言机公钥（pubKey）在部署时注入，不在链上 genesis 硬编码。
type TeeOracle struct {
	pubKey cryptotypes.PubKey
}

// NewTeeOracle 用预言机账户公钥构造 TeeOracle。
func NewTeeOracle(pubKey cryptotypes.PubKey) *TeeOracle {
	return &TeeOracle{pubKey: pubKey}
}

func (o *TeeOracle) VerifyDeviceAttestation(_ sdk.Context, deviceAddr, challenge, signature string) error {
	if o.pubKey == nil {
		// 未配置预言机公钥时降级为拒绝，避免静默放行。
		return ErrInvalidAttestation
	}
	if signature == "" {
		return ErrInvalidAttestation
	}
	sigBz, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return ErrInvalidAttestation
	}
	// 待签消息：设备地址 + "|" + challenge，由预言机服务按此约定签名。
	msg := []byte(deviceAddr + "|" + challenge)
	if !o.pubKey.VerifySignature(msg, sigBz) {
		return ErrInvalidAttestation
	}
	return nil
}

// NewSecp256k1PubKey 由 33 字节压缩公钥构造 TeeOracle 所需的公钥对象（部署辅助）。
func NewSecp256k1PubKey(bz []byte) cryptotypes.PubKey {
	return &secp256k1.PubKey{Key: bz}
}
