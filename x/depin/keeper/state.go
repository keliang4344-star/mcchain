package keeper

// DeviceState 记录单个贡献设备的链上状态（持久化于模块 KVStore）。
type DeviceState struct {
	Address     string `json:"address"`
	Model       string `json:"model"`
	OS          string `json:"os"`
	Registered  bool   `json:"registered"`  // 是否已注册
	Attested    bool   `json:"attested"`    // 是否通过 attestation（防女巫）
	TotalReward int    `json:"total_reward"` // 累计获得的 MC 奖励
	TaskCount   int    `json:"task_count"`   // 已入账的贡献任务数
}

// Contribution 单条已验证贡献记录（持久化于模块 KVStore，可审计）。
type Contribution struct {
	TaskID   string `json:"task_id"`
	Device   string `json:"device"`
	TaskType string `json:"task_type"`
	Score    int    `json:"score"`
	Reward   int    `json:"reward"`
}
