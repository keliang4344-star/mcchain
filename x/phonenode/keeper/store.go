package keeper

import (
	"encoding/json"
	"fmt"

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
func (k Keeper) SetNode(ctx sdk.Context, st *NodeState) error {
	bz, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("phonenode: marshal node state: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(nodeKey(st.Address), bz)
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

// SubmitStateProof 提交一条在线状态证明：校验 → 落盘 → 节点心跳计数 + 记录最新 root。
//
// 返回 (proofCount, error)。真实 Merkle/状态验证为后续项；当前做基础非空校验（占位）。
func (k Keeper) SubmitStateProof(ctx sdk.Context, node, root, leaf, index, proof string) (int, error) {
	st, err := k.GetNode(ctx, node)
	if err != nil {
		return 0, err
	}
	if root == "" || leaf == "" || index == "" || proof == "" {
		return 0, types.ErrInvalidProof
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
