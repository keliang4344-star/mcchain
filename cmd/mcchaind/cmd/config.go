package cmd

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"mcchain/app"
)

const feeCollectorName = "fee_collector"

func initSDKConfig() {
	// Set prefixes
	accountPubKeyPrefix := app.AccountAddressPrefix + "pub"
	validatorAddressPrefix := app.AccountAddressPrefix + "valoper"
	validatorPubKeyPrefix := app.AccountAddressPrefix + "valoperpub"
	consNodeAddressPrefix := app.AccountAddressPrefix + "valcons"
	consNodePubKeyPrefix := app.AccountAddressPrefix + "valconspub"

	// Set and seal config
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(app.AccountAddressPrefix, accountPubKeyPrefix)
	config.SetBech32PrefixForValidator(validatorAddressPrefix, validatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(consNodeAddressPrefix, consNodePubKeyPrefix)
	config.Seal()

	assertAddressPrefixNotPoisoned()
}

// assertAddressPrefixNotPoisoned 是一道 **启动期自愈守卫**，专门拦截一类
// 「build / vet / test 全绿、链却起不来」的静默故障。
//
// 背景：sdk.AccAddress.String() 走一个全局 LRU 缓存（types/address.go 的 accAddrCache），
// 它以**原始地址字节**为 key，完全不带前缀维度；而 SetBech32PrefixForAccount 并不会
// 失效这个缓存。于是只要有任何代码在「前缀被设置之前」对某个地址调用过 .String()
// ——最典型的就是包级 var 初始化表达式，它在 main() 之前求值——那个 "cosmos1..." 字符串
// 就会被永久钉住，后续无论前缀怎么改都返回旧值。
//
// 后果是 app.New() 里 `authtypes.NewModuleAddress(govtypes.ModuleName).String()` 返回
// cosmos 前缀，bankkeeper.NewBaseKeeper 的 authority 校验 panic：
//
//	panic: invalid bank authority address: invalid Bech32 prefix; expected mc, got cosmos
//
// 真实案例：x/mcchain/keeper 的 DefaultGovernanceHandoverConfig 曾是包级 var，
// 直接让整条链无法启动（见该处注释）。单元测试发现不了它，因为测试进程的
// 初始化顺序与主程序不同。
//
// 这里做两件事：
//  1. 探测一组关键模块账户的地址前缀是否正确——gov / mint / staking / fee_collector
//     / bonded_tokens_pool 等任意一个被污染都可能让 InitChain 在不同位置 panic。
//     只检查 gov 是不全面的：mint / staking 出错会在 BeginBlock 第一次 mint reward
//     或 staking 触发时炸，而那时还以为是「生产 bug」；
//  2. 若任一被污染，**关闭地址缓存**让其重新按当前前缀编码，从而自愈。
//     缓存只是省一次 bech32 编码的性能优化，正确性显然优先；而且该分支
//     正常永远不会触发，所以不构成常规性能开销。
func assertAddressPrefixNotPoisoned() {
	want := app.AccountAddressPrefix + "1"

	// 探测一组在 app.New() 里被 SDK authority 校验或模块账户查询广泛使用的地址。
	// 任一被污染都会在 InitChain 或首次 BeginBlock 触发非「链无法启动」而是更
	// 隐蔽的「特定路径 panic」，提前全部探测可以一次性把整组包级 var 污染定位
	// 干净。模块名常量必须分别 import 是为了避免循环 import（mcchain 顶层包
	// 不引 staketypes 等）。
	probeNames := []string{
		authtypes.ModuleName,    // gov 默认治理账户，bank authority
		minttypes.ModuleName,    // SDK 的默认 InflationCalculationFn 在 BeginBlock 用到
		stakingtypes.ModuleName, // SDK validator set update 路径
		feeCollectorName,        // fee_collector 模块账户
	}

	for _, name := range probeNames {
		got := authtypes.NewModuleAddress(name).String()
		if strings.HasPrefix(got, want) {
			continue // 正常路径：前缀正确
		}
		// 任一被污染：先关闭地址缓存，再统一重新探测确认自愈成功
		sdk.SetAddrCacheEnabled(false)
		var bad []string
		for _, n2 := range probeNames {
			v := authtypes.NewModuleAddress(n2).String()
			if !strings.HasPrefix(v, want) {
				bad = append(bad, fmt.Sprintf("%s=%s", n2, v))
			}
		}
		if len(bad) > 0 {
			panic(fmt.Sprintf(
				"mcchain: bech32 前缀异常且无法自愈——期望前缀 %q，以下地址仍异常：%s。"+
					"请检查是否有包级变量在 init 阶段就对地址调用了 String()（见 initSDKConfig 注释）。",
				app.AccountAddressPrefix, strings.Join(bad, ", ")))
		}
		fmt.Printf("[warn] mcchain: 检测到 bech32 地址缓存被 init 阶段污染，已关闭地址缓存自愈\n")
		return
	}
}
