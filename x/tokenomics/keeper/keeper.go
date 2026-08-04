package keeper

import (
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/telemetry"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"mcchain/x/tokenomics/types"
)

type (
	// Keeper 是 tokenomics 的「发行与分配总账」keeper。
	// 唯一持 Minter 的模块；无 Msg service，运行期不增发/不销毁（R3）。
	Keeper struct {
		cdc           codec.BinaryCodec
		storeKey      storetypes.StoreKey
		accountKeeper types.AccountKeeper
		bankKeeper    types.BankKeeper
	}
)

// NewKeeper 构造 tokenomics keeper。
// 生态切片拨付目标模块固定为 types.DepinModuleName（C2：编译期常量，不再依赖调用方传入字符串）。
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
) *Keeper {
	return &Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		accountKeeper: accountKeeper,
		bankKeeper:    bankKeeper,
	}
}

// Logger 返回模块日志器。
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// MintCoins 是整条链唯一的铸币入口。任何铸币都必须使累计 minted_supply <= TotalSupplyCap，
// 否则 panic（R1：总量固化，不可突破）。铸造后累加并持久化 minted_supply。
func (k Keeper) MintCoins(ctx sdk.Context, amt sdk.Coins) error {
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, amt); err != nil {
		return err
	}
	oldMinted := k.GetMintedSupply(ctx)
	newMinted := oldMinted.Add(amt.AmountOf(types.DefaultDenom))
	if newMinted.GT(sdk.NewIntFromUint64(types.TotalSupplyCap)) {
		panic(fmt.Sprintf(
			"tokenomics: minted supply %s would exceed cap %d",
			newMinted.String(), types.TotalSupplyCap,
		))
	}
	k.SetMintedSupply(ctx, newMinted)
	telemetry.IncrCounter(float32(amt.AmountOf(types.DefaultDenom).Int64()), "tokenomics", "minted_amount")
	return nil
}

// BurnMC 是 MC 销毁入口：把代币从 tokenomics 模块账户永久打入黑洞地址。
// 注意这里不再调用 bank.BurnCoins —— 全链销毁统一去向为黑洞地址（R1 总量固化）：
// 总供应量 10 亿恒定不变，销毁体现为「有效流通量」下降，且黑洞余额链上永久可查、
// 任何人可独立核对，比 bank 内部销毁更透明。
func (k Keeper) BurnMC(ctx sdk.Context, amt sdk.Coin) error {
	return k.SendToBlackHole(ctx, types.ModuleName, sdk.NewCoins(amt))
}

// SendToBlackHole 把指定模块账户中的代币永久打入黑洞地址（不可逆、不可支出）。
// 这是 tokenomics 侧的统一销毁出口；其他模块（depin / dex / referral）出于
// 避免 keeper 交叉依赖的考虑，直接向 types.BlackHoleAddress() 转账，语义完全一致。
func (k Keeper) SendToBlackHole(ctx sdk.Context, senderModule string, amt sdk.Coins) error {
	if amt.IsZero() {
		return nil
	}
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(
		ctx, senderModule, types.BlackHoleAddress(), amt,
	); err != nil {
		return err
	}
	burned := amt.AmountOf(types.DefaultDenom)
	telemetry.IncrCounter(float32(burned.Int64()), "tokenomics", "burned_amount")
	ctx.EventManager().EmitEvent(
		sdk.NewEvent("tokenomics.BlackHoleBurn",
			sdk.NewAttribute("from_module", senderModule),
			sdk.NewAttribute("amount", amt.String()),
			sdk.NewAttribute("black_hole", types.BlackHoleAddress().String()),
		),
	)
	return nil
}

// GetBurnedSupply 返回累计已销毁量，即黑洞地址的 MC 余额。
// 黑洞只进不出，故其余额就是销毁总量的权威口径，无需额外计数器。
func (k Keeper) GetBurnedSupply(ctx sdk.Context) sdk.Int {
	return k.bankKeeper.GetBalance(ctx, types.BlackHoleAddress(), types.DefaultDenom).Amount
}

// ProcessEnterpriseSettlementFee applies the enterprise settlement fee policy
// (finalized 2026-08): a settlement `amount` (denom = DefaultDenom) is split as
//
//	40% 分给节点（转入 fee_collector，随出块奖励分配给验证人与委托人）
//	60% 进入国库（protocol_treasury，第 6 个地址，储备用于社区与生态建设）
//
// 国库资金不由任何模块凭空生成，只能像这样由真实业务收入转入（R1 零新印）。
// The caller MUST first transfer `amount` into the tokenomics module account.
// Called by settlement modules (oracle data / device settlement / EdgeAI inference).
func (k Keeper) ProcessEnterpriseSettlementFee(ctx sdk.Context, amount sdk.Int) error {
	if amount.IsZero() {
		return nil
	}
	nodeAmt := amount.MulRaw(int64(types.EnterpriseFeeNodeRatioBps)).QuoRaw(10000)
	treasuryAmt := amount.Sub(nodeAmt) // remainder == EnterpriseFeeTreasuryRatioBps, dust-free

	telemetry.IncrCounter(float32(nodeAmt.Int64()), "tokenomics", "enterprise_fee_nodes")
	telemetry.IncrCounter(float32(treasuryAmt.Int64()), "tokenomics", "enterprise_fee_treasury")

	if nodeAmt.IsPositive() {
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			ctx, types.ModuleName, authtypes.FeeCollectorName,
			sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, nodeAmt)),
		); err != nil {
			return err
		}
	}
	if treasuryAmt.IsPositive() {
		if err := k.bankKeeper.SendCoinsFromModuleToModule(
			ctx, types.ModuleName, types.ProtocolTreasuryPoolName,
			sdk.NewCoins(sdk.NewCoin(types.DefaultDenom, treasuryAmt)),
		); err != nil {
			return err
		}
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent("tokenomics.EnterpriseSettlementFee",
			sdk.NewAttribute("to_nodes", nodeAmt.String()),
			sdk.NewAttribute("to_treasury", treasuryAmt.String()),
		),
	)
	k.Logger(ctx).Info("tokenomics: enterprise settlement fee settled",
		"to_nodes_umc", nodeAmt.String(), "to_treasury_umc", treasuryAmt.String())
	return nil
}
