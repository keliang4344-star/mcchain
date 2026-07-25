package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/phonenode/types"
)

// DetectOffline 在 BeginBlock 调用：遍历全部已注册节点，对持有有效 attestation
// 但超过 OfflineGraceBlocks 个区块未提交 state proof（在线心跳）的节点执行离线 slash。
//
// 仅对已 attest 节点检测；未 attest 节点不触发（其领取已被 depin 闸口拒绝）。
// 离线判据基于区块高度（LastProofBlock），与链上时间无关，避免弱网秒级抖动误 slash。
func (k Keeper) DetectOffline(ctx sdk.Context) {
	params := k.GetParams(ctx)
	if params.OfflineGraceBlocks <= 0 {
		return
	}
	curHeight := ctx.BlockHeight()
	nodes := k.AllNodes(ctx)
	for _, st := range nodes {
		att, ok := k.GetAttestation(ctx, st.Address)
		if !ok || att.Status != types.AttestationStatusValid {
			continue
		}
		// 仅在超过 OfflineGraceBlocks 个区块未提交 state proof（在线心跳）时判离线。
		// 入网/重attest 已将 LastProofBlock 置为当前高度，新生节点享有完整宽限，不再被瞬时误 slash。
		if (curHeight - st.LastProofBlock) > params.OfflineGraceBlocks {
			k.SlashIfBad(ctx, st.Address, "offline", params.OfflineSlashBps)
		}
	}
}
