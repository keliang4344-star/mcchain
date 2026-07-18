package keeper

import (
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"mcchain/x/edgeai/types"
)

// verification key prefix for KV store
var verificationKeyPrefix = []byte("verification:")

func verificationKey(taskID, verifier string) []byte {
	return append(verificationKeyPrefix, []byte(taskID+"/"+verifier)...)
}

// SetVerification persists a verification record (JSON encoded).
func (k Keeper) SetVerification(ctx sdk.Context, v *types.Verification) error {
	bz, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("edgeai: marshal verification: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(verificationKey(v.TaskId, v.Verifier), bz)
	return nil
}

// GetVerification retrieves a verification record by (taskID, verifier).
func (k Keeper) GetVerification(ctx sdk.Context, taskID, verifier string) (*types.Verification, error) {
	bz := ctx.KVStore(k.storeKey).Get(verificationKey(taskID, verifier))
	if bz == nil {
		return nil, nil
	}
	var v types.Verification
	if err := json.Unmarshal(bz, &v); err != nil {
		return nil, fmt.Errorf("edgeai: unmarshal verification: %w", err)
	}
	return &v, nil
}

// HasVerification checks whether a verification record exists for the given
// (taskID, verifier) pair.
func (k Keeper) HasVerification(ctx sdk.Context, taskID, verifier string) bool {
	return ctx.KVStore(k.storeKey).Has(verificationKey(taskID, verifier))
}

// AllVerifications returns all stored verification records.
func (k Keeper) AllVerifications(ctx sdk.Context) []*types.Verification {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), verificationKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]*types.Verification, 0)
	for ; it.Valid(); it.Next() {
		var v types.Verification
		if err := json.Unmarshal(it.Value(), &v); err != nil {
			continue
		}
		out = append(out, &v)
	}
	return out
}

// SelectVerifierNode picks a random verifier from the set of eligible nodes
// returned by the phononode keeper.  Returns empty string if no eligible
// verifier exists.
func (k Keeper) SelectVerifierNode(ctx sdk.Context) string {
	addrs := k.phonenodeKeeper.GetVerifierNodes(ctx)
	if len(addrs) == 0 {
		return ""
	}
	idx := rand.Intn(len(addrs))
	return addrs[idx]
}

// SampleTask selects a random done-status task that has NOT yet been verified
// by the given verifier.  Returns nil if no eligible task is found.
func (k Keeper) SampleTask(ctx sdk.Context, verifierAddr string) *Task {
	taskIDs := k.AllTaskIDs(ctx)
	candidates := make([]*Task, 0, len(taskIDs))
	for _, tid := range taskIDs {
		task, err := k.GetTask(ctx, tid)
		if err != nil || task == nil {
			continue
		}
		if task.Status != types.TaskStatusDone {
			continue
		}
		// skip tasks this verifier has already sampled
		if k.HasVerification(ctx, tid, verifierAddr) {
			continue
		}
		candidates = append(candidates, task)
	}
	if len(candidates) == 0 {
		return nil
	}
	idx := rand.Intn(len(candidates))
	return candidates[idx]
}

// AssignVerification creates a new verification assignment record with
// status "assigned".
func (k Keeper) AssignVerification(ctx sdk.Context, taskID, verifierAddr string) (*types.Verification, error) {
	v := &types.Verification{
		TaskId:    taskID,
		Verifier:  verifierAddr,
		IsHonest:  false,
		Proof:     "",
		Rewarded:  false,
		CreatedAt: ctx.BlockTime().Unix(),
	}
	if err := k.SetVerification(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// SubmitVerification processes a verification submission from a verifier node.
//   - isHonest == true  → mark verified, pay reward from module account.
//   - isHonest == false → auto-create a Dispute (calling existing dispute logic).
func (k Keeper) SubmitVerification(ctx sdk.Context, taskID, verifierAddr string, isHonest bool, proof string) error {
	// Update the verification record
	v, err := k.GetVerification(ctx, taskID, verifierAddr)
	if err != nil || v == nil {
		return fmt.Errorf("edgeai: verification not found for task %s verifier %s", taskID, verifierAddr)
	}
	v.IsHonest = isHonest
	v.Proof = proof
	_ = k.SetVerification(ctx, v)

	if !isHonest {
		// Verifier claims the result is dishonest → auto-create dispute.
		// Re-use the existing OpenDispute pathway by constructing the same
		// Dispute record that msg_server_open_dispute.go creates.
		existing, _ := k.GetDispute(ctx, taskID)
		if existing != nil {
			// dispute already exists, skip duplicate
			return nil
		}

		task, err := k.GetTask(ctx, taskID)
		if err != nil || task == nil {
			return fmt.Errorf("edgeai: task %s not found for verification dispute", taskID)
		}

		// Find the submitter of the disputed result
		result, _ := k.GetResultByTask(ctx, taskID)
		submitter := ""
		if result != nil {
			submitter = result.Submitter
		}

		d := &Dispute{
			TaskId:        taskID,
			Challenger:    verifierAddr,
			Submitter:     submitter,
			Reason:        fmt.Sprintf("verifier sampling flagged cheating: %s", proof),
			Status:        "open",
			Resolution:    "none",
			OpenedAt:      ctx.BlockTime().Unix(),
			OpenedAtBlock: ctx.BlockHeight(),
		}
		if err := k.SetDispute(ctx, d); err != nil {
			return err
		}
		task.Status = types.TaskStatusDisputed
		_ = k.SetTask(ctx, task)

		ctx.EventManager().EmitEvent(
			sdk.NewEvent("edgeai.VerifierDispute",
				sdk.NewAttribute("task_id", taskID),
				sdk.NewAttribute("verifier", verifierAddr),
				sdk.NewAttribute("proof", proof),
			),
		)
		telemetry.IncrCounter(1, "edgeai", "verifier_dispute_count")
		return nil
	}

	// Honest verification → pay reward
	if v.Rewarded {
		return nil // already rewarded
	}

	reward := types.VerifierRewardPerSample
	addr, addrErr := sdk.AccAddressFromBech32(verifierAddr)
	if addrErr != nil {
		return fmt.Errorf("edgeai: invalid verifier address %s: %w", verifierAddr, addrErr)
	}
	if sendErr := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, addr,
		sdk.NewCoins(sdk.NewInt64Coin(types.EdgeAIDenom, int64(reward))),
	); sendErr != nil {
		k.Logger(ctx).Error("edgeai: verifier reward failed",
			"task_id", taskID, "verifier", verifierAddr, "err", sendErr.Error())
		return sendErr
	}

	v.Rewarded = true
	_ = k.SetVerification(ctx, v)

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("edgeai.VerifierRewarded",
			sdk.NewAttribute("task_id", taskID),
			sdk.NewAttribute("verifier", verifierAddr),
			sdk.NewAttribute("reward", fmt.Sprintf("%d", reward)),
		),
	)
	telemetry.IncrCounter(1, "edgeai", "verifier_reward_count")
	telemetry.IncrCounter(float32(reward), "edgeai", "verifier_reward_amount")
	return nil
}

// SampleAndVerify is called during BeginBlock (Phase 3) to randomly sample
// one settled (done) task and assign it to a verifier for inspection.
//
// Workflow:
//  1. Select a random eligible verifier node (staked, attested, online).
//  2. Select a random done task not yet verified by this verifier.
//  3. Assign a verification record and immediately "submit" the result.
//     In this simplified on-chain path we auto-pass as honest (true AI
//     re-execution requires off-chain infrastructure).
//
// Sampling is capped at MaxVerificationsPerBlock per block.
func (k Keeper) SampleAndVerify(ctx sdk.Context) {
	// Seed randomness with block time + height for deterministic-but-unpredictable
	// sampling within a block.  rand is not cryptographically secure but is
	// sufficient for randomised sampling that cannot be gamed in advance.
	rand.Seed(ctx.BlockTime().UnixNano() + ctx.BlockHeight())

	verifierAddr := k.SelectVerifierNode(ctx)
	if verifierAddr == "" {
		return
	}

	task := k.SampleTask(ctx, verifierAddr)
	if task == nil {
		return
	}

	if _, err := k.AssignVerification(ctx, task.Id, verifierAddr); err != nil {
		k.Logger(ctx).Error("edgeai: assign verification failed",
			"task_id", task.Id, "verifier", verifierAddr, "err", err.Error())
		return
	}

	// Auto-submit as honest: in this simplified on-chain path we cannot
	// actually re-execute the AI computation, so we trust the result.
	if err := k.SubmitVerification(ctx, task.Id, verifierAddr, true,
		"auto-pass: on-chain sampling"); err != nil {
		k.Logger(ctx).Error("edgeai: submit verification failed",
			"task_id", task.Id, "verifier", verifierAddr, "err", err.Error())
	}
}
