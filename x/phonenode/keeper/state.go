package keeper

// NodeState 记录单个移动全节点的链上状态（持久化于模块 KVStore）。
// 移动端全节点是 MobileChain 的差异化底座：手机即节点，参与出块与 DePIN 贡献。
type NodeState struct {
	Address        string `json:"address"`
	Model          string `json:"model"`
	OS             string `json:"os"`
	Role           string `json:"role"`           // 节点角色：validator / edge / light
	Registered     bool   `json:"registered"`     // 是否已注册
	ProofCount     int    `json:"proof_count"`    // 已提交的在线状态证明数
	LastRoot       string `json:"last_root"`      // 最近一次 state proof 的 root（在线心跳）
	LastProofBlock int64  `json:"last_proof_block"` // 最近一次 state proof 所处区块高度（离线检测用）
	VerifierStatus string `json:"verifier_status"`  // 验证者状态：active / inactive / jailed
}

// StateProof 单条移动节点提交的在线状态证明（持久化于模块 KVStore，可审计）。
type StateProof struct {
	Node  string `json:"node"`
	Root  string `json:"root"`
	Leaf  string `json:"leaf"`
	Index string `json:"index"`
	Proof string `json:"proof"`
}
