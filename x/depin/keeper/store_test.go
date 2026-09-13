package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	keepertest "mcchain/testutil/keeper"
	"mcchain/x/depin/keeper"
	"mcchain/x/depin/types"
)

// TestDePinStorePersistence 验证 DePIN 模块 KVStore 持久化 + 奖励引擎（差异化核心）：
// 注册/去重/attest 置位/提交计奖/阈值拒绝/越界与类型校验/累计与遍历。
//
// 注意：keeper 层以字符串为存储键，无需合法 bech32，故测试可直接用任意地址串。
// bech32 校验只在 msg_server 层做。
func TestDePinStorePersistence(t *testing.T) {
	k, ctx := keepertest.DepinKeeper(t)

	// 1) 注册设备
	st, err := k.RegisterDevice(ctx, "device1", "Pixel8", "Android14")
	require.NoError(t, err)
	require.True(t, st.Registered)
	require.False(t, st.Attested)
	require.Equal(t, 1, k.CountDevices(ctx))

	// 2) 重复注册报错
	_, err = k.RegisterDevice(ctx, "device1", "x", "y")
	require.ErrorIs(t, err, types.ErrDeviceExists)

	// 3) attest 置位（模拟 attestation 通过）
	st.Attested = true
	require.NoError(t, k.SetDevice(ctx, st))

	// 4) 合法贡献：inference(score 80) → 80*5 = 400
	reward, err := k.SubmitAndReward(ctx, "task1", "device1", keeper.TaskTypeInference, 80)
	require.NoError(t, err)
	require.Equal(t, 400, reward)

	// 5) 低于阈值（score 20）→ 奖励 0，但仍记录
	reward, err = k.SubmitAndReward(ctx, "task2", "device1", keeper.TaskTypeBandwidth, 20)
	require.NoError(t, err)
	require.Equal(t, 0, reward)

	// 6) 越界分数报错
	_, err = k.SubmitAndReward(ctx, "task3", "device1", keeper.TaskTypeInference, 150)
	require.ErrorIs(t, err, types.ErrInvalidScore)

	// 7) 不支持的任务类型报错
	_, err = k.SubmitAndReward(ctx, "task4", "device1", "mining", 90)
	require.ErrorIs(t, err, types.ErrUnsupportedType)

	// 8) 重复 taskID 报错（task1 已存在）
	_, err = k.SubmitAndReward(ctx, "task1", "device1", keeper.TaskTypeInference, 80)
	require.ErrorIs(t, err, types.ErrTaskExists)

	// 9) 设备累计奖励 = 400（仅 task1 发放）
	total, err := k.DeviceReward(ctx, "device1")
	require.NoError(t, err)
	require.Equal(t, 400, total)

	// 10) 贡献计数 = 2（task1 + task2；task3/4 因错误未落盘）
	require.Equal(t, 2, k.CountContributions(ctx))

	// 11) 全部贡献遍历
	all := k.AllContributions(ctx)
	require.Len(t, all, 2)

	// 12) 不存在设备报错
	_, err = k.GetDevice(ctx, "ghost")
	require.ErrorIs(t, err, types.ErrDeviceNotFound)

	// 13) 不存在设备提交报错
	_, err = k.SubmitAndReward(ctx, "taskX", "ghost", keeper.TaskTypeInference, 80)
	require.ErrorIs(t, err, types.ErrDeviceNotFound)
}
