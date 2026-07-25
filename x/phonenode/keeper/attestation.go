package keeper

import (
	"encoding/binary"

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

// SetDeviceOwner 记录 device_id_hash → 节点地址 的反查索引（防女巫设备绑定）。
func (k Keeper) SetDeviceOwner(ctx sdk.Context, deviceIDHash, addr string) {
	ctx.KVStore(k.storeKey).Set(types.DeviceHashKey(deviceIDHash), []byte(addr))
}

// SetSlashCooldown 写入某节点 slash 后再认证的截止高度（B2 非验证人细则）。
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
func (k Keeper) SubmitAttestation(ctx sdk.Context, nodeAddr, rootHash, nonce, deviceIDHash string) error {
	if _, err := k.GetNode(ctx, nodeAddr); err != nil {
		return err
	}
	// B2 非验证人细则：被 slash 后的冷却期内禁止再认证（防作弊后秒重连）。
	if k.InSlashCooldown(ctx, nodeAddr) {
		return types.ErrSlashCooldown
	}
	if rootHash == "" || nonce == "" || deviceIDHash == "" {
		return types.ErrInvalidAttestation
	}

	params := k.GetParams(ctx)

	// nonce 不可重放：已使用过的 nonce 直接拒绝
	if ctx.KVStore(k.storeKey).Has(types.NonceKey(nodeAddr, nonce)) {
		return types.ErrNonceReused
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

	// 重置在线宽限计时：重新 attest 视为刚上线，给予完整 OfflineGraceBlocks 宽限。
	if node, nerr := k.GetNode(ctx, nodeAddr); nerr == nil {
		node.LastProofBlock = ctx.BlockHeight()
		_ = k.SetNode(ctx, node)
	}

	// 标记 nonce 已用，防重放
	ctx.KVStore(k.storeKey).Set(types.NonceKey(nodeAddr, nonce), []byte{1})

	return nil
}
