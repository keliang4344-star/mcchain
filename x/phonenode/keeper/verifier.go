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
//   - Node is registered (HasNode)
//   - Attestation status is valid
//   - Heartbeat is recent (LastProofBlock within OfflineGraceBlocks of
//     current block height)
//   - Node is a bonded validator with self‑bonded tokens >= VerifierMinStake
//
// Non‑validator / non‑bonded / under‑staked nodes are excluded.
func (k Keeper) GetVerifierNodes(ctx sdk.Context) []string {
	params := k.GetParams(ctx)
	curHeight := ctx.BlockHeight()
	nodes := k.AllNodes(ctx)

	out := make([]string, 0, len(nodes))
	for _, st := range nodes {
		// 1. Attestation must be valid
		att, ok := k.GetAttestation(ctx, st.Address)
		if !ok || att.Status != types.AttestationStatusValid {
			continue
		}

		// 2. Heartbeat must be recent
		if params.OfflineGraceBlocks > 0 &&
			(curHeight-st.LastProofBlock) > params.OfflineGraceBlocks {
			continue
		}

		// 3. Must be a bonded validator with sufficient stake
		valAddr, err := sdk.ValAddressFromBech32(st.Address)
		if err != nil {
			continue
		}
		val := k.stakingKeeper.Validator(ctx, valAddr)
		if val == nil || !val.IsBonded() {
			continue
		}
		if val.GetTokens().LT(VerifierMinStake) {
			continue
		}

		out = append(out, st.Address)
	}
	return out
}
