package liquidstaking

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"mcchain/x/liquidstaking/client/cli"
	"mcchain/x/liquidstaking/keeper"
	"mcchain/x/liquidstaking/types"
)

var (
	_ module.AppModule           = AppModule{}
	_ module.AppModuleBasic      = AppModuleBasic{}
	_ module.AppModuleSimulation = AppModule{}
)

// RewardEpochBlocks is how often accrued staking rewards are compounded back
// into the pool (~6h at 6s blocks).
const RewardEpochBlocks int64 = 3_600

// ----------------------------------------------------------------------------
// AppModuleBasic
// ----------------------------------------------------------------------------

// AppModuleBasic implements the non-dependent module surface.
//
// x/liquidstaking composes x/staking, x/bank and x/distribution. Pool state is
// persisted as JSON in the module KVStore, so genesis is encoded with
// encoding/json rather than the proto codec. The transaction and query
// surfaces are protobuf services defined in proto/mcchain/liquidstaking.
type AppModuleBasic struct {
	cdc codec.BinaryCodec
}

func NewAppModuleBasic(cdc codec.BinaryCodec) AppModuleBasic {
	return AppModuleBasic{cdc: cdc}
}

func (AppModuleBasic) Name() string { return types.ModuleName }

func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

func (AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

func (AppModuleBasic) DefaultGenesis(_ codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(types.DefaultGenesis())
	if err != nil {
		panic(fmt.Sprintf("liquidstaking: marshal default genesis: %v", err))
	}
	return bz
}

func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return gs.Validate()
}

func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	// liquidstaking 的 query.proto 未带 google.api.http 注解、无 .pb.gw.go；这里手动
	// 注册等价 REST 路由，经 abci_query（baseapp gRPC router）转发到 grpc Query service。
	// 路由前缀 /mcchain/liquidstaking/v1/...，供 web 仪表盘与第三方工具查询。
	handleQuery := func(w http.ResponseWriter, grpcPath string, req codec.ProtoMarshaler, resp codec.ProtoMarshaler) error {
		reqBz, err := clientCtx.Codec.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		respBz, _, err := clientCtx.QueryWithData(grpcPath, reqBz)
		if err != nil {
			return err
		}
		if err := clientCtx.Codec.Unmarshal(respBz, resp); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(resp)
	}
	addRest := func(path string, h runtime.HandlerFunc) {
		segs := strings.Split(strings.Trim(path, "/"), "/")
		ops := make([]int, 0, len(segs)*2)
		for i := range segs {
			ops = append(ops, 2, i)
		}
		pat := runtime.MustPattern(runtime.NewPattern(1, ops, segs, "", runtime.AssumeColonVerbOpt(true)))
		mux.Handle(http.MethodGet, pat, h)
	}

	addRest("/mcchain/liquidstaking/v1/params", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		if err := handleQuery(w, "/mcchain.liquidstaking.Query/Params",
			&types.QueryParamsRequest{}, &types.QueryParamsResponse{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
	addRest("/mcchain/liquidstaking/v1/pool", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		if err := handleQuery(w, "/mcchain.liquidstaking.Query/Pool",
			&types.QueryPoolRequest{}, &types.QueryPoolResponse{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
	addRest("/mcchain/liquidstaking/v1/unbondings", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		if err := handleQuery(w, "/mcchain.liquidstaking.Query/Unbondings",
			&types.QueryUnbondingsRequest{Delegator: r.URL.Query().Get("delegator")},
			&types.QueryUnbondingsResponse{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
}

func (AppModuleBasic) GetTxCmd() *cobra.Command { return cli.GetTxCmd() }

func (AppModuleBasic) GetQueryCmd() *cobra.Command { return cli.GetQueryCmd() }

// ----------------------------------------------------------------------------
// AppModule
// ----------------------------------------------------------------------------

type AppModule struct {
	AppModuleBasic

	keeper        keeper.Keeper
	accountKeeper types.AccountKeeper
	bankKeeper    types.BankKeeper
}

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

func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

func (AppModule) RegisterInvariants(sdk.InvariantRegistry) {}

func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, bz json.RawMessage) []abci.ValidatorUpdate {
	gs := types.DefaultGenesis()
	if len(bz) > 0 {
		if err := json.Unmarshal(bz, &gs); err != nil {
			panic(fmt.Sprintf("liquidstaking: unmarshal genesis: %v", err))
		}
	}
	InitGenesis(ctx, am.keeper, gs)
	return []abci.ValidatorUpdate{}
}

func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(ExportGenesis(ctx, am.keeper))
	if err != nil {
		panic(fmt.Sprintf("liquidstaking: marshal genesis: %v", err))
	}
	return bz
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

// BeginBlock compounds accrued staking rewards once per reward epoch.
func (am AppModule) BeginBlock(ctx sdk.Context, _ abci.RequestBeginBlock) {
	if ctx.BlockHeight() == 0 || ctx.BlockHeight()%RewardEpochBlocks != 0 {
		return
	}
	if _, err := am.keeper.AccrueRewards(ctx); err != nil {
		ctx.Logger().Error("liquidstaking: accrue rewards", "err", err)
	}
}

func (AppModule) EndBlock(sdk.Context, abci.RequestEndBlock) []abci.ValidatorUpdate {
	return []abci.ValidatorUpdate{}
}
