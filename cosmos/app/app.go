// Package app 是 MobileChain 生产链（B 线 / CometBFT）的“逻辑总装层”。
//
// 当前用纯内存实现：持有各模块的 Keeper，模拟一个最小化的出块周期
// （交易池 → Commit 出块 → 处理贡献 → mint 奖励）。这一层不需要 cosmos-sdk
// 即可离线编译与单测，用于验证生产链的经济闭环是否成立。
//
// 未来升级为真实 CometBFT 链时：
//   - 各 Keeper 的底层 store 换成 Cosmos SDK collections.Store；
//   - Commit() 的逻辑搬进 EndBlocker / 交易处理器（msg server）；
//   - App 结构升级为 Cosmos SDK 的 app.App（含 BaseApp + 模块管理器）。
// 业务语义（奖励、验证、证明）保持不变。
package app

import (
	"sync"

	"mcchain-cosmos/x/depin"
	"mcchain-cosmos/x/phonenode"
)

// PendingContribution 交易池中等待出块打包的贡献交易。
type PendingContribution struct {
	Device   string
	TaskID   string
	TaskType string
	Score    int
}

// App 生产链逻辑总装。
type App struct {
	mu        sync.Mutex
	Depin     *depin.Keeper
	PhoneNode *phonenode.Keeper

	Height  int64  // 当前链高
	Minted  uint64 // 累计已 mint 的 DePIN 奖励（MC）
	pending []PendingContribution
}

// New 构造空链。
func New() *App {
	return &App{
		Depin:     depin.NewKeeper(),
		PhoneNode: phonenode.NewKeeper(),
	}
}

// RegisterDevice 便捷封装：在 DePIN 模块注册一台贡献设备。
func (a *App) RegisterDevice(addr, model, osVer string) error {
	_, err := a.Depin.RegisterDevice(addr, model, osVer)
	return err
}

// SubmitContribution 把一条贡献放入交易池（待下一个区块打包）。
func (a *App) SubmitContribution(dev, taskID, taskType string, score int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending = append(a.pending, PendingContribution{dev, taskID, taskType, score})
}

// PendingCount 返回未打包贡献数。
func (a *App) PendingCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}

// Commit 模拟一个出块周期：处理交易池中所有待打包贡献，计算并发放奖励，
// 累计 mint 总量。返回本区块成功处理的贡献数。
//
// 生产链语义：失败的贡献（设备未知/类型非法/重复）进入失败收据并跳过，
// 不中断出块（与真实链一致：单笔坏交易不炸整个区块）。
func (a *App) Commit() (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Height++
	processed := 0
	for _, pc := range a.pending {
		reward, err := a.Depin.SubmitAndReward(pc.TaskID, pc.Device, pc.TaskType, pc.Score)
		if err != nil {
			// 失败收据：这里仅跳过，真实链会写事件/回执
			continue
		}
		a.Minted += uint64(reward)
		processed++
	}
	a.pending = nil
	return processed, nil
}

// DeviceReward 便捷查询设备累计奖励。
func (a *App) DeviceReward(addr string) (int, error) {
	return a.Depin.DeviceReward(addr)
}

// SubmitLightProof 便捷封装：手机轻节点提交 Merkle 状态证明。
func (a *App) SubmitLightProof(addr string, root, leaf []byte, index int, proof [][]byte) (bool, error) {
	return a.PhoneNode.SubmitStateProof(addr, root, leaf, index, proof)
}
