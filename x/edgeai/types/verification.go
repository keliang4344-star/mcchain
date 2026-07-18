package types

// Verification records a verifier's sampling of a settled task result.
// Persisted in the edgeai module KV store under the "verification:" prefix
// using encoding/json.
type Verification struct {
	TaskId    string `json:"task_id"`
	Verifier  string `json:"verifier"`
	IsHonest  bool   `json:"is_honest"`
	Proof     string `json:"proof"`
	Rewarded  bool   `json:"rewarded"`
	CreatedAt int64  `json:"created_at"`
}
