package keeper

// DeviceState 记录单个贡献设备的链上状态（持久化于模块 KVStore）。
type DeviceState struct {
	Address     string `json:"address"`
	Model       string `json:"model"`
	OS          string `json:"os"`
	Registered  bool   `json:"registered"`   // 是否已注册
	Attested    bool   `json:"attested"`     // 是否通过 attestation（防女巫）
	TotalReward int    `json:"total_reward"` // 累计获得的 MC 奖励
	TaskCount   int    `json:"task_count"`   // 已入账的贡献任务数

	// AttestedUntil 本次 attestation 的到期 Unix 秒时间戳。
	//
	// Attested 若为永不过期的布尔位：设备成功认证一次即可永久领取
	// DePIN 贡献奖励，与其后 attestation 是否过期、是否被 slash、硬件是否更换无关。
	// 现在 Attested 只是「曾经通过」的标记，真正的闸门是 AttestedUntil：
	// 奖励拨付要求 Attested == true 且 now < AttestedUntil。
	// 0 值表示「未认证或未记录有效期」，按未认证处理（fail-closed）。
	AttestedUntil int64 `json:"attested_until"`
}

// Contribution 单条已验证贡献记录（持久化于模块 KVStore，可审计）。
type Contribution struct {
	TaskID   string `json:"task_id"`
	Device   string `json:"device"`
	TaskType string `json:"task_type"`
	Score    int    `json:"score"`
	Reward   int    `json:"reward"`
}
