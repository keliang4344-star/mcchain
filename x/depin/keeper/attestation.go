package keeper

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/types"
)

// AttestationResultKeyPrefix 是 attestation 结果存储前缀。
var AttestationResultKeyPrefix = []byte("AttestResult:")

func attestationResultKey(deviceID string) []byte {
	return append(AttestationResultKeyPrefix, []byte(deviceID)...)
}

// VerifyDeviceAttestation 验证设备身份证明（真实 attestation）。
//
// 移除原 SHA256(deviceID) 占位校验（人人可凭哈希伪造，属安全空壳）：
// 改为直接信任 phonenode 模块中该设备的链上 attestation 状态——设备须先在
// phonenode 注册为移动节点，并持有当前有效（status=valid 且未过期）的硬件
// attestation。任何未经验证者一律拒绝，从根源杜绝伪造 attestation。
// DePIN 的贡献拨付（SubmitContribution）亦叠加该 IsAttested 闸口，确保发币
// 仅面向真实 attest 设备。
func (k Keeper) VerifyDeviceAttestation(ctx sdk.Context, deviceID, proof, signature string) (bool, string) {
	// 设备须先在 phonenode 注册为移动节点
	if !k.phonenodeKeeper.HasNode(ctx, deviceID) {
		return false, "device not registered as phonenode"
	}

	// 真实 attestation：信任 phonenode 模块中该节点的链上 attestation 状态
	if !k.phonenodeKeeper.IsAttested(ctx, deviceID) {
		return false, "device attestation not yet complete / expired in phonenode"
	}

	return true, "attestation verified via phonenode"
}

// StoreAttestationResult 存储验证结果到 KVStore。
func (k Keeper) StoreAttestationResult(ctx sdk.Context, deviceID string, result types.AttestationResult) error {
	// 追加到历史记录列表
	history := k.GetAttestationHistory(ctx, deviceID)
	history.Results = append(history.Results, result)

	bz, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("depin: marshal attestation history: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(attestationResultKey(deviceID), bz)
	return nil
}

// GetAttestationHistory 查询设备历史验证记录。
func (k Keeper) GetAttestationHistory(ctx sdk.Context, deviceID string) types.AttestationHistory {
	bz := ctx.KVStore(k.storeKey).Get(attestationResultKey(deviceID))
	if bz == nil {
		return types.AttestationHistory{Results: []types.AttestationResult{}}
	}
	var history types.AttestationHistory
	if err := json.Unmarshal(bz, &history); err != nil {
		return types.AttestationHistory{Results: []types.AttestationResult{}}
	}
	return history
}
