// Package chainprofile 提供 chain-id → 部署档位（生产/非生产）的显式判定。
//
// 为什么需要显式白名单：早期实现是「黑名单式」——只要 chain-id 不含
// test/dev/local/sim 就按生产处理。副作用是任意命名的新链（如 mcchain-qa-2）
// 会被误判为生产而触发生产闸门；而生产链若命名不规范（名字里碰巧带这些
// 子串）又会误放行，判定依据完全依赖命名巧合。
//
// 本包改为「显式白名单 + 黑名单标记 + fail-closed 兜底」：
//   - 精确命中 KnownProductionChainIDs 的 chain-id 一律按生产；
//   - 含 test/dev/local/sim 标记的 chain-id 一律按非生产（兼容本地开发/CI）；
//   - 其余未识别 chain-id 一律按生产（fail-closed：闸门向「拦截」倾斜）。
//
// 新增主网链名时必须同步登记 KnownProductionChainIDs，否则会被兜底按生产
// 处理（安全侧，不会误放行）。
package chainprofile

import "strings"

// KnownProductionChainIDs 是当前已上线/规划上线的主网 chain-id 精确集合。
var KnownProductionChainIDs = map[string]bool{
	"mcchain-mainnet-1": true,
	"mcchain-mainnet":   true,
}

// NonProductionMarkers 是明确声明「非生产」的命名标记（本地 devnet / CI / 测试网）。
var NonProductionMarkers = []string{"test", "dev", "local", "sim"}

// IsProduction 判定 chain-id 是否属于生产部署。
func IsProduction(chainID string) bool {
	id := strings.ToLower(strings.TrimSpace(chainID))
	if KnownProductionChainIDs[id] {
		return true
	}
	for _, m := range NonProductionMarkers {
		if strings.Contains(id, m) {
			return false
		}
	}
	// fail-closed：无法识别的 chain-id 一律按生产拦截。
	return true
}
