package keeper

import (
	"crypto/sha256"
	"encoding/hex"
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

// VerifyDeviceAttestation 验证设备身份证明。
// 流程：读取链上已注册节点设备指纹 → SHA256 校验 → 返回结果。
func (k Keeper) VerifyDeviceAttestation(ctx sdk.Context, deviceID, proof, signature string) (bool, string) {
	// 查询 phonenode 模块：设备是否已注册为节点
	if !k.phonenodeKeeper.HasNode(ctx, deviceID) {
		return false, "device not registered as phonenode"
	}

	// 校验 attestation proof：对 deviceID 做 SHA256，与 proof 比对
	hash := sha256.Sum256([]byte(deviceID))
	expectedProof := hex.EncodeToString(hash[:])

	if expectedProof != proof {
		return false, fmt.Sprintf("attestation proof mismatch: expected %s, got %s", expectedProof, proof)
	}

	// 通过 phonenode keeper 检查该节点是否已有有效 attestation
	if !k.phonenodeKeeper.IsAttested(ctx, deviceID) {
		return false, "device attestation not yet complete in phonenode"
	}

	return true, "attestation verified"
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
