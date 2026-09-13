// Package kvgenesis 提供模块 KVStore 的通用全量快照 / 回灌能力，用于让
// 各模块的 ExportGenesis / InitGenesis 真正满足「导出 → 导入」闭环。
//
// 背景：
//
//	Cosmos SDK 的 `export` 子命令按模块收集创世 JSON，标准约定是
//	**导出必须无损**——否则「导出旧链状态 → 作为新链创世」这条迁移路径
//	会在无人察觉的情况下丢掉全部运行期状态。本仓库中 referral / depin /
//	edgeai / phonenode 等模块的 GenesisState 只声明了 Params 一个字段，
//	导出后导入等于把推荐关系树、待发返佣、延后账本、设备状态、任务托管
//	全部清零，而生成出来的创世文件在语法上完全合法，`validate-genesis`
//	也会 PASS —— 这是最难被发现的一类故障。
//
// 为什么不逐实体定义 proto 字段：
//
//	这些模块的状态是「多索引 + 计数器 + 游标 + 自建 JSON 结构」的混合体
//	（例如 referral 有 value/count/inviter/invitee 四套索引、日熔断计数器、
//	清游标、延后账本；depin 有设备、贡献、防线计数器、释放金库）。
//	逐实体建模会把「新增一个索引就忘记同步 proto」变成常态，而这正是
//	本次缺陷的成因。改为**原始键值快照**后，完整性由构造保证：
//	只要键在模块 store 里，就一定被导出、被回灌，新增索引无需改任何代码。
//
// 确定性：
//
//	KVStore 迭代顺序由键的字典序唯一确定，因此 Dump 的产物在任意节点上
//	逐字节一致；回灌按同一顺序写入，不依赖任何非确定性来源。
//
// 与 Params 的关系：
//
//	快照包含 params 子空间写入的键（params 直接存于模块 KVStore）。回灌顺序
//	固定为「先 Restore 快照，再 SetParams」，因此 GenesisState.Params 始终是
//	参数的权威来源（人工改过参数的创世文件不会被旧快照里的参数覆盖）。
//	这也让本机制对既有创世文件保持向后兼容：没有 state_kv 字段时行为不变。
package kvgenesis

import (
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Entry 一条原始键值快照记录。
type Entry struct {
	Key   []byte
	Value []byte
}

// MaxRestoreEntries 单次回灌允许的最大条目数。
//
// 快照来自 genesis JSON（本地文件），但仍需要一道上界：畸形或恶意构造的
// 创世文件不应把节点拖进无界内存分配。默认上限远高于任何合理状态规模
// （百万级设备 + 多套索引仍在其内），触发即说明文件本身有问题，应硬失败
// 而不是静默截断 —— 截断等于制造一份「看起来导入成功」的残缺状态。
const MaxRestoreEntries = 20_000_000

// Dump 按字典序导出模块 store 的全部键值。
//
// 只读操作：不修改任何链上状态，可在导出/查询路径安全调用。
func Dump(ctx sdk.Context, storeKey storetypes.StoreKey) []Entry {
	store := ctx.KVStore(storeKey)
	it := store.Iterator(nil, nil)
	defer it.Close()

	out := make([]Entry, 0, 256)
	for ; it.Valid(); it.Next() {
		// 必须复制：迭代器的 Key()/Value() 复用底层缓冲，直接持有会在 Next() 后失效。
		k := append([]byte(nil), it.Key()...)
		v := append([]byte(nil), it.Value()...)
		out = append(out, Entry{Key: k, Value: v})
	}
	return out
}

// Restore 把快照按原顺序写回模块 store。
//
// 幂等性：直接覆写，重复回灌结果一致。若快照超过 MaxRestoreEntries 则 panic
// （创世阶段硬失败优于带病出块）。
func Restore(ctx sdk.Context, storeKey storetypes.StoreKey, entries []Entry) {
	if len(entries) > MaxRestoreEntries {
		panic(fmt.Sprintf("kvgenesis: state snapshot too large: %d entries (max %d)",
			len(entries), MaxRestoreEntries))
	}
	store := ctx.KVStore(storeKey)
	for _, e := range entries {
		if len(e.Key) == 0 {
			// 空键是畸形快照，跳过而不是写入：KVStore 不接受空键，
			// 且静默写入会让后续迭代出现不可预期的行为。
			continue
		}
		store.Set(e.Key, e.Value)
	}
}

// CountEntries 统计模块 store 的条目数（供导出日志/校验使用）。
func CountEntries(ctx sdk.Context, storeKey storetypes.StoreKey) int {
	store := ctx.KVStore(storeKey)
	it := store.Iterator(nil, nil)
	defer it.Close()
	n := 0
	for ; it.Valid(); it.Next() {
		n++
	}
	return n
}

// KV 抽象出各模块 proto 生成的 StateKV 的只读视图。
//
// 各模块的 genesis.proto 各自定义了一份 StateKV（proto 包不同，无法共用一个类型），
// 但它们的键值语义完全一致；这里用最小接口把「读」统一起来，
// 「写」则通过 ToProto 的构造函数注入 —— 避免泛型无法表达结构体字面量的限制。
//
// 注意：proto 生成的 getter 挂在**指针**接收者上，因此 FromProto 需要用
// 指针约束（PT interface{ *T; KV }）来同时表达「*T 满足 KV」与「T 由切片元素推断」。
type KV interface {
	GetKey() []byte
	GetValue() []byte
}

// FromProto 把模块 proto 快照转换为通用 Entry 列表。
func FromProto[T any, PT interface {
	*T
	KV
}](in []T) []Entry {
	if len(in) == 0 {
		return nil
	}
	out := make([]Entry, 0, len(in))
	for i := range in {
		kv := PT(&in[i])
		out = append(out, Entry{Key: kv.GetKey(), Value: kv.GetValue()})
	}
	return out
}

// ToProto 把通用 Entry 列表转换为模块 proto 快照。
//
// make 由调用方提供（通常是 types.StateKV{Key: k, Value: v}），
// 既保持类型安全，又不必为每个模块重复写一遍转换循环。
func ToProto[T any](in []Entry, makeKV func(key, value []byte) T) []T {
	if len(in) == 0 {
		return nil
	}
	out := make([]T, 0, len(in))
	for _, e := range in {
		out = append(out, makeKV(e.Key, e.Value))
	}
	return out
}
