package keeper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/phonenode/types"
)

// 本文件把 phonenode 模块的移动节点状态从「空 keeper」迁到 Cosmos SDK 模块 KVStore，
// 实现链上持久化（与 x/depin 同构）。编解码用 encoding/json，绕开 collections.Store 版本耦合。
// 手机即节点：注册后可提交 state proof（在线心跳 / 轻量验证），为出块与 DePIN 贡献提供底座。

var (
	NodeKeyPrefix     = []byte("Node:")
	StateProofKeyPrefix = []byte("StateProof:")
)

func nodeKey(addr string) []byte {
	return append(NodeKeyPrefix, []byte(addr)...)
}

func stateProofKey(node string) []byte {
	return append(StateProofKeyPrefix, []byte(node)...)
}

// SetNode 持久化节点状态（upsert）。
//
// SCALE-1：同步维护「按最近心跳高度排序」的节点索引。本函数是节点状态的唯一
// 写入点（注册 / 心跳 / attestation / 验证者状态变更全部经此），索引与主记录
// 同事务更新，不会漂移。有了它，离线检测才能只看真正超时的那一小段区间，
// 而不是每个区块把全量节点读进内存。
func (k Keeper) SetNode(ctx sdk.Context, st *NodeState) error {
	bz, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("phonenode: marshal node state: %w", err)
	}
	store := ctx.KVStore(k.storeKey)

	// 心跳高度变化时先摘除旧索引位，避免同一节点在索引中留下多个残影。
	if old := store.Get(nodeKey(st.Address)); old != nil {
		var prev NodeState
		if json.Unmarshal(old, &prev) == nil && prev.LastProofBlock != st.LastProofBlock {
			store.Delete(types.HeartbeatIndexKey(prev.LastProofBlock, st.Address))
		}
	}

	store.Set(nodeKey(st.Address), bz)
	store.Set(types.HeartbeatIndexKey(st.LastProofBlock, st.Address), []byte{1})
	return nil
}

// GetNode 读取节点状态；不存在返回 ErrNodeNotFound。
func (k Keeper) GetNode(ctx sdk.Context, addr string) (*NodeState, error) {
	bz := ctx.KVStore(k.storeKey).Get(nodeKey(addr))
	if bz == nil {
		return nil, types.ErrNodeNotFound
	}
	var st NodeState
	if err := json.Unmarshal(bz, &st); err != nil {
		return nil, fmt.Errorf("phonenode: unmarshal node state: %w", err)
	}
	return &st, nil
}

// HasNode reports whether a node with the given address is registered.
// It wraps GetNode's not-found error into a boolean, so it is safe to call from
// other modules (e.g. depin's association check in SubmitContribution).
func (k Keeper) HasNode(ctx sdk.Context, addr string) bool {
	if _, err := k.GetNode(ctx, addr); err != nil {
		return false
	}
	return true
}

// RegisterNode 注册一台移动全节点。重复地址报错。
func (k Keeper) RegisterNode(ctx sdk.Context, addr, model, osVer, role string) (*NodeState, error) {
	if _, err := k.GetNode(ctx, addr); err == nil {
		return nil, types.ErrNodeExists
	}
	st := &NodeState{
		Address:        addr,
		Model:          model,
		OS:             osVer,
		Role:           role,
		Registered:     true,
		LastProofBlock: ctx.BlockHeight(), // 入网即起算在线宽限，避免新生节点被误判离线
	}
	if err := k.SetNode(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

// SetStateProof 持久化一条 state proof（每个节点保留最新一条，upsert）。
func (k Keeper) SetStateProof(ctx sdk.Context, p *StateProof) error {
	bz, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("phonenode: marshal state proof: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(stateProofKey(p.Node), bz)
	return nil
}

// GetStateProof 读取节点最新 state proof；不存在返回 (nil, false)。
func (k Keeper) GetStateProof(ctx sdk.Context, node string) (*StateProof, bool) {
	bz := ctx.KVStore(k.storeKey).Get(stateProofKey(node))
	if bz == nil {
		return nil, false
	}
	var p StateProof
	if err := json.Unmarshal(bz, &p); err != nil {
		return nil, false
	}
	return &p, true
}

// SubmitStateProof 提交一条在线状态证明（在线心跳）：先做真实 Merkle 包含性校验，
// 通过后才落盘并更新节点心跳计数 + 记录最新 root。
//
// 返回 (proofCount, error)。校验失败（非法证明）直接拒绝，不落盘、不计心跳。
func (k Keeper) SubmitStateProof(ctx sdk.Context, node, root, leaf, index, proof string) (int, error) {
	st, err := k.GetNode(ctx, node)
	if err != nil {
		return 0, err
	}
	if root == "" || leaf == "" || index == "" || proof == "" {
		return 0, types.ErrInvalidProof
	}

	// === 真实 Merkle 包含性校验（修复“空壳心跳”）===
	// 节点提交 (root, leaf, index, proof)，链上验证 leaf 确实被包含在 root 对应的
	// 状态树中（proof 为兄弟节点哈希链）。校验失败即视为伪造/篡改，拒绝提交。
	if err := k.VerifyStateProof(root, leaf, index, proof); err != nil {
		return 0, err
	}

	p := &StateProof{
		Node:  node,
		Root:  root,
		Leaf:  leaf,
		Index: index,
		Proof: proof,
	}
	if err := k.SetStateProof(ctx, p); err != nil {
		return 0, err
	}
	st.ProofCount++
	st.LastRoot = root
	st.LastProofBlock = ctx.BlockHeight()
	if err := k.SetNode(ctx, st); err != nil {
		return 0, err
	}
	return st.ProofCount, nil
}

// verifyStateProof 对提交的在线状态证明做真实的 Merkle 包含性校验。
//
// 算法（二进制 Merkle 树，hash-chain 包含证明）：
//   - leaf 哈希 = sha256(leaf bytes)
//   - proof 为十六进制拼接的兄弟节点哈希链，每条 32 字节，由低层到高层排列
//   - index 为叶子下标（uint），其各比特位决定每层的左右拼接顺序
//   - 逐层折叠出的根哈希必须等于提交的 root
//
// 链上不保存完整状态树；root 由提交方声明、proof 由提交方提供。本函数验证
// (leaf, index, proof) 能合法重建出该 root —— 即该节点状态确实被声明根包含。
// 若需更强保证（root 必须等于某历史承诺根），可在此追加与外部锚定根的对比。
// 校验通过返回 nil；否则返回 ErrInvalidProof（拒绝伪造心跳）。
func (k Keeper) VerifyStateProof(rootHex, leaf, indexStr, proofHex string) error {
	idx, err := strconv.ParseUint(indexStr, 10, 64)
	if err != nil {
		return types.ErrInvalidProof.Wrapf("index must be a non-negative integer, got %q", indexStr)
	}
	root, err := hex.DecodeString(rootHex)
	if err != nil || len(root) != 32 {
		return types.ErrInvalidProof.Wrap("root must be a 32-byte hex string")
	}
	proofBytes, err := hex.DecodeString(proofHex)
	if err != nil || len(proofBytes)%32 != 0 {
		return types.ErrInvalidProof.Wrap("proof must be a hex string of 32-byte-aligned sibling hashes")
	}

	cur := sha256.Sum256([]byte(leaf))
	i := idx
	for off := 0; off < len(proofBytes); off += 32 {
		sib := proofBytes[off : off+32]
		if i&1 == 0 {
			// 当前节点在左，兄弟在右
			cur = sha256.Sum256(append(cur[:], sib...))
		} else {
			// 当前节点在右，兄弟在左
			cur = sha256.Sum256(append(sib, cur[:]...))
		}
		i >>= 1
	}

	if !bytes.Equal(cur[:], root) {
		return types.ErrInvalidProof.Wrap("merkle proof does not reconstruct the claimed root")
	}
	return nil
}

// CountNodes 返回已注册节点数。
func (k Keeper) CountNodes(ctx sdk.Context) int {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), NodeKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	n := 0
	for ; it.Valid(); it.Next() {
		n++
	}
	return n
}

// CountProofs 返回已提交 state proof 的节点数（每个节点保留最新一条）。
func (k Keeper) CountProofs(ctx sdk.Context) int {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), StateProofKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	n := 0
	for ; it.Valid(); it.Next() {
		n++
	}
	return n
}

// AllNodes 按地址字典序返回全部节点（确定性，便于对账/审计）。
func (k Keeper) AllNodes(ctx sdk.Context) []NodeState {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), NodeKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]NodeState, 0)
	for ; it.Valid(); it.Next() {
		var st NodeState
		if err := json.Unmarshal(it.Value(), &st); err != nil {
			continue
		}
		out = append(out, st)
	}
	return out
}
