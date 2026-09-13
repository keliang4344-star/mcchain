package keeper

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strconv"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/phonenode/types"
)

// SetAttestation 持久化某节点的 attestation 状态（upsert）。
func (k Keeper) SetAttestation(ctx sdk.Context, addr string, att *types.Attestation) {
	bz := k.cdc.MustMarshal(att)
	ctx.KVStore(k.storeKey).Set(types.AttestationKey(addr), bz)
}

// GetAttestation 读取某节点 attestation；不存在返回 (nil, false)。
func (k Keeper) GetAttestation(ctx sdk.Context, addr string) (*types.Attestation, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.AttestationKey(addr))
	if bz == nil {
		return nil, false
	}
	var att types.Attestation
	k.cdc.MustUnmarshal(bz, &att)
	return &att, true
}

// IsAttested 返回节点是否持有「当前有效」的 attestation（status=valid 且未过期）。
//
// 语义保持冷启动口径不变：设备自签即可通过，用于「能否参与挖矿」这类不需要
// 高可信度的判定。
func (k Keeper) IsAttested(ctx sdk.Context, addr string) bool {
	att, ok := k.GetAttestation(ctx, addr)
	if !ok {
		return false
	}
	if att.Status != types.AttestationStatusValid {
		return false
	}
	return !att.IsExpired(ctx.BlockTime().Unix())
}

// IsVerifiedAttested 返回节点是否持有「预言机已背书」且当前有效的 attestation。
//
// 这是「按什么权重参与」的判定闸门——只有 TierOracle 的节点才被认为是
// 可独立验证的真实设备。与 IsAttested 的区别在于多了一层外部背书，
// 用于 DePIN 贡献拨付等需要真实设备支撑的经济动作。
//
// 背书有期限。仅当背书时间戳存在且距当前区块时间未超过
// OracleEndorsementValidity 时才继续成立；缺时间戳（旧状态/被清理）一律按未背书处理，
// 避免「一次真机校验、永久享受增强层权重」。
func (k Keeper) IsVerifiedAttested(ctx sdk.Context, addr string) bool {
	if !k.IsAttested(ctx, addr) {
		return false
	}
	if k.GetAttestationTier(ctx, addr) < types.AttestationTierOracle {
		return false
	}
	at, ok := k.GetOracleEndorsementAt(ctx, addr)
	if !ok {
		// 无背书时间戳 = 无法证明背书仍在有效期内，fail-closed。
		return false
	}
	return ctx.BlockTime().Unix()-at <= types.OracleEndorsementValidity
}

// SetAttestationTier 持久化某节点的 attestation 验证层级。
func (k Keeper) SetAttestationTier(ctx sdk.Context, addr string, tier uint8) {
	ctx.KVStore(k.storeKey).Set(types.AttestationTierKey(addr), []byte{tier})
}

// GetAttestationTier 读取某节点的 attestation 验证层级；未登记返回 TierSelf。
func (k Keeper) GetAttestationTier(ctx sdk.Context, addr string) uint8 {
	bz := ctx.KVStore(k.storeKey).Get(types.AttestationTierKey(addr))
	if len(bz) == 0 {
		return types.AttestationTierSelf
	}
	return bz[0]
}

// MarkOracleVerified 由外部验签方（x/depin 的 TeeOracle 路径）在链上验签成功后调用，
// 把节点提升到 TierOracle 并留痕 challenge。
//
// 打破循环信任链的核心：若 phonenode 的 attestation 完全自证，x/depin 的
// VerifyDeviceAttestation 又只是回读 phonenode.IsAttested —— 两边互为对方的
// 「证据」，构成一条循环信任链，预言机白名单虽在但内容为空转。
// 现在预言机的验签结果**回写**到 phonenode 的层级上，信任方向变为单向：
// 设备自签（基础层） → 预言机真机校验（增强层） → 经济权重。
//
// 前置条件：节点必须已持有当前有效的自签 attestation（不能凭外部调用凭空造壳）。
//
//  1. challenge 一次性消费——按 SHA-256 摘要登记消费标记，同一 challenge 只能用一次。
//     否则一对 (challenge, signature) 在被广播后可以无限重放：设备被 slash 进冷却期、
//     或背书过期降级后，重放旧签名即可重新拿回 TierOracle。
//  2. 记录背书区块时间——配合 IsVerifiedAttested 让背书成为有期限的断言，
//     而不是一次校验永久生效。
//
// 幂等性说明：同一次背书重复调用会因 challenge 已消费而失败（fail-closed），
// 调用方（depin）不得把该错误当成「一致性故障」重试。
func (k Keeper) MarkOracleVerified(ctx sdk.Context, addr, challenge string) error {
	if !k.IsAttested(ctx, addr) {
		return types.ErrInvalidAttestation.Wrap("cannot promote attestation tier without a valid self attestation")
	}
	if challenge == "" || len(challenge) > types.MaxOracleChallengeLength {
		return types.ErrInvalidAttestation.Wrap("invalid verified challenge")
	}
	if !isPrintableASCII(challenge) {
		return types.ErrInvalidAttestation.Wrap("verified challenge contains non-printable bytes")
	}

	digest := challengeDigest(addr, challenge)
	usedKey := types.OracleChallengeUsedKey(addr, digest)
	if ctx.KVStore(k.storeKey).Has(usedKey) {
		return types.ErrInvalidAttestation.Wrap("oracle challenge already consumed (replay rejected)")
	}

	k.SetAttestationTier(ctx, addr, types.AttestationTierOracle)
	ctx.KVStore(k.storeKey).Set(types.AttestationChallengeKey(addr), []byte(challenge))
	ctx.KVStore(k.storeKey).Set(types.OracleEndorsementAtKey(addr), encodeInt64(ctx.BlockTime().Unix()))
	// 消费标记必须与背书同事务写入，否则验签成功但标记失败会留下重放窗口。
	ctx.KVStore(k.storeKey).Set(usedKey, []byte{1})

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"phonenode.AttestationTierPromoted",
		sdk.NewAttribute("address", addr),
		sdk.NewAttribute("tier", "oracle"),
		sdk.NewAttribute("endorsement_validity_seconds", strconv.FormatInt(types.OracleEndorsementValidity, 10)),
	))
	return nil
}

// GetOracleEndorsementAt 读取某节点最近一次预言机背书的区块时间戳。
//
// 返回值语义按「键是否存在」判定，而不是「值是否大于 0」：区块时间在
// 创世/测试环境下可能是 0，用值判定会把合法背书误判为未背书。
func (k Keeper) GetOracleEndorsementAt(ctx sdk.Context, addr string) (int64, bool) {
	bz := ctx.KVStore(k.storeKey).Get(types.OracleEndorsementAtKey(addr))
	if len(bz) < 8 {
		return 0, false
	}
	return int64(binary.BigEndian.Uint64(bz)), true
}

// encodeInt64 大端编码 int64（供背书时间戳等定长键值使用）。
func encodeInt64(v int64) []byte {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(v))
	return bz
}

// challengeDigest 计算「节点 + challenge」的域分隔摘要（hex 编码，长度恒定 64）。
//
// 直接以 challenge 原文入键会让键长受外部输入影响（虽然已限长 256，但摘要更规整），
// 同时域分隔避免不同节点的相同 challenge 互相碰撞。
func challengeDigest(addr, challenge string) string {
	sum := sha256.Sum256([]byte(addr + "\x00" + challenge))
	return hex.EncodeToString(sum[:])
}

// SetDeviceOwner 记录 device_id_hash → 节点地址 的反查索引（防女巫设备绑定）。
func (k Keeper) SetDeviceOwner(ctx sdk.Context, deviceIDHash, addr string) {
	ctx.KVStore(k.storeKey).Set(types.DeviceHashKey(deviceIDHash), []byte(addr))
}

// SetSlashCooldown 写入某节点 slash 后再认证的截止高度。
func (k Keeper) SetSlashCooldown(ctx sdk.Context, addr string, untilBlock int64) {
	bz := make([]byte, 8)
	binary.BigEndian.PutUint64(bz, uint64(untilBlock))
	ctx.KVStore(k.storeKey).Set(types.SlashCooldownKey(addr), bz)
}

// InSlashCooldown 返回节点是否仍处于 slash 后冷却期（当前高度 < 截止高度）。
func (k Keeper) InSlashCooldown(ctx sdk.Context, addr string) bool {
	bz := ctx.KVStore(k.storeKey).Get(types.SlashCooldownKey(addr))
	if bz == nil || len(bz) < 8 {
		return false
	}
	until := int64(binary.BigEndian.Uint64(bz))
	return ctx.BlockHeight() < until
}

// GetDeviceOwner 反查 device_id_hash 绑定的节点地址；未绑定返回空串。
func (k Keeper) GetDeviceOwner(ctx sdk.Context, deviceIDHash string) string {
	bz := ctx.KVStore(k.storeKey).Get(types.DeviceHashKey(deviceIDHash))
	if bz == nil {
		return ""
	}
	return string(bz)
}

// SubmitAttestation 登记并提交一条硬件 attestation：
//   - 提交者须为已注册节点
//   - root_hash/nonce/device_id_hash 非空
//   - nonce 不可重放（同一节点不可重复使用同一 nonce）
//   - 若开启设备绑定，device_id_hash 须 1:1 绑定本地址
//   - 计算 expiry = 当前时间 + AttestationValidity，写入 status=valid
//
// 链上只存根哈希 + nonce + device_id_hash + expiry + status；重验证（Play Integrity / Key Attestation）链下完成。
func (k Keeper) SubmitAttestation(ctx sdk.Context, nodeAddr, rootHash, nonce, deviceIDHash, devicePubKey, signature string) error {
	if _, err := k.GetNode(ctx, nodeAddr); err != nil {
		return err
	}
	// 非验证人细则：被 slash 后的冷却期内禁止再认证（防作弊后秒重连）。
	if k.InSlashCooldown(ctx, nodeAddr) {
		return types.ErrSlashCooldown
	}
	if err := validateAttestationInputs(rootHash, nonce, deviceIDHash); err != nil {
		return err
	}

	params := k.GetParams(ctx)

	// nonce 不可重放：已使用过的 nonce 直接拒绝
	if ctx.KVStore(k.storeKey).Has(types.NonceKey(nodeAddr, nonce)) {
		return types.ErrNonceReused
	}

	// === attestation 链上验签（修复“空壳”：原证明=SHA256(deviceID) 人人可伪造）===
	// 设备用其私钥对 "deviceIDHash|nonce" 签名，链上用注册公钥验签；
	// 未过验签一律拒绝，杜绝任何人仅凭哈希伪造 attestation。
	if err := k.verifyDeviceAttestation(ctx, nodeAddr, deviceIDHash, nonce, devicePubKey, signature); err != nil {
		return err
	}

	// 设备绑定防女巫：device_id_hash 1:1 绑定地址
	if params.SybilDeviceBinding {
		if owner := k.GetDeviceOwner(ctx, deviceIDHash); owner != "" && owner != nodeAddr {
			return types.ErrDeviceAlreadyBound
		}
		k.SetDeviceOwner(ctx, deviceIDHash, nodeAddr)
	}

	expiry := ctx.BlockTime().Unix() + params.AttestationValidity
	att := types.NewValidAttestation(rootHash, nonce, deviceIDHash, expiry)
	k.SetAttestation(ctx, nodeAddr, att)

	// 一个认证周期开始时层级回到基础层。预言机背书只对「它实际验过的那一次
	// challenge」有效，不能跨周期继承——否则设备在某个时刻通过真机校验后即可长期
	// 以 TierOracle 身份领取设备级经济权重，中间换成模拟器也无人察觉。
	// 需要 TierOracle 的动作（DePIN 贡献独立校验）在下一次认证时重新走预言机。
	k.SetAttestationTier(ctx, nodeAddr, types.AttestationTierSelf)

	// 重置在线宽限计时：重新 attest 视为刚上线，给予完整 OfflineGraceBlocks 宽限。
	if node, nerr := k.GetNode(ctx, nodeAddr); nerr == nil {
		node.LastProofBlock = ctx.BlockHeight()
		if err := k.SetNode(ctx, node); err != nil {
			// 写入失败不得静默吞掉（序列化异常属确定性故障，记录以便排障）。
			ctx.Logger().Error("phonenode: SetNode failed", "err", err.Error())
		}
	}

	// 标记 nonce 已用，防重放
	ctx.KVStore(k.storeKey).Set(types.NonceKey(nodeAddr, nonce), []byte{1})

	// §4 预言机/认证新鲜度：记录最近一次链上 attestation 的区块时间。
	// ALERTS.md 的 OracleAttestationStale 告警以此判定「认证链路是否停滞」——
	// 若全网长时间没有新的 attestation（设备无法过认证），说明预言机服务
	// 或 APP 端认证链路出了问题。放在写状态成功之后，失败路径不刷新。
	telemetry.SetGauge(float32(ctx.BlockTime().Unix()), "mcchain", "oracle_last_attestation_timestamp")

	return nil
}

// verifyDeviceAttestation 验证设备对 challenge 的签名，并维护设备公钥绑定。
func (k Keeper) verifyDeviceAttestation(ctx sdk.Context, nodeAddr, deviceIDHash, nonce, devicePubKey, signature string) error {
	if devicePubKey == "" || signature == "" {
		return types.ErrInvalidAttestation.Wrap("device pubkey and signature are required for attestation")
	}
	pubBz, err := hex.DecodeString(devicePubKey)
	if err != nil || len(pubBz) != 33 {
		return types.ErrInvalidAttestation.Wrap("invalid device pubkey encoding (expect 33-byte compressed hex)")
	}
	sigBz, err := hex.DecodeString(signature)
	if err != nil {
		return types.ErrInvalidAttestation.Wrap("invalid signature encoding (expect hex)")
	}
	// 兼容 keplr / cosmjs 的 65 字节（含 recovery id）格式：去掉首字节恢复位。
	if len(sigBz) == 65 {
		sigBz = sigBz[1:]
	}
	if len(sigBz) != 64 {
		return types.ErrInvalidAttestation.Wrap("invalid signature length (expect 64-byte compact)")
	}
	pk := &secp256k1.PubKey{Key: pubBz}
	msg := []byte(deviceIDHash + "|" + nonce)
	if !pk.VerifySignature(msg, sigBz) {
		return types.ErrInvalidAttestation.Wrap("attestation signature verification failed")
	}
	// 设备公钥绑定：首次存储；后续必须与已存公钥一致，防止设备密钥随意轮换绕过绑定。
	if stored := k.GetDevicePubKey(ctx, nodeAddr); stored != "" && stored != devicePubKey {
		return types.ErrDeviceAlreadyBound.Wrap("device pubkey changed after registration")
	}
	k.SetDevicePubKey(ctx, nodeAddr, devicePubKey)
	return nil
}

// SetDevicePubKey 持久化某节点绑定的设备公钥（hex 字符串）。
func (k Keeper) SetDevicePubKey(ctx sdk.Context, addr, pubKeyHex string) {
	ctx.KVStore(k.storeKey).Set(types.DevicePubKeyKey(addr), []byte(pubKeyHex))
}

// GetDevicePubKey 读取某节点绑定的设备公钥（hex 字符串）；未绑定返回空串。
func (k Keeper) GetDevicePubKey(ctx sdk.Context, addr string) string {
	bz := ctx.KVStore(k.storeKey).Get(types.DevicePubKeyKey(addr))
	if bz == nil {
		return ""
	}
	return string(bz)
}

// validateAttestationInputs 对 attestation 的三个外部输入做边界与字符集闸门。
//
// 长度与字符集闸门：只判空（`rootHash == "" || nonce == "" || deviceIDHash == ""`）。
// 而 nonce 与 device_id_hash 都会**原样拼进 KVStore 键**（NonceKey / DeviceHashKey），
// 无上限即等于把状态体积的控制权交给任意提交者：一个 1 MiB 的 nonce 就是一次
// 确定性的状态膨胀，gas 也挡不住（写入按字节计费，但状态永久驻留）。
// 现在把三类输入统一收敛到「有界 + 可打印 + 可预期的字符集」。
func validateAttestationInputs(rootHash, nonce, deviceIDHash string) error {
	if rootHash == "" || nonce == "" || deviceIDHash == "" {
		return types.ErrInvalidAttestation
	}
	// root_hash：链上只作留痕，长度设上界即可（base64(32B)=44，余量给 hex）。
	if len(rootHash) > types.MaxRootHashLength {
		return types.ErrInvalidAttestation.Wrap("root_hash too long")
	}
	if !isPrintableASCII(rootHash) {
		return types.ErrInvalidAttestation.Wrap("root_hash contains non-printable bytes")
	}
	// nonce：进入重放索引键，长度与字符集都必须收敛。
	if len(nonce) < types.MinNonceLength || len(nonce) > types.MaxNonceLength {
		return types.ErrInvalidAttestation.Wrap("nonce length out of range")
	}
	if !isPrintableASCII(nonce) {
		return types.ErrInvalidAttestation.Wrap("nonce contains non-printable bytes")
	}
	// device_id_hash：进入女巫绑定反查索引键，统一要求小写 hex（16~32 字节）。
	if !isLowerHex(deviceIDHash) {
		return types.ErrInvalidAttestation.Wrap("device_id_hash must be lowercase hex")
	}
	if len(deviceIDHash) < 32 || len(deviceIDHash) > types.MaxDeviceIDHashLength || len(deviceIDHash)%2 != 0 {
		return types.ErrInvalidAttestation.Wrap("device_id_hash length out of range (expect 32..64 lowercase hex chars)")
	}
	return nil
}

// isPrintableASCII 报告 s 是否全部由可打印 ASCII（0x21..0x7E）组成。
func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7E {
			return false
		}
	}
	return true
}

// isLowerHex 报告 s 是否非空且全部由 [0-9a-f] 组成。
func isLowerHex(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

// DeviceIDCommitment 返回 device_id_hash 的「链上承诺」：SHA-256(chainID || deviceIDHash) 的前 8 字节。
//
// device_id_hash 是设备指纹的哈希，一旦原样进入 tx 事件，事件日志
// （永久、全公开、可被任意索引器抓取）就形成一条稳定的「设备指纹 ↔ 地址」映射。
// 只要链下设备库泄露一次，历史事件即可被直接反查到具体设备。带链 ID 域分隔的
// 短承诺保留了「同一设备是否重复出现」的可观测性，但去掉了直接反查能力，
// 也让同一设备在不同链上的承诺不可交叉关联。
func DeviceIDCommitment(chainID, deviceIDHash string) string {
	sum := sha256.Sum256([]byte(chainID + "\x00" + deviceIDHash))
	return hex.EncodeToString(sum[:8])
}
