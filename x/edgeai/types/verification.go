package types

// Verification records a verifier's sampling of a settled task result.
// Persisted in the edgeai module KV store under the "verification:" prefix
// using encoding/json.
type Verification struct {
	TaskId            string `json:"task_id"`
	Verifier          string `json:"verifier"`
	IsHonest          bool   `json:"is_honest"`
	Proof             string `json:"proof"`
	Rewarded          bool   `json:"rewarded"`
	CreatedAt         int64  `json:"created_at"`
	// VerifierResultHash is the result hash the verifier observed after
	// re-running the task off-chain. Real verification compares it against the
	// submitter's ResultHash (see verifyResultHashMatch in the keeper).
	VerifierResultHash string `json:"verifier_result_hash"`
}
