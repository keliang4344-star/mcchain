// Package depin 定义 MobileChain 生产链（B 线 / CometBFT）DePIN 模块的类型与错误。
//
// 设计目标：本包是 Cosmos SDK 自定义模块 x/depin 的“逻辑蓝本”。
// 当前用内存存储实现，可在本机离线编译与单测；未来只需把 Keeper 的底层
// store 换成 Cosmos SDK 的 collections.Store / KVStore，即可无缝升级为真实链上模块。
package depin

import "errors"

// 任务类型（与 A 线、staging 完全一致）。
const (
	TaskTypeInference = "inference"
	TaskTypeDataLabel = "data_label"
	TaskTypeBandwidth = "bandwidth"
)

// 模块级错误。
var (
	ErrDeviceExists    = errors.New("depin: device already registered")
	ErrDeviceNotFound  = errors.New("depin: device not found")
	ErrTaskExists      = errors.New("depin: contribution task id already exists")
	ErrInvalidScore    = errors.New("depin: score out of range [0,100]")
	ErrUnsupportedType = errors.New("depin: unsupported task type")
)

// DeviceState 记录单个贡献设备的链上状态。
type DeviceState struct {
	Address     string // MC 前缀地址，例如 MCxxxx
	Model       string // 设备型号，如 Pixel8 / iPhone15
	OS          string // 系统版本，如 Android14 / iOS17
	Registered  bool   // 是否已注册（通过 attestation）
	TotalReward int    // 累计获得的 MC 奖励
	TaskCount   int    // 已入账的贡献任务数
}

// Contribution 单条已验证贡献记录。
type Contribution struct {
	TaskID   string
	Device   string
	TaskType string
	Score    int
	Reward   int
}
