package keeper

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// FORK-3 修复：共识路径禁止使用 math/rand 全局随机源。
//
// 背景：x/edgeai 的验证者抽检（BeginBlock Phase 3）此前直接调用包级
// rand.Shuffle / rand.Intn。这在多节点网络中必然导致状态分叉：
//
//  1. Go 1.20 起，math/rand 的全局源在进程启动时以随机值自动播种，
//     每个节点进程的随机序列互不相同；
//  2. rand.Seed 自 Go 1.20 起已废弃，且全局源为进程共享——
//     即便播种，任何其它代码（gRPC 网关 goroutine、依赖库）取用全局源
//     都会改变后续序列，播种与取值之间不存在可复现的因果链；
//  3. ScoreAndVerify 路径压根没有播种。
//
// 结果是不同验证人会选出不同的验证者/任务、写入不同的 Verification 记录，
// AppHash 不一致 → 全网停链。
//
// 修复方式：所有共识路径的随机性一律由区块数据派生的本地 *rand.Rand 提供。
// 种子 = SHA256(chainID ‖ blockHeight ‖ blockTime ‖ headerHash ‖ domain) 的前 8 字节。
//   - 确定性：同一区块内，所有诚实节点得到完全相同的序列；
//   - 不可预测性：headerHash 在区块被提议前不可知，无法提前布局刷取抽检；
//   - 域分离：domain 保证不同用途（选验证者 / 抽任务）互不相关，
//     避免同一区块内多次调用退化为同一序列。
func newDeterministicRand(ctx sdk.Context, domain string) *rand.Rand {
	h := sha256.New()

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(ctx.BlockHeight()))
	_, _ = h.Write(buf[:])

	binary.BigEndian.PutUint64(buf[:], uint64(ctx.BlockTime().UnixNano()))
	_, _ = h.Write(buf[:])

	// HeaderHash 在 BeginBlock 由 baseapp 注入（req.Hash）；
	// 在单元测试等未设置的场景下为空，此时退化为 height+time+domain 派生，
	// 仍然完全确定，不影响共识安全。
	_, _ = h.Write(ctx.HeaderHash())
	_, _ = h.Write([]byte(ctx.ChainID()))
	_, _ = h.Write([]byte(domain))

	sum := h.Sum(nil)
	seed := int64(binary.BigEndian.Uint64(sum[:8]))

	return rand.New(rand.NewSource(seed))
}
