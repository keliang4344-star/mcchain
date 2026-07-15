module mcchain-cosmos

go 1.22

require mcchain-staging v0.0.0

// 复用 A 线提炼出的生产级经济逻辑（DePIN 奖励引擎 + 国库），离线可用、零网络依赖。
// 真实上链时，本模块 keeper 直接包裹 Cosmos SDK 的 Keeper/Store，逻辑无需改写。
replace mcchain-staging => ../mcchain_staging
