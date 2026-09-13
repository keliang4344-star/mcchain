// Package daywindow 提供全链统一的「日界」定义。
//
// 背景：
//
//	若日界在两个经济模块里各有一套互不相干的定义——
//	  · x/depin 用区块时间的日期串：ctx.BlockTime().UTC().Format("20060102")
//	  · x/referral 用区块高度除以一个硬编码除数：ctx.BlockHeight() / 14400
//	而 14400 是「按 6 秒出块」算出来的，本链实际已固化 4 秒出块
//	（创世配置 timeout_commit = "4s"）。于是 referral 的
//	「一天」只有 14400 × 4s = 16 小时，每个真实 24 小时含 1.5 个「日」窗口，
//	全网日上限被等效放宽 1.5 倍——推荐返佣预算因此提前约 3.65 年耗尽。
//
//	更根本的问题是：返佣与挖矿奖励是同一笔经济流的上下游（返佣按挖矿奖励的
//	百分比计提），却受两套不同窗口约束。窗口错位处即错峰套利面。
//
// 本包给出唯一权威口径，全链一律以区块时间（UTC 零点）切日：
//   - 与出块速度解耦：改出块时间不会静默改变「一天」的含义；
//   - 全网确定性一致：BlockTime 是共识输入，不存在节点间分歧；
//   - 语义可读：DayKey 直接打印成 YYYYMMDD，便于对账与排障。
//
// 注意：出块时间相关的容量规划（每块 gas、扫描预算）仍然必须按 4 秒口径计算，
// 本包同时提供 BlockTimeSeconds / BlocksPerDay 作为唯一常量来源，避免再次出现
// 各模块各写一个「每天多少块」的常量而彼此失配。
package daywindow

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// SecondsPerDay 一天的秒数（UTC）。
	SecondsPerDay int64 = 86400

	// BlockTimeSeconds 链上固化的出块间隔（秒）。
	// 依据：出块间隔已固化 timeout_commit = "4s"。
	// 任何「一天多少块」的换算都必须以此为准。
	BlockTimeSeconds int64 = 4

	// BlocksPerDay 每 UTC 日的出块数：86400 / 4 = 21,600。
	// 与 x/dex/types/params.go 的 BlocksPerDay 保持一致（同为 4 秒口径）。
	BlocksPerDay int64 = SecondsPerDay / BlockTimeSeconds
)

// DayIndex 返回区块时间所属的 UTC 日序号（Unix 秒 / 86400）。
//
// 用于需要数值比较的场景（例如键后缀排序、过期键清理判据）。
// 序号严格随区块时间单调递增，因此基于序号的大小比较在共识上是一致的。
//
// 创世 / 单测中区块时间可能为零值（time.Time{} 的 Unix() 是负数），此时统一
// 归为第 0 日，避免负数转 uint64 后回绕成一个极大的值而让「过期」判定失真。
func DayIndex(ctx sdk.Context) uint64 {
	return DayIndexAt(ctx.BlockTime())
}

// DayIndexAt 是 DayIndex 的纯函数形式，便于单测直接构造时间。
func DayIndexAt(t time.Time) uint64 {
	sec := t.Unix()
	if sec < 0 {
		return 0
	}
	return uint64(sec / SecondsPerDay)
}

// DayKey 返回区块时间所属 UTC 日的可读键，格式 YYYYMMDD。
//
// 与 DayIndex 语义完全等价，仅形式不同；用于人可读的存储键与日志。
// 同样对零值时间做归零保护。
func DayKey(ctx sdk.Context) string {
	return DayKeyAt(ctx.BlockTime())
}

// DayKeyAt 是 DayKey 的纯函数形式。
func DayKeyAt(t time.Time) string {
	if t.Unix() < 0 {
		return "19700101"
	}
	return t.UTC().Format("20060102")
}

// DaysBetween 返回两个区块时间之间相隔的整天数（不为负）。
func DaysBetween(from, to time.Time) uint64 {
	a := DayIndexAt(from)
	b := DayIndexAt(to)
	if b <= a {
		return 0
	}
	return b - a
}
