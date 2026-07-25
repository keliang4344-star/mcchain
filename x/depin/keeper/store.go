package keeper

import (
	"encoding/json"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"mcchain/x/depin/types"
)

// 本文件把 DePIN 模块的业务状态从「内存 map」（蓝本）迁到 Cosmos SDK 模块 KVStore，
// 实现链上持久化与确定性遍历（可审计）。编解码使用 encoding/json（与 state.go 的 json tag 对齐），
// 绕开 collections.Store 的版本耦合风险，100% 编译安全。

var (
	// DeviceKeyPrefix 设备状态前缀；完整键 = Prefix + 设备地址。
	DeviceKeyPrefix = []byte("Device:")
	// ContributionKeyPrefix 贡献记录前缀；完整键 = Prefix + taskID（天然去重）。
	ContributionKeyPrefix = []byte("Contribution:")
)

func deviceKey(addr string) []byte {
	return append(DeviceKeyPrefix, []byte(addr)...)
}

func contributionKey(taskID string) []byte {
	return append(ContributionKeyPrefix, []byte(taskID)...)
}

// SetDevice 持久化设备状态（upsert）。
func (k Keeper) SetDevice(ctx sdk.Context, st *DeviceState) error {
	bz, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("depin: marshal device state: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(deviceKey(st.Address), bz)
	return nil
}

// GetDevice 读取设备状态；不存在返回 ErrDeviceNotFound。
func (k Keeper) GetDevice(ctx sdk.Context, addr string) (*DeviceState, error) {
	bz := ctx.KVStore(k.storeKey).Get(deviceKey(addr))
	if bz == nil {
		return nil, types.ErrDeviceNotFound
	}
	var st DeviceState
	if err := json.Unmarshal(bz, &st); err != nil {
		return nil, fmt.Errorf("depin: unmarshal device state: %w", err)
	}
	return &st, nil
}

// RegisterDevice 注册一台贡献设备（attestation 通过后由 msg 调用）。重复地址报错。
func (k Keeper) RegisterDevice(ctx sdk.Context, addr, model, osVer string) (*DeviceState, error) {
	if _, err := k.GetDevice(ctx, addr); err == nil {
		return nil, types.ErrDeviceExists
	}
	st := &DeviceState{
		Address:    addr,
		Model:      model,
		OS:         osVer,
		Registered: true,
		Attested:   false,
	}
	if err := k.SetDevice(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

// SetContribution 持久化一条贡献记录（upsert，按 taskID 索引）。
func (k Keeper) SetContribution(ctx sdk.Context, c *Contribution) error {
	bz, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("depin: marshal contribution: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(contributionKey(c.TaskID), bz)
	return nil
}

// GetContribution 读取贡献记录；不存在返回 (nil, false)。
func (k Keeper) GetContribution(ctx sdk.Context, taskID string) (*Contribution, bool) {
	bz := ctx.KVStore(k.storeKey).Get(contributionKey(taskID))
	if bz == nil {
		return nil, false
	}
	var c Contribution
	if err := json.Unmarshal(bz, &c); err != nil {
		return nil, false
	}
	return &c, true
}

// SubmitAndReward 提交一条已验证贡献：校验 → 计算奖励 → 入账。
//
// 规则（与 A 线 / staging 等价，引擎来自本包 reward.go）：
//   - 任务类型不支持 → ErrUnsupportedType
//   - 分数越界 [0,100] → ErrInvalidScore
//   - 设备不存在 → ErrDeviceNotFound
//   - 重复 taskID → ErrTaskExists
//   - score < ContributionThreshold(30) → 奖励为 0，但仍记录（贡献被拒）
//   - 否则 reward = score * RewardRate(taskType)，封顶 MaxRewardPerTask(500)
//
// 返回实际入账奖励（可能 0）。发币由 msg_server 在 reward>0 时从 DePIN 池拨付（方案 A）。
func (k Keeper) SubmitAndReward(ctx sdk.Context, taskID, addr, taskType string, score int) (int, error) {
	if !IsValidTaskType(taskType) {
		return 0, types.ErrUnsupportedType
	}
	if score < 0 || score > 100 {
		return 0, types.ErrInvalidScore
	}
	st, err := k.GetDevice(ctx, addr)
	if err != nil {
		return 0, err
	}
	if _, ok := k.GetContribution(ctx, taskID); ok {
		return 0, types.ErrTaskExists
	}

	reward := ComputeReward(score, taskType)
	c := &Contribution{
		TaskID:   taskID,
		Device:   addr,
		TaskType: taskType,
		Score:    score,
		Reward:   reward,
	}
	if err := k.SetContribution(ctx, c); err != nil {
		return 0, err
	}
	st.TotalReward += reward
	st.TaskCount++
	if err := k.SetDevice(ctx, st); err != nil {
		return 0, err
	}
	return reward, nil
}

// DeviceReward 返回设备累计奖励。
func (k Keeper) DeviceReward(ctx sdk.Context, addr string) (int, error) {
	st, err := k.GetDevice(ctx, addr)
	if err != nil {
		return 0, err
	}
	return st.TotalReward, nil
}

// CountDevices 返回已注册设备数（O(n) 前缀遍历，dev 规模足够；可换 counter 优化）。
func (k Keeper) CountDevices(ctx sdk.Context) int {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), DeviceKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	n := 0
	for ; it.Valid(); it.Next() {
		n++
	}
	return n
}

// CountContributions 返回已记录贡献数。
func (k Keeper) CountContributions(ctx sdk.Context) int {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), ContributionKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	n := 0
	for ; it.Valid(); it.Next() {
		n++
	}
	return n
}

// AllContributions 按 taskID 字典序返回全部贡献（确定性，便于对账/审计）。
func (k Keeper) AllContributions(ctx sdk.Context) []Contribution {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), ContributionKeyPrefix)
	it := store.Iterator(nil, nil)
	defer it.Close()
	out := make([]Contribution, 0)
	for ; it.Valid(); it.Next() {
		var c Contribution
		if err := json.Unmarshal(it.Value(), &c); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}
