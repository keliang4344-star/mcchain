package keeper_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/types"
)

// leafHashHex 返回 sha256(leaf) 的十六进制串（32 字节）。
func leafHashHex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// merkleRoot2 构造两叶子 Merkle 树根哈希（hex）并返回两叶各自的兄弟证明（hex）。
// 返回：rootHex, siblingFor0, siblingFor1。
func merkleRoot2(leaf0, leaf1 string) (string, string, string) {
	h0 := sha256.Sum256([]byte(leaf0))
	h1 := sha256.Sum256([]byte(leaf1))
	root := sha256.Sum256(append(h0[:], h1[:]...))
	return hex.EncodeToString(root[:]), hex.EncodeToString(h1[:]), hex.EncodeToString(h0[:])
}

// TestPhonenodeStorePersistence 验证移动节点模块 KVStore 持久化：
// 注册/去重/提交 state proof（在线心跳，含真实 Merkle 校验）/ 计数与遍历。
func TestPhonenodeStorePersistence(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)

	// 1) 注册节点
	st, err := k.RegisterNode(ctx, "node1", "Pixel8", "Android14", "edge")
	require.NoError(t, err)
	require.True(t, st.Registered)
	require.Equal(t, 0, st.ProofCount)
	require.Equal(t, 1, k.CountNodes(ctx))

	// 2) 重复注册报错
	_, err = k.RegisterNode(ctx, "node1", "x", "y", "light")
	require.ErrorIs(t, err, types.ErrNodeExists)

	// 构造一棵两叶 Merkle 树，leaf-1→index 0（兄弟 leaf-2 哈希），leaf-2→index 1
	root, sib0, sib1 := merkleRoot2("leaf-1", "leaf-2")

	// 3) 提交合法 state proof（在线心跳，Merkle 校验通过）
	cnt, err := k.SubmitStateProof(ctx, "node1", root, "leaf-1", "0", sib0)
	require.NoError(t, err)
	require.Equal(t, 1, cnt)

	// 4) 再次提交另一叶子，计数累加，LastRoot 更新
	cnt, err = k.SubmitStateProof(ctx, "node1", root, "leaf-2", "1", sib1)
	require.NoError(t, err)
	require.Equal(t, 2, cnt)

	// 5) 非法 Merkle 证明（用错兄弟哈希）必须被拒
	_, err = k.SubmitStateProof(ctx, "node1", root, "leaf-1", "0", sib1)
	require.ErrorIs(t, err, types.ErrInvalidProof)

	// 6) 缺失字段报错
	_, err = k.SubmitStateProof(ctx, "node1", "", "leaf", "0", "proof")
	require.ErrorIs(t, err, types.ErrInvalidProof)

	// 7) 未注册节点提交报错
	_, err = k.SubmitStateProof(ctx, "ghost", root, "leaf-1", "0", sib0)
	require.ErrorIs(t, err, types.ErrNodeNotFound)

	// 8) 最新 proof 校验
	p, ok := k.GetStateProof(ctx, "node1")
	require.True(t, ok)
	require.Equal(t, root, p.Root)

	// 9) 节点计数与 proof 计数
	require.Equal(t, 1, k.CountNodes(ctx))
	require.Equal(t, 1, k.CountProofs(ctx))

	// 10) 全部节点遍历
	all := k.AllNodes(ctx)
	require.Len(t, all, 1)
	require.Equal(t, 2, all[0].ProofCount)
	require.Equal(t, root, all[0].LastRoot)

	// 11) 不存在节点
	_, err = k.GetNode(ctx, "ghost")
	require.ErrorIs(t, err, types.ErrNodeNotFound)
}

// TestVerifyStateProofMerkle 直接验证 Merkle 包含性校验逻辑（覆盖单/多层树）。
func TestVerifyStateProofMerkle(t *testing.T) {
	k, ctx := keepertest.PhonenodeKeeper(t)
	_ = ctx

	// 两叶树：leaf-1(0) 与 leaf-2(1)
	root2, sib0, sib1 := merkleRoot2("leaf-1", "leaf-2")
	require.NoError(t, k.VerifyStateProof(root2, "leaf-1", "0", sib0))
	require.NoError(t, k.VerifyStateProof(root2, "leaf-2", "1", sib1))
	require.ErrorIs(t, k.VerifyStateProof(root2, "leaf-1", "0", sib1), types.ErrInvalidProof)

	// 篡改 leaf 内容 → 重建根不一致
	require.ErrorIs(t, k.VerifyStateProof(root2, "leaf-X", "0", sib0), types.ErrInvalidProof)

	// root 非 32 字节 → 格式错误
	require.ErrorIs(t, k.VerifyStateProof("abcd", "leaf-1", "0", sib0), types.ErrInvalidProof)

	// index 非整数 → 格式错误
	require.ErrorIs(t, k.VerifyStateProof(root2, "leaf-1", "x", sib0), types.ErrInvalidProof)

	// 四叶树：逐叶验证
	l0, l1, l2, l3 := "a", "b", "c", "d"
	h0 := sha256.Sum256([]byte(l0))
	h1 := sha256.Sum256([]byte(l1))
	h2 := sha256.Sum256([]byte(l2))
	h3 := sha256.Sum256([]byte(l3))
	node01 := sha256.Sum256(append(h0[:], h1[:]...))
	node23 := sha256.Sum256(append(h2[:], h3[:]...))
	root4 := sha256.Sum256(append(node01[:], node23[:]...))
	root4Hex := hex.EncodeToString(root4[:])
	// index 0: 兄弟 = h1，再上层兄弟 = node23
	proof0 := hex.EncodeToString(h1[:]) + hex.EncodeToString(node23[:])
	require.NoError(t, k.VerifyStateProof(root4Hex, l0, "0", proof0))
}
