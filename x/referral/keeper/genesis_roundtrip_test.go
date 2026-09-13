package keeper_test

import (
	"testing"

	tmdb "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	typesparams "github.com/cosmos/cosmos-sdk/x/params/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/referral/keeper"
	"mcchain/x/referral/types"
)

// --- 本地 mock（不复用 integration_test.go 的，避免与该文件内的全局状态耦合）---

// exportRoundTripPhonenode 让任何地址都视为已注册/活跃挖矿节点。
type exportRoundTripPhonenode struct{}

func (exportRoundTripPhonenode) HasNode(_ sdk.Context, _ string) bool      { return true }
func (exportRoundTripPhonenode) IsActiveNode(_ sdk.Context, _ string) bool { return true }

// exportRoundTripBank 满足 referral 的 BankKeeper 依赖。
type exportRoundTripBank struct{ balance sdk.Coins }

func (m *exportRoundTripBank) GetBalance(_ sdk.Context, _ sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, m.balance.AmountOf(denom))
}

func (m *exportRoundTripBank) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins {
	return m.balance
}

func (m *exportRoundTripBank) SendCoinsFromModuleToAccount(_ sdk.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}

// newIsolatedReferralStore 构造一套独立的（keeper, ctx, cdc）。
// 多次调用得到互不干扰的两套状态，用于模拟「旧链导出 → 新链创世」。
func newIsolatedReferralStore(t *testing.T) (*keeper.Keeper, sdk.Context, codec.JSONCodec) {
	t.Helper()
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	memStoreKey := storetypes.NewMemoryStoreKey(types.MemStoreKey)
	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)
	stateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, db)
	stateStore.MountStoreWithDB(memStoreKey, storetypes.StoreTypeMemory, nil)
	require.NoError(t, stateStore.LoadLatestVersion())

	registry := codectypes.NewInterfaceRegistry()
	cdc := codec.NewProtoCodec(registry)
	ps := typesparams.NewSubspace(cdc, types.Amino, storeKey, memStoreKey, "ReferralParams")
	bank := &exportRoundTripBank{balance: sdk.NewCoins(sdk.NewInt64Coin(types.BaseDenom, 1_000_000_000_000))}
	k := keeper.NewKeeper(cdc, storeKey, ps, bank, exportRoundTripPhonenode{})
	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, log.NewNopLogger())
	return k, ctx, cdc
}

func roundTripAddr() string {
	return sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
}

// TestGenesisRoundTrip_PreservesRuntimeState 是 导出无损性的回归测试：导出必须无损。
//
// 修复前 referral 的 ExportGenesis 只导出 Params，运行期状态（推荐关系树、
// 待发返佣、日熔断计数器、延后账本、释放节奏）在「导出 → 作为新链创世」的迁移中
// 全部丢失，而生成出来的创世文件语法完全合法、validate-genesis 也会 PASS ——
// 这是最难被发现的一类故障。
//
// 这里走的是模块真实的创世通道：先 JSON 序列化（module.go 用 cdc.MustMarshalJSON），
// 再 JSON 反序列化后 InitGenesis，因此顺带覆盖了新增 StateKv 字段的 JSON 往返。
func TestGenesisRoundTrip_PreservesRuntimeState(t *testing.T) {
	src, srcCtx, cdc := newIsolatedReferralStore(t)
	src.SetParams(srcCtx, types.DefaultParams())

	// ---- 播种运行期状态（全部通过公开接口，模拟真实链上累积）----
	inviter, invitee := roundTripAddr(), roundTripAddr()
	_, err := src.CreateReferral(srcCtx, inviter, invitee, "")
	require.NoError(t, err)

	// 触发一次返佣计提 → 产生 pending reward 状态
	require.NoError(t, src.TrackReward(srcCtx, invitee, sdk.NewInt(1_000_000)))

	// 释放节奏配置（配速基准）
	sched := keeper.DefaultReleaseSchedule()
	sched.StartTimeUnix = 1_700_000_000
	sched.Phase1Days = 123
	src.SetReleaseSchedule(srcCtx, sched)

	// 导出前的状态快照（用于逐项比对）
	srcRefs := src.GetReferralsByInviter(srcCtx, inviter)
	require.Len(t, srcRefs, 1, "前置条件：推荐关系已建立")
	srcPending := src.GetPendingRewards(srcCtx, inviter)
	require.True(t, srcPending.Amount.IsPositive(), "前置条件：待发返佣已计提")

	// ---- 导出 → JSON（模块真实通道）→ 解析回来 ----
	exported := keeper.ExportGenesis(srcCtx, *src)
	require.NotEmpty(t, exported.StateKv,
		"导出必须携带运行期状态快照（修复前这里只有 params）")

	raw := cdc.MustMarshalJSON(exported)
	var decoded types.GenesisState
	cdc.MustUnmarshalJSON(raw, &decoded)
	require.Equal(t, len(exported.StateKv), len(decoded.StateKv),
		"StateKv 必须能通过 genesis JSON 往返（键值字段是 bytes，走 base64 编码）")

	// ---- 导入到一套全新的 store（等价于「旧链导出 → 新链创世」）----
	dst, dstCtx, _ := newIsolatedReferralStore(t)
	keeper.InitGenesis(dstCtx, *dst, decoded)

	// ---- 逐项断言状态存活 ----
	dstRefs := dst.GetReferralsByInviter(dstCtx, inviter)
	require.Len(t, dstRefs, 1, "推荐关系必须跨导出/导入存活")
	require.Equal(t, srcRefs[0].Invitee, dstRefs[0].Invitee)
	require.Equal(t, srcRefs[0].Inviter, dstRefs[0].Inviter)

	dstPending := dst.GetPendingRewards(dstCtx, inviter)
	require.Equal(t, srcPending.String(), dstPending.String(),
		"待发返佣必须跨导出/导入存活（否则用户的钱凭空消失）")

	dstSched := dst.GetReleaseSchedule(dstCtx)
	require.Equal(t, sched.StartTimeUnix, dstSched.StartTimeUnix,
		"释放窗口起点必须存活，否则配速重新从 0 开始、预算节奏被打乱")
	require.Equal(t, sched.Phase1Days, dstSched.Phase1Days)

	require.Equal(t, types.DefaultParams(), dst.GetParams(dstCtx), "参数也必须往返一致")
}

// TestGenesisInit_EmptySnapshotBackwardCompatible
// 保证既有创世文件（没有 state_kv 字段）行为不变：只按 params 初始化。
func TestGenesisInit_EmptySnapshotBackwardCompatible(t *testing.T) {
	dst, dstCtx, _ := newIsolatedReferralStore(t)
	params := types.DefaultParams()
	params.Level1RewardRateBps = 777

	// 模拟旧版创世文件：只有 params，没有 state_kv
	keeper.InitGenesis(dstCtx, *dst, types.GenesisState{Params: params})
	require.Equal(t, uint32(777), dst.GetParams(dstCtx).Level1RewardRateBps)
}

// TestGenesisInit_ParamsFieldWinsOverSnapshot
// 快照里也含 params 子空间的键，因此必须保证「创世文件里的 Params」是权威来源：
// 人工修改过参数的创世文件不得被快照里的旧值覆盖回去。
func TestGenesisInit_ParamsFieldWinsOverSnapshot(t *testing.T) {
	src, srcCtx, _ := newIsolatedReferralStore(t)
	old := types.DefaultParams()
	old.Level1RewardRateBps = 1000
	src.SetParams(srcCtx, old)
	exported := keeper.ExportGenesis(srcCtx, *src)

	dst, dstCtx, _ := newIsolatedReferralStore(t)
	gs := *exported
	gs.Params.Level1RewardRateBps = 4321 // 运维在创世文件上手工改参数

	keeper.InitGenesis(dstCtx, *dst, gs)

	require.Equal(t, uint32(4321), dst.GetParams(dstCtx).Level1RewardRateBps,
		"创世文件里的 Params 必须是权威来源，不能被快照覆盖")
}

// TestGenesisExport_CoversNonParamsKeys 确认快照确实覆盖了 params 子空间之外的
// 运行期键（否则「Params 胜出」就无从谈起）。
func TestGenesisExport_CoversNonParamsKeys(t *testing.T) {
	src, srcCtx, _ := newIsolatedReferralStore(t)
	src.SetParams(srcCtx, types.DefaultParams())
	inviter, invitee := roundTripAddr(), roundTripAddr()
	_, err := src.CreateReferral(srcCtx, inviter, invitee, "")
	require.NoError(t, err)

	exported := keeper.ExportGenesis(srcCtx, *src)
	require.NotEmpty(t, exported.StateKv)
	for _, kv := range exported.StateKv {
		require.NotEmpty(t, kv.Key, "快照键不得为空")
	}
	require.Greater(t, len(exported.StateKv), 1,
		"快照应包含 params 键之外的运行期状态（推荐记录 / 邀请索引 / 计数器等）")
}
