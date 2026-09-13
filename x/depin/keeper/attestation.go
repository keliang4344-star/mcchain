package keeper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/types"
)

// AttestationResultKeyPrefix 是 attestation 结果存储前缀。
var AttestationResultKeyPrefix = []byte("AttestResult:")

// OracleWhitelistKey 存储被授权提交设备 attestation 结果的预言机签名者地址白名单。
var OracleWhitelistKey = []byte("OracleWhitelist")

// MaxAttestationHistory 限制每个设备保留的 attestation 历史记录条数。
// 不设点上限会导致历史列表随每次提交无界增长，放大 KVStore 读写与 gas。
// 仅保留最近 MaxAttestationHistory 条，超出部分丢弃最旧记录。
const MaxAttestationHistory = 64

func attestationResultKey(deviceID string) []byte {
	return append(AttestationResultKeyPrefix, []byte(deviceID)...)
}

// VerifyDeviceAttestation 验证设备身份证明（真实 attestation）。
//
// 移除原 SHA256(deviceID) 占位校验（人人可凭哈希伪造，属安全空壳）：
// 改为对接 phonenode 模块中该设备的链上 attestation 状态——设备须先在
// phonenode 注册为移动节点，并持有当前有效（status=valid 且未过期）的硬件
// attestation。任何未经验证者一律拒绝，从根源杜绝伪造 attestation。
// DePIN 的贡献拨付（SubmitContribution）亦叠加该 IsAttested 闸口，确保发币
// 仅面向真实 attest 设备。
//
// 打破循环信任链：若把入参 proof / signature **整段丢弃**，
// 只回读 phonenode.IsAttested —— 而 phonenode 的 attestation 本身只是设备自签，
// 于是「depin 信任 phonenode、phonenode 只信设备自己」，预言机白名单虽在却
// 不含任何独立信息，构成一条闭环空转。现在：
//
//  1. proof / signature 被真正消费：走 types.DefaultOracle（生产为 TeeOracle）
//     做独立验签，验签通过后**回写** phonenode 的验证层级（TierOracle），
//     使信任方向变为单向：设备自签 → 预言机真机校验 → 经济权重。
//  2. 若调用方未携带独立证明材料，则生产链要求该设备已具备预言机背书层级；
//     开发/测试链放宽到基础层，保持本地挖矿与 CI 流程不变（冷启动门槛不抬高）。
func (k Keeper) VerifyDeviceAttestation(ctx sdk.Context, deviceID, proof, signature string) (bool, string) {
	// 设备须先在 phonenode 注册为移动节点
	if !k.phonenodeKeeper.HasNode(ctx, deviceID) {
		return false, "device not registered as phonenode"
	}

	// 真实 attestation：信任 phonenode 模块中该节点的链上 attestation 状态
	if !k.phonenodeKeeper.IsAttested(ctx, deviceID) {
		return false, "device attestation not yet complete / expired in phonenode"
	}

	// 路径一：带独立证明材料 → 交给可插拔预言机（生产为 TeeOracle）做真实验签。
	if proof != "" || signature != "" {
		if err := types.DefaultOracle.VerifyDeviceAttestation(ctx, deviceID, proof, signature); err != nil {
			return false, "oracle rejected device attestation: " + err.Error()
		}
		if err := k.phonenodeKeeper.MarkOracleVerified(ctx, deviceID, proof); err != nil {
			return false, "failed to record oracle endorsement: " + err.Error()
		}
		return true, "attestation verified via oracle + phonenode"
	}

	// 路径二：无独立证明材料 → 生产链必须已有预言机背书层级。
	if isProductionChainID(ctx.ChainID()) {
		if !k.phonenodeKeeper.IsVerifiedAttested(ctx, deviceID) {
			return false, "device holds self attestation only; production requires oracle-endorsed attestation"
		}
		return true, "attestation verified via phonenode (oracle-endorsed)"
	}
	return true, "attestation verified via phonenode (self tier, non-production chain)"
}

// StoreAttestationResult 存储验证结果到 KVStore。
func (k Keeper) StoreAttestationResult(ctx sdk.Context, deviceID string, result types.AttestationResult) error {
	// 追加到历史记录列表
	history := k.GetAttestationHistory(ctx, deviceID)
	history.Results = append(history.Results, result)

	// 保留最近 MaxAttestationHistory 条，超出部分丢弃最旧记录（防无界膨胀 / 整表读写放大）
	if len(history.Results) > MaxAttestationHistory {
		history.Results = history.Results[len(history.Results)-MaxAttestationHistory:]
	}

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

// GetOracleWhitelist 返回被授权的预言机签名者地址列表。
func (k Keeper) GetOracleWhitelist(ctx sdk.Context) []string {
	bz := ctx.KVStore(k.storeKey).Get(OracleWhitelistKey)
	if bz == nil {
		return nil
	}
	var list []string
	if err := json.Unmarshal(bz, &list); err != nil {
		return nil
	}
	return list
}

// SetOracleWhitelist 持久化被授权的预言机签名者地址列表。
func (k Keeper) SetOracleWhitelist(ctx sdk.Context, addrs []string) {
	bz, err := json.Marshal(addrs)
	if err != nil {
		panic(fmt.Sprintf("depin: marshal oracle whitelist: %v", err))
	}
	ctx.KVStore(k.storeKey).Set(OracleWhitelistKey, bz)
}

// oracleInWhitelist 报告 addr 是否在被授权列表中。
func oracleInWhitelist(addr string, list []string) bool {
	for _, a := range list {
		if a == addr {
			return true
		}
	}
	return false
}

// isProductionChainID 报告当前是否生产部署。
// test/dev/local/sim 链豁免预言机白名单闸门（兼容 SoftOracle 本地开发与 CI）。
func isProductionChainID(chainID string) bool {
	return !strings.Contains(chainID, "test") &&
		!strings.Contains(chainID, "dev") &&
		!strings.Contains(chainID, "local") &&
		!strings.Contains(chainID, "sim")
}

// IsDeviceAttestationValid 报告设备的 depin attestation 当前是否仍然有效。
//
// 把「曾经通过」（Attested）与「现在仍有效」（AttestedUntil）分开。
// 未记录有效期的旧状态（AttestedUntil == 0）一律按失效处理，fail-closed——
// 宁可让老设备重新认证一次，也不能让一条永不过期的布尔位长期兑现奖励。
func IsDeviceAttestationValid(st *DeviceState, nowUnix int64) bool {
	if st == nil || !st.Attested {
		return false
	}
	if st.AttestedUntil <= 0 {
		return false
	}
	return nowUnix < st.AttestedUntil
}

// ConsumeAttestChallenge 登记一次设备认证 challenge 的消费标记（重放防护）。
//
// 返回 true 表示消费成功（首次使用），false 表示该 challenge 已被使用过。
// 键 = "AttChalUsed:" + addr + "/" + hex(SHA256(addr‖challenge))，长度恒定。
func (k Keeper) ConsumeAttestChallenge(ctx sdk.Context, addr, challenge string) bool {
	sum := sha256.Sum256([]byte(addr + "\x00" + challenge))
	key := types.AttestChallengeUsedKey(addr, hex.EncodeToString(sum[:]))
	store := ctx.KVStore(k.storeKey)
	if store.Has(key) {
		return false
	}
	store.Set(key, []byte{1})
	return true
}
