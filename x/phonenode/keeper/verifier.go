package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/phonenode/types"
)

// VerifierMinStake is the minimum stake required for a phononode to qualify
// as an EdgeAI verifier: 30000 MC = 30000000000 umc.
var VerifierMinStake = sdk.NewInt(30_000_000_000)

// GetVerifierNodes returns the bech32 addresses of all phononodes that are
// eligible to act as EdgeAI verifier nodes.  Eligibility criteria:
//   - Node is a bonded validator with self-bonded tokens >= VerifierMinStake
//   - Node is registered in phonenode (HasNode)
//   - Attestation status is valid
//   - Heartbeat is recent (LastProofBlock within OfflineGraceBlocks of
//     current block height)
//
// Non-validator / non-bonded / under-staked nodes are excluded.
//
// SCALE-2（上线前审计发现的致命项）：本函数由 EdgeAI 的 BeginBlock 第三阶段
// 每区块调用。旧实现先 AllNodes() 把**全量注册设备**读进内存，再逐个问 staking
// 「你是验证人吗」。设备规模是以亿计的，验证人是以百计的——拿亿去筛百，
// 每个区块做一次，链必然停摆。
//
// 现改为反向遍历：直接取 staking 的 bonded 验证人集合（受 MaxValidators 共识
// 参数硬性封顶），再对这一小撮地址在 phonenode 做 O(1) 主键查询。
// 复杂度从 O(全网设备) 降为 O(验证人数)，且遍历顺序由 staking 的 power 索引
// 给出，全网确定性一致。
func (k Keeper) GetVerifierNodes(ctx sdk.Context) []string {
	if k.stakingKeeper == nil {
		return []string{}
	}
	params := k.GetParams(ctx)
	curHeight := ctx.BlockHeight()

	vals := k.stakingKeeper.GetBondedValidatorsByPower(ctx)
	out := make([]string, 0, len(vals))
	for _, val := range vals {
		// 1. Must be bonded with sufficient self-stake
		if !val.IsBonded() {
			continue
		}
		if val.GetTokens().LT(VerifierMinStake) {
			continue
		}

		// 2. VALADDR-1：验证人操作地址与账户地址是同一串 20 字节，
		//    phonenode 里以账户地址为主键登记（详见 slash.go 的 nodeValAddress）。
		addr := sdk.AccAddress(val.GetOperator()).String()
		st, err := k.GetNode(ctx, addr)
		if err != nil || st == nil || !st.Registered {
			continue // 该验证人并未作为移动节点注册
		}

		// 3. Attestation must be valid
		att, ok := k.GetAttestation(ctx, addr)
		if !ok || att.Status != types.AttestationStatusValid {
			continue
		}

		// 4. Heartbeat must be recent
		if params.OfflineGraceBlocks > 0 &&
			(curHeight-st.LastProofBlock) > params.OfflineGraceBlocks {
			continue
		}

		out = append(out, addr)
	}
	return out
}

// UpdateVerifierStatus 更新指定节点的验证者状态（active / inactive / jailed）。
func (k Keeper) UpdateVerifierStatus(ctx sdk.Context, nodeID, status string) error {
	st, err := k.GetNode(ctx, nodeID)
	if err != nil {
		return err
	}
	st.VerifierStatus = status
	return k.SetNode(ctx, st)
}
