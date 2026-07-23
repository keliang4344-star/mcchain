package keeper

import (
	"encoding/json"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ============================================================================
// 线性摊薄释放 — 白皮书行 372
// ============================================================================
//
// MobileChain DePIN 设备激励金库采用线性摊薄释放机制，将一次性大额激励
// 平滑分散到约 11 年的生命周期中，避免：
//   - 早期参与者一次性获取过多代币后抛售
//   - 后期参与者无激励可领（金库枯竭）
//   - 短期刷量者获取不成比例的奖励
//
// 释放规则：
//   每日释放上限 = 金库当前余额 / 剩余天数
//   默认周期 4015 天（≈ 11 年），与 tokenomics 的五年双池模型对齐
//
// 注：本文件实现的是每日全局释放上限检查，与 per-device 的日收益上限
// （defense.go Layer7）形成双层经济约束。

// ---------------------------------------------------------------------------
// 释放常量
// ---------------------------------------------------------------------------

const (
	// DefaultVaultDays 默认金库释放总天数（11 年 × 365.25 ≈ 4016）。
	// 取整为 4015 天，与白皮书对齐。
	DefaultVaultDays = 4015

	// DaySeconds 一天的秒数。
	DaySeconds = 86400
)

// ---------------------------------------------------------------------------
// KVStore 前缀
// ---------------------------------------------------------------------------

var (
	// ReleaseVaultKey 存储金库元数据。
	ReleaseVaultKey = []byte("ReleaseVault")

	// ReleaseDailyPrefix 存储每日释放记录（键格式: prefix + YYYY-MM-DD）。
	ReleaseDailyPrefix = []byte("ReleaseDaily:")
)

func releaseDailyKey(day string) []byte {
	return append(ReleaseDailyPrefix, []byte(day)...)
}

// ---------------------------------------------------------------------------
// 数据结构
// ---------------------------------------------------------------------------

// ReleaseVault 记录 DePIN 激励金库的线性释放状态。
type ReleaseVault struct {
	// StartTime 金库启用时间（Unix 秒时间戳）。首次拨付时自动初始化。
	StartTime int64 `json:"start_time"`

	// TotalDays 金库释放总天数，默认 DefaultVaultDays。
	TotalDays int64 `json:"total_days"`

	// InitialBalance 金库初始余额（单位 umc），在首次拨付时记录。
	InitialBalance uint64 `json:"initial_balance"`

	// TotalReleased 累计已释放金额（单位 umc）。
	TotalReleased uint64 `json:"total_released"`
}

// ReleaseDaily 记录单日的释放情况。
type ReleaseDaily struct {
	// Date 日期（YYYY-MM-DD 格式）。
	Date string `json:"date"`

	// Released 当日已释放金额（单位 umc）。
	Released uint64 `json:"released"`

	// Cap 当日释放上限（单位 umc），记录时的快照值。
	Cap uint64 `json:"cap"`
}

// ============================================================================
// Keeper 方法：金库管理
// ============================================================================

// GetOrInitReleaseVault 获取或初始化释放金库。
//
// 金库在首次调用时自动初始化：从 DePIN 模块账户余额中读取当前余额作为
// InitialBalance，以当前时间作为 StartTime。若金库已存在，直接返回。
func (k Keeper) GetOrInitReleaseVault(ctx sdk.Context) (*ReleaseVault, error) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(ReleaseVaultKey)
	if bz != nil {
		var vault ReleaseVault
		if err := json.Unmarshal(bz, &vault); err != nil {
			return nil, fmt.Errorf("depin: unmarshal release vault: %w", err)
		}
		return &vault, nil
	}

	// 金库首次初始化：查询模块账户余额
	denom := k.GetParams(ctx).RewardDenom
	moduleAddr := k.getModuleAddress()
	coins := k.bankKeeper.SpendableCoins(ctx, moduleAddr)
	balance := coins.AmountOf(denom).Uint64()

	vault := &ReleaseVault{
		StartTime:      ctx.BlockTime().Unix(),
		TotalDays:      DefaultVaultDays,
		InitialBalance: balance,
		TotalReleased:  0,
	}

	newBz, err := json.Marshal(vault)
	if err != nil {
		return nil, fmt.Errorf("depin: marshal release vault: %w", err)
	}
	store.Set(ReleaseVaultKey, newBz)

	k.Logger(ctx).Info("depin release vault initialized",
		"initial_balance", balance,
		"total_days", DefaultVaultDays,
		"start_time", vault.StartTime,
	)

	return vault, nil
}

// SaveReleaseVault 持久化释放金库状态。
func (k Keeper) SaveReleaseVault(ctx sdk.Context, vault *ReleaseVault) error {
	bz, err := json.Marshal(vault)
	if err != nil {
		return fmt.Errorf("depin: marshal release vault: %w", err)
	}
	ctx.KVStore(k.storeKey).Set(ReleaseVaultKey, bz)
	return nil
}

// ============================================================================
// Keeper 方法：每日释放额度检查
// ============================================================================

// CheckDailyReleaseCap 检查本次拨付是否在当日释放额度内。
//
// 调用时机：每次 PayoutReward / SubmitContribution 发币前。
//
// 参数：
//   - amount: 本次计划拨付的金额（单位 umc）
//
// 返回值：
//   - allowed:   本次拨付是否允许
//   - dailyCap:  当前日释放上限（单位 umc）
//   - remaining: 当日剩余可释放额度（单位 umc）
//   - err:       错误信息
//
// 若 allowed == false，调用方应拒绝本次拨付。
func (k Keeper) CheckDailyReleaseCap(ctx sdk.Context, amount uint64) (allowed bool, dailyCap uint64, remaining uint64, err error) {
	vault, err := k.GetOrInitReleaseVault(ctx)
	if err != nil {
		return false, 0, 0, err
	}

	// 计算已过天数
	elapsedSeconds := ctx.BlockTime().Unix() - vault.StartTime
	elapsedDays := elapsedSeconds / DaySeconds
	if elapsedDays < 0 {
		elapsedDays = 0
	}

	// 剩余天数
	remainingDays := vault.TotalDays - elapsedDays
	if remainingDays <= 0 {
		remainingDays = 1 // 最后一天兜底，确保金库可完全释放
	}

	// 当前金库余额 = 初始余额 - 累计已释放
	currentBalance := vault.InitialBalance
	if vault.TotalReleased <= currentBalance {
		currentBalance -= vault.TotalReleased
	} else {
		currentBalance = 0
	}

	// 日释放上限 = 当前余额 / 剩余天数
	dailyCap = currentBalance / uint64(remainingDays)

	// 读取当日已释放金额
	todayDate := time.Now().UTC().Format("2006-01-02")
	store := ctx.KVStore(k.storeKey)

	var todayReleased uint64
	bz := store.Get(releaseDailyKey(todayDate))
	if bz != nil {
		var rd ReleaseDaily
		if err := json.Unmarshal(bz, &rd); err == nil {
			todayReleased = rd.Released
		}
	}

	// 当日剩余额度
	if dailyCap > todayReleased {
		remaining = dailyCap - todayReleased
	} else {
		remaining = 0
	}

	if amount > remaining {
		return false, dailyCap, remaining, nil
	}

	return true, dailyCap, remaining, nil
}

// RecordDailyRelease 记录当日释放金额，在拨付成功之后调用。
func (k Keeper) RecordDailyRelease(ctx sdk.Context, amount uint64) {
	todayDate := time.Now().UTC().Format("2006-01-02")
	store := ctx.KVStore(k.storeKey)

	var rd ReleaseDaily
	bz := store.Get(releaseDailyKey(todayDate))
	if bz != nil {
		_ = json.Unmarshal(bz, &rd)
	}
	rd.Date = todayDate
	rd.Released += amount

	// 同时更新金库累计
	vault, err := k.GetOrInitReleaseVault(ctx)
	if err == nil {
		vault.TotalReleased += amount
		_ = k.SaveReleaseVault(ctx, vault)
	}

	newBz, _ := json.Marshal(rd)
	store.Set(releaseDailyKey(todayDate), newBz)
}

// getModuleAddress 返回 DePIN 模块账户地址。
func (k Keeper) getModuleAddress() sdk.AccAddress {
	return sdk.AccAddress([]byte(types.ModuleName))
}
