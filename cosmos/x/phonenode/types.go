// Package phonenode 定义 MobileChain 生产链（B 线 / CometBFT）手机节点模块的类型与错误。
//
// 这是 MobileChain 的核心差异化模块：手机作为“轻节点”参与网络——
// 不存全状态，只保存与自己相关的状态 + 用 Merkle 证明向全节点验证状态正确性，
// 并可对已同步的状态做剪枝（pruning）以省存储。本包提供内存实现，
// 接口对齐 Cosmos SDK Keeper，未来可平滑升级为真实链上模块 x/phonenode。
package phonenode

import "errors"

// 节点角色。
const (
	RoleLight = "light" // 轻节点（手机主流角色）：只存相关状态 + 验证证明
	RoleFull  = "full"  // 全节点：存全状态（少数高性能设备/服务器）
)

// 模块级错误。
var (
	ErrNodeExists   = errors.New("phonenode: node already registered")
	ErrNodeNotFound = errors.New("phonenode: node not found")
	ErrBadProof     = errors.New("phonenode: invalid merkle proof")
)

// NodeState 单台手机节点的链上状态。
type NodeState struct {
	Address   string
	Model     string
	OS        string
	Role      string // light / full
	Pruned    bool   // 是否已对本地状态做剪枝
	ProofsOK  int    // 成功验证的 Merkle 证明次数
	ProofsBad int    // 验证失败的证明次数
}
