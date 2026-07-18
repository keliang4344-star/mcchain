package types

import "fmt"

// NodeRole 定义节点角色枚举。
type NodeRole string

const (
	// NodeRoleDePIN 设备贡献节点（默认新注册节点角色）。
	// 参与 DePIN 贡献任务，提交状态证明和 attestation。
	NodeRoleDePIN NodeRole = "depin"

	// NodeRoleValidator 验证人节点。
	// 参与共识出块，需满足质押要求。
	NodeRoleValidator NodeRole = "validator"

	// NodeRoleFullNode 全节点。
	// 同步全量区块数据，不参与出块但可提供 RPC 服务。
	NodeRoleFullNode NodeRole = "fullnode"
)

// DefaultNodeRole 新注册节点的默认角色。
const DefaultNodeRole = NodeRoleDePIN

// ValidRoles 合法角色集合。
var ValidRoles = map[NodeRole]bool{
	NodeRoleDePIN:     true,
	NodeRoleValidator: true,
	NodeRoleFullNode:  true,
}

// WalletCompatRoles 钱包 v1.6 传值兼容映射。
// 钱包传 'validator' 时自动映射为 NodeRoleDePIN（临时兼容）。
var WalletCompatRoles = map[string]NodeRole{
	"validator": NodeRoleDePIN, // 兼容钱包 v1.6 旧传值
}

// NormalizeRole 标准化角色字符串：校验合法性，做兼容映射，返回标准 NodeRole。
func NormalizeRole(roleStr string) (NodeRole, error) {
	if roleStr == "" {
		return DefaultNodeRole, nil
	}

	// 检查兼容映射
	if mapped, ok := WalletCompatRoles[roleStr]; ok {
		return mapped, nil
	}

	role := NodeRole(roleStr)
	if ValidRoles[role] {
		return role, nil
	}

	return "", fmt.Errorf("unknown node role: %q (valid: depin, validator, fullnode)", roleStr)
}
