package depin

import (
	"fmt"
	"sync"

	"mcchain-staging/depin" // 复用 A 线提炼的奖励引擎（inference5x/data_label3x/bandwidth1x）
)

// Keeper 持有 DePIN 模块的链上状态。当前为内存实现，接口与 Cosmos SDK
// Keeper 对齐（Set/Get + 业务方法），未来可平滑替换为 SDK Store 后端。
type Keeper struct {
	mu          sync.RWMutex
	devices     map[string]*DeviceState
	contribs    map[string]*Contribution
	contribList []string // 保持插入顺序，便于确定性遍历
}

// NewKeeper 构造空 Keeper。
func NewKeeper() *Keeper {
	return &Keeper{
		devices:  make(map[string]*DeviceState),
		contribs: make(map[string]*Contribution),
	}
}

// RegisterDevice 注册一台贡献设备（由 attestation 通过后调用）。重复注册报错。
func (k *Keeper) RegisterDevice(addr, model, osVer string) (*DeviceState, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.devices[addr]; ok {
		return nil, ErrDeviceExists
	}
	st := &DeviceState{Address: addr, Model: model, OS: osVer, Registered: true}
	k.devices[addr] = st
	return st, nil
}

// GetDevice 查询设备状态。
func (k *Keeper) GetDevice(addr string) (*DeviceState, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	st, ok := k.devices[addr]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return st, nil
}

// SubmitAndReward 提交一条已验证贡献：校验 → 计算奖励 → 入账。
//
// 规则（与 A 线 verifyTask 等价，引擎来自 staging/depin）：
//   - 任务类型不支持 → ErrUnsupportedType
//   - 分数越界 [0,100] → ErrInvalidScore（引擎返回 0，这里显式报错更明确）
//   - 重复 taskID → ErrTaskExists
//   - score < 30（ContributionThreshold）→ 奖励为 0，但仍记录（贡献被拒）
//   - 否则 reward = score * rate，封顶 500
//
// 返回实际发放的奖励（可能 0）与错误。
func (k *Keeper) SubmitAndReward(taskID, addr, taskType string, score int) (int, error) {
	if !depin.IsValidTaskType(taskType) {
		return 0, ErrUnsupportedType
	}
	if score < 0 || score > 100 {
		return 0, ErrInvalidScore
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	st, ok := k.devices[addr]
	if !ok {
		return 0, ErrDeviceNotFound
	}
	if _, dup := k.contribs[taskID]; dup {
		return 0, ErrTaskExists
	}

	reward := depin.ComputeReward(score, taskType) // 引擎已封顶 + 阈值处理
	c := &Contribution{
		TaskID:   taskID,
		Device:   addr,
		TaskType: taskType,
		Score:    score,
		Reward:   reward,
	}
	k.contribs[taskID] = c
	k.contribList = append(k.contribList, taskID)
	st.TotalReward += reward
	st.TaskCount++
	return reward, nil
}

// DeviceReward 返回设备累计奖励。
func (k *Keeper) DeviceReward(addr string) (int, error) {
	st, err := k.GetDevice(addr)
	if err != nil {
		return 0, err
	}
	return st.TotalReward, nil
}

// CountDevices / CountContributions 统计。
func (k *Keeper) CountDevices() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.devices)
}

func (k *Keeper) CountContributions() int {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return len(k.contribs)
}

// AllContributions 按提交顺序返回贡献列表（确定性，便于对账）。
func (k *Keeper) AllContributions() []Contribution {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]Contribution, 0, len(k.contribList))
	for _, id := range k.contribList {
		out = append(out, *k.contribs[id])
	}
	return out
}

// String 便于调试与日志。
func (c Contribution) String() string {
	return fmt.Sprintf("task=%s dev=%s type=%s score=%d reward=%d",
		c.TaskID, c.Device, c.TaskType, c.Score, c.Reward)
}
