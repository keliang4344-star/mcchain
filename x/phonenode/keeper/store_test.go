package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/x/phonenode/types"
)

// TestPhonenodeStorePersistence 验证移动节点模块 KVStore 持久化：
// 注册/去重/提交 state proof（在线心跳）/ 基础校验/计数与遍历。
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

	// 3) 提交 state proof（在线心跳）
	cnt, err := k.SubmitStateProof(ctx, "node1", "root-hash-1", "leaf-1", "0", "proof-bytes")
	require.NoError(t, err)
	require.Equal(t, 1, cnt)

	// 4) 再次提交，计数累加，LastRoot 更新
	cnt, err = k.SubmitStateProof(ctx, "node1", "root-hash-2", "leaf-2", "1", "proof-bytes-2")
	require.NoError(t, err)
	require.Equal(t, 2, cnt)

	// 5) 缺失字段报错
	_, err = k.SubmitStateProof(ctx, "node1", "", "leaf", "0", "proof")
	require.ErrorIs(t, err, types.ErrInvalidProof)

	// 6) 未注册节点提交报错
	_, err = k.SubmitStateProof(ctx, "ghost", "r", "l", "0", "p")
	require.ErrorIs(t, err, types.ErrNodeNotFound)

	// 7) 最新 proof 校验
	p, ok := k.GetStateProof(ctx, "node1")
	require.True(t, ok)
	require.Equal(t, "root-hash-2", p.Root)

	// 8) 节点计数与 proof 计数
	require.Equal(t, 1, k.CountNodes(ctx))
	require.Equal(t, 1, k.CountProofs(ctx))

	// 9) 全部节点遍历
	all := k.AllNodes(ctx)
	require.Len(t, all, 1)
	require.Equal(t, 2, all[0].ProofCount)
	require.Equal(t, "root-hash-2", all[0].LastRoot)

	// 10) 不存在节点
	_, err = k.GetNode(ctx, "ghost")
	require.ErrorIs(t, err, types.ErrNodeNotFound)
}
