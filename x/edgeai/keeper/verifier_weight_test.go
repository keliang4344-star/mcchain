package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// WP-2 回归测试：验证者遴选必须按声誉加权
//
// 白皮书载明「Verifier selection is reputation-weighted」/「按声誉加权抽取」。
// 修复前的实现是 rng.Intn(len(addrs)) 等概率抽样，声誉分只用于扣分与接单
// 限流，从不影响被抽中的概率——白皮书的这条承诺在链上并不成立。
//
// 本文件锁定加权语义，防止日后被静默改回等概率。
// ---------------------------------------------------------------------------

// setRep 便捷写入节点声誉。
func setRep(t *testing.T, k *Keeper, ctx sdk.Context, addr string, score uint32) {
	t.Helper()
	require.NoError(t, k.SetReputation(ctx, &Reputation{NodeAddr: addr, Score: score}))
}

// sampleSelection 在连续区块高度上重复遴选，统计各候选被选中的次数。
// 每个高度对应一份不同的确定性随机种子，等价于对分布做采样。
func sampleSelection(k *Keeper, ctx sdk.Context, rounds int) map[string]int {
	counts := make(map[string]int)
	for i := 0; i < rounds; i++ {
		c := ctx.WithBlockHeight(int64(i + 1))
		counts[k.SelectVerifierNode(c)]++
	}
	return counts
}

// TestWP2VerifierSelectionIsReputationWeighted 锁定核心语义：
// 声誉越高，被抽中做评分者的概率越高。
//
// 构造三名候选，声誉 100 / 50 / 1，期望命中比例趋近 100:50:1。
// 断言取保守区间，只要求严格单调 + 高声誉显著压制低声誉，
// 以免因随机采样波动产生偶发失败。
func TestWP2VerifierSelectionIsReputationWeighted(t *testing.T) {
	nodes := []string{"repHigh", "repMid", "repLow"}
	k, ctx, _ := setupEdgeaiFull(t, nodes)

	setRep(t, k, ctx, "repHigh", 100)
	setRep(t, k, ctx, "repMid", 50)
	setRep(t, k, ctx, "repLow", 1)

	const rounds = 600
	counts := sampleSelection(k, ctx, rounds)

	require.Empty(t, counts[""], "存在合格候选时不应返回空地址")
	require.Equal(t, rounds, counts["repHigh"]+counts["repMid"]+counts["repLow"])

	// 严格单调：高 > 中 > 低。
	require.Greater(t, counts["repHigh"], counts["repMid"],
		"声誉 100 应比声誉 50 更常被抽中，实际 high=%d mid=%d", counts["repHigh"], counts["repMid"])
	require.Greater(t, counts["repMid"], counts["repLow"],
		"声誉 50 应比声誉 1 更常被抽中，实际 mid=%d low=%d", counts["repMid"], counts["repLow"])

	// 权重比 100:1，等概率实现下三者各约 200 次；此断言可判定退化。
	require.Greater(t, counts["repHigh"], counts["repLow"]*5,
		"声誉 100 的命中次数应远高于声誉 1（退化为等概率时该断言必然失败）")
}

// TestWP2ZeroReputationNeverSelected 锁定：声誉被扣至 0 的节点权重归零，
// 只要还存在任何非零声誉候选，它就再也拿不到评分权。
//
// 这是白皮书「反复作弊者自动退出抽检队列」在链上的实际含义——
// 不是靠额外的黑名单，而是权重本身归零。
func TestWP2ZeroReputationNeverSelected(t *testing.T) {
	nodes := []string{"honest", "cheaterZero"}
	k, ctx, _ := setupEdgeaiFull(t, nodes)

	setRep(t, k, ctx, "honest", 80)
	setRep(t, k, ctx, "cheaterZero", 0)

	counts := sampleSelection(k, ctx, 300)

	require.Equal(t, 0, counts["cheaterZero"], "声誉为 0 的节点不得被选为验证者")
	require.Equal(t, 300, counts["honest"])
}

// TestWP2AllZeroReputationFallsBackToUniform 锁定退化分支：
// 极端情形下全部候选声誉都为 0，抽检机制不能因总权重为零而停摆，
// 此时退化为等概率抽样，仍必须选出验证者。
func TestWP2AllZeroReputationFallsBackToUniform(t *testing.T) {
	nodes := []string{"z1", "z2"}
	k, ctx, _ := setupEdgeaiFull(t, nodes)

	setRep(t, k, ctx, "z1", 0)
	setRep(t, k, ctx, "z2", 0)

	counts := sampleSelection(k, ctx, 200)

	require.Empty(t, counts[""], "全员声誉为 0 时仍须选出验证者，不得停摆")
	require.Positive(t, counts["z1"], "退化分支应为等概率，两者都应出现")
	require.Positive(t, counts["z2"], "退化分支应为等概率，两者都应出现")
}

// TestWP2DefaultReputationParticipates 锁定：从未有过声誉记录的新节点
// 按初始声誉 100 参与加权（GetReputation 对缺失键返回默认分），
// 不能因为「查不到记录」被静默排除在抽检之外。
func TestWP2DefaultReputationParticipates(t *testing.T) {
	nodes := []string{"brandNew", "known"}
	k, ctx, _ := setupEdgeaiFull(t, nodes)

	// 只给 known 写记录，brandNew 完全没有声誉键。
	setRep(t, k, ctx, "known", 100)

	counts := sampleSelection(k, ctx, 300)

	require.Positive(t, counts["brandNew"], "无声誉记录的新节点应按默认 100 分参与遴选")
	require.Positive(t, counts["known"])
}

// TestWP2SelectionIsDeterministicWithinBlock 锁定 FORK-3 约束在加权路径上
// 依然成立：同一区块上下文重复调用，结果必须逐次相同，
// 否则各验证人写入不同的 Verification 记录 → AppHash 分叉。
func TestWP2SelectionIsDeterministicWithinBlock(t *testing.T) {
	nodes := []string{"n1", "n2", "n3", "n4"}
	k, ctx, _ := setupEdgeaiFull(t, nodes)

	setRep(t, k, ctx, "n1", 90)
	setRep(t, k, ctx, "n2", 70)
	setRep(t, k, ctx, "n3", 40)
	setRep(t, k, ctx, "n4", 10)

	c := ctx.WithBlockHeight(4242)
	first := k.SelectVerifierNode(c)
	require.NotEmpty(t, first)
	for i := 0; i < 20; i++ {
		require.Equal(t, first, k.SelectVerifierNode(c),
			"同一区块上下文的遴选结果必须确定")
	}
}

// TestWP2NoEligibleVerifierReturnsEmpty 锁定边界：候选集为空时返回空串，
// 调用方（BeginBlock 抽检）据此跳过本轮，不得 panic。
func TestWP2NoEligibleVerifierReturnsEmpty(t *testing.T) {
	k, ctx, _ := setupEdgeaiFull(t, nil)
	require.Equal(t, "", k.SelectVerifierNode(ctx))
}
