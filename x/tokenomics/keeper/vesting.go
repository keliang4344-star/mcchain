package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"

	"mcchain/x/tokenomics/types"
)

// CreateTeamVestingAccount 在团队多签地址上创建连续锁仓账户并写入状态，
// 实现「1 年 cliff（0 释放）+ 3 年线性（总 4 年）」释放模型（Q3/Q6）。
// originalVesting 必须等于拨付给团队池的金额（1.5e14 umc）。
func (k Keeper) CreateTeamVestingAccount(ctx sdk.Context, originalVesting sdk.Coins, startTime, endTime int64) error {
	teamAddr := types.TeamAddress
	baseAcc := k.accountKeeper.NewAccountWithAddress(ctx, teamAddr)
	ba, ok := baseAcc.(*authtypes.BaseAccount)
	if !ok {
		return fmt.Errorf("tokenomics: expected base account for team address")
	}
	ba.SetPubKey(types.TeamMultisigPubKey)
	bva := vestingtypes.NewBaseVestingAccount(ba, originalVesting, endTime)
	cva := vestingtypes.NewContinuousVestingAccountRaw(bva, startTime)
	k.accountKeeper.SetAccount(ctx, cva)
	return nil
}

// ComputeVested 按线性模型实时计算已释放额度（Q9：曲线元数据缓存、进度实时算）。
//   - now <= startTime（cliff 内）：vested = 0
//   - now >= endTime：vested = totalLocked
//   - 之间：线性 from startTime to endTime
//
// 返回 vested（已释放）、remaining（未释放）、progressBps（释放进度，基点）。
func ComputeVested(totalLocked uint64, startTime, endTime int64, now int64) (vested, remaining uint64, progressBps uint32) {
	if now <= startTime {
		return 0, totalLocked, 0
	}
	if now >= endTime || startTime >= endTime {
		return totalLocked, 0, 10000
	}
	span := endTime - startTime
	elapsed := now - startTime
	// 防溢出：totalLocked * elapsed 对大锁仓额（如 1.5e14 * 4.7e7 ≈ 7e21）会超过
	// uint64 上限（1.8e19），必须先用 sdk.Int 做大数乘法再除，否则释放额静默错算。
	vested = sdk.NewInt(int64(totalLocked)).Mul(sdk.NewInt(elapsed)).Quo(sdk.NewInt(span)).Uint64()
	if vested > totalLocked {
		vested = totalLocked
	}
	remaining = totalLocked - vested
	if totalLocked > 0 {
		progressBps = uint32(vested * 10000 / totalLocked)
	}
	return vested, remaining, progressBps
}
