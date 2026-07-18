package tokenomics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"mcchain/x/tokenomics/client/cli"
	"mcchain/x/tokenomics/keeper"
	"mcchain/x/tokenomics/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
)

// ----------------------------------------------------------------------------
// AppModuleBasic
// ----------------------------------------------------------------------------

// AppModuleBasic 实现 AppModuleBasic 接口（模块独立的编解码/genesis 校验/cli 等）。
type AppModuleBasic struct {
	cdc codec.BinaryCodec
}

func NewAppModuleBasic(cdc codec.BinaryCodec) AppModuleBasic {
	return AppModuleBasic{cdc: cdc}
}

// Name 返回模块名。
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec 注册 amino 编解码（本模块无 Msg）。
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

// RegisterInterfaces 注册接口类型（本模块无接口实现）。
func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

// DefaultGenesis 返回默认创世状态（json.RawMessage）。
// tokenomics 的创世状态为普通 Go struct，经 encoding/json 序列化，
// 不依赖 protobuf 生成代码（B1 修复：绕过 codec.JSONCodec 的 protojson 约束）。
func (AppModuleBasic) DefaultGenesis(_ codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(types.DefaultGenesis())
	if err != nil {
		panic(fmt.Sprintf("tokenomics: failed to marshal default genesis: %v", err))
	}
	return bz
}

// ValidateGenesis 校验创世状态。
func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var genState types.GenesisState
	if err := json.Unmarshal(bz, &genState); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genState.Validate()
}

// RegisterGRPCGatewayRoutes 注册 gRPC Gateway 路由。
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))
}

// GetTxCmd 返回模块根交易命令（tokenomics 无 Msg service，返回 nil）。
func (a AppModuleBasic) GetTxCmd() *cobra.Command {
	return nil
}

// GetQueryCmd 返回模块根查询命令。
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd(types.StoreKey)
}

// ----------------------------------------------------------------------------
// AppModule
// ----------------------------------------------------------------------------

// AppModule 实现 AppModule 接口（模块间依赖方法）。
type AppModule struct {
	AppModuleBasic

	keeper        keeper.Keeper
	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
}

// NewAppModule 构造 tokenomics AppModule。
func NewAppModule(
	cdc codec.Codec,
	k keeper.Keeper,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
) AppModule {
	return AppModule{
		AppModuleBasic: NewAppModuleBasic(cdc),
		keeper:         k,
		accountKeeper:  accountKeeper,
		bankKeeper:     bankKeeper,
	}
}

// RegisterServices 注册 gRPC 查询服务（无 Msg service）。
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

// RegisterInvariants 注册 tokenomics 不变量（R1 minted<=cap；R2 池和==minted）。
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {
	ir.RegisterRoute(types.ModuleName, "minted-supply", keeper.MintedSupplyInvariant(am.keeper))
	ir.RegisterRoute(types.ModuleName, "pool-sum", keeper.PoolSumInvariant(am.keeper))
}

// InitGenesis 执行模块创世初始化。
func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate {
	var genState types.GenesisState
	if err := json.Unmarshal(gs, &genState); err != nil {
		panic(fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err))
	}

	InitGenesis(ctx, am.keeper, genState)

	return []abci.ValidatorUpdate{}
}

// ExportGenesis 导出模块创世状态（json.RawMessage）。
func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	genState := ExportGenesis(ctx, am.keeper)
	bz, err := json.Marshal(genState)
	if err != nil {
		panic(fmt.Errorf("tokenomics: failed to marshal export genesis: %w", err))
	}
	return bz
}

// ConsensusVersion 返回共识版本（首版为 1）。
func (AppModule) ConsensusVersion() uint64 { return 1 }

// BeginBlock 模块 begin block 逻辑：
//  - 每 100 区块执行 gas 费回流：将 fee_collector 余额的 10% 转入安全池
//  - 每 100 区块执行安全池滴灌：将安全池余额的 5% 转入分布模块按质押比例分配
func (am AppModule) BeginBlock(ctx sdk.Context, _ abci.RequestBeginBlock) {
	// 仅每 DripIntervalBlocks 执行一次，避免每块都转账
	if ctx.BlockHeight()%keeper.DripIntervalBlocks != 0 {
		return
	}

	// Gas 回流：fee_collector → staking_security (10%)
	if err := am.keeper.RebateGasFeesToSecurity(ctx); err != nil {
		am.keeper.Logger(ctx).Error("tokenomics: gas rebate failed in BeginBlock", "err", err.Error())
	}

	// 安全池滴灌：staking_security → distribution (5%)
	if err := am.keeper.DripStakingSecurity(ctx); err != nil {
		am.keeper.Logger(ctx).Error("tokenomics: drip failed in BeginBlock", "err", err.Error())
	}
}

// EndBlock 模块 end block 逻辑（本模块无）。
func (am AppModule) EndBlock(_ sdk.Context, _ abci.RequestEndBlock) []abci.ValidatorUpdate {
	return []abci.ValidatorUpdate{}
}
