package phonenode

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"
)

// Keeper 持有手机节点模块的链上状态（内存实现，接口对齐 Cosmos SDK Keeper）。
type Keeper struct {
	mu    sync.RWMutex
	nodes map[string]*NodeState
}

// NewKeeper 构造空 Keeper。
func NewKeeper() *Keeper {
	return &Keeper{nodes: make(map[string]*NodeState)}
}

// RegisterNode 注册一台手机节点。role 应为 RoleLight 或 RoleFull。
func (k *Keeper) RegisterNode(addr, model, osVer, role string) (*NodeState, error) {
	if role != RoleLight && role != RoleFull {
		return nil, errors.New("phonenode: invalid role " + role)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.nodes[addr]; ok {
		return nil, ErrNodeExists
	}
	st := &NodeState{Address: addr, Model: model, OS: osVer, Role: role}
	k.nodes[addr] = st
	return st, nil
}

// GetNode 查询节点状态。
func (k *Keeper) GetNode(addr string) (*NodeState, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	st, ok := k.nodes[addr]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return st, nil
}

// MarkPruned 标记该节点已对本地状态做剪枝（手机省存储的核心机制）。
func (k *Keeper) MarkPruned(addr string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	st, ok := k.nodes[addr]
	if !ok {
		return ErrNodeNotFound
	}
	st.Pruned = true
	return nil
}

// SubmitStateProof 让轻节点提交一条 Merkle 证明，验证其本地状态与链上
// 状态根一致。leaf 为节点关心的一段状态（如余额/贡献记录）的哈希，
// proof 为从 leaf 到 root 的兄弟哈希序列，index 为 leaf 在叶子层的位置。
//
// 返回验证是否通过；同时累计 ProofsOK / ProofsBad 统计。验证失败返回
// ErrBadProof（但节点仍记录失败次数，便于风控/女巫检测）。
func (k *Keeper) SubmitStateProof(addr string, root, leaf []byte, index int, proof [][]byte) (bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	st, ok := k.nodes[addr]
	if !ok {
		return false, ErrNodeNotFound
	}
	ok2 := VerifyMerkleProof(root, leaf, proof, index)
	if ok2 {
		st.ProofsOK++
	} else {
		st.ProofsBad++
	}
	if !ok2 {
		return false, ErrBadProof
	}
	return true, nil
}

// VerifyMerkleProof 验证 Merkle 证明：从 leaf 沿 proof 逐层与兄弟哈希
// 做 sha256 拼接，最终结果与 root 比较。index 的二进制位决定每层的左右顺序。
func VerifyMerkleProof(root, leaf []byte, proof [][]byte, index int) bool {
	node := leaf
	for i, sibling := range proof {
		if (index>>i)&1 == 0 {
			node = HashPair(node, sibling) // 本节点在左
		} else {
			node = HashPair(sibling, node) // 本节点在右
		}
	}
	return equalBytes(node, root)
}

// HashPair 两个哈希拼接后做 sha256，供 Merkle 树构造与证明验证复用。
func HashPair(a, b []byte) []byte {
	h := sha256.New()
	h.Write(a)
	h.Write(b)
	return h.Sum(nil)
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// BuildMerkleRoot 工具：由一组叶子哈希构造 Merkle 根（自底向上两两配对，
// 奇数层复制最后一个）。用于测试与演示，真实链上由 CometBFT/应用状态树提供。
func BuildMerkleRoot(leaves [][]byte) []byte {
	if len(leaves) == 0 {
		empty := sha256.New()
		return empty.Sum(nil)
	}
	level := make([][]byte, len(leaves))
	copy(level, leaves)
	for len(level) > 1 {
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1]) // 复制末位
		}
		next := make([][]byte, 0, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			next = append(next, HashPair(level[i], level[i+1]))
		}
		level = next
	}
	return level[0]
}

// LeafHash 工具：把任意字节序列化后做 sha256（用于构造叶子哈希）。
func LeafHash(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

// LeafIndexBytes 工具：把整数索引编码为固定长度字节（演示用叶子内容）。
func LeafIndexBytes(i int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(i))
	return b
}

// CountNodes 统计已注册节点数。
func (k *Keeper) CountNodes() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.nodes)
}
