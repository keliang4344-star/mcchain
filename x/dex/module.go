package dex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"mcchain/x/dex/client/cli"
	"mcchain/x/dex/keeper"
	"mcchain/x/dex/types"
)

var (
	_ module.AppModule           = AppModule{}
	_ module.AppModuleBasic      = AppModuleBasic{}
	_ module.AppModuleSimulation = AppModule{}
)

// ----------------------------------------------------------------------------
// AppModuleBasic
// ----------------------------------------------------------------------------

type AppModuleBasic struct {
	cdc codec.BinaryCodec
}

func NewAppModuleBasic(cdc codec.BinaryCodec) AppModuleBasic {
	return AppModuleBasic{cdc: cdc}
}

func (AppModuleBasic) Name() string {
	return types.ModuleName
}

func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesis())
}

func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var genState types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &genState); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genState.Validate()
}

func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	// dex 的 query.proto 未带 google.api.http 注解、无 .pb.gw.go；这里手动注册等价
	// REST 路由：经 abci_query（baseapp gRPC router）转发到 grpc Query service。
	// 路由前缀 /mcchain/dex/v1/...，供 web 仪表盘与第三方工具查询。
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
	// addRest 把 "/a/b/c" 转为 grpc-gateway v1 的 Pattern 并注册 GET 路由。
	addRest := func(path string, h runtime.HandlerFunc) {
		segs := strings.Split(strings.Trim(path, "/"), "/")
		ops := make([]int, 0, len(segs)*2)
		for i := range segs {
			ops = append(ops, 2, i)
		}
		pat := runtime.MustPattern(runtime.NewPattern(1, ops, segs, "", runtime.AssumeColonVerbOpt(true)))
		mux.Handle(http.MethodGet, pat, h)
	}

	addRest("/mcchain/dex/v1/pools", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		if err := handleQuery(w, "/mcchain.dex.Query/Pools", &types.QueryPoolsRequest{}, &types.QueryPoolsResponse{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
	addRest("/mcchain/dex/v1/pool", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		poolID, _ := strconv.ParseUint(r.URL.Query().Get("pool_id"), 10, 64)
		if err := handleQuery(w, "/mcchain.dex.Query/Pool", &types.QueryPoolRequest{PoolId: poolID}, &types.QueryPoolResponse{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
	addRest("/mcchain/dex/v1/price", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		poolID, _ := strconv.ParseUint(r.URL.Query().Get("pool_id"), 10, 64)
		if err := handleQuery(w, "/mcchain.dex.Query/Price",
			&types.QueryPriceRequest{PoolId: poolID, Denom: r.URL.Query().Get("denom")},
			&types.QueryPriceResponse{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
	addRest("/mcchain/dex/v1/estimate_swap", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		poolID, _ := strconv.ParseUint(r.URL.Query().Get("pool_id"), 10, 64)
		if err := handleQuery(w, "/mcchain.dex.Query/EstimateSwap",
			&types.QueryEstimateSwapRequest{
				PoolId:   poolID,
				DenomIn:  r.URL.Query().Get("denom_in"),
				DenomOut: r.URL.Query().Get("denom_out"),
				AmountIn: r.URL.Query().Get("amount_in"),
			},
			&types.QueryEstimateSwapResponse{}); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	})
}

func (a AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.GetTxCmd()
}

func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

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
	keeper keeper.Keeper,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
) AppModule {
	return AppModule{
		AppModuleBasic: NewAppModuleBasic(cdc),
		keeper:         keeper,
		accountKeeper:  accountKeeper,
		bankKeeper:     bankKeeper,
	}
}

func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

func (am AppModule) RegisterInvariants(_ sdk.InvariantRegistry) {}

func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate {
	var genState types.GenesisState
	cdc.MustUnmarshalJSON(gs, &genState)

	InitGenesis(ctx, am.keeper, genState)

	return []abci.ValidatorUpdate{}
}

func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	genState := ExportGenesis(ctx, am.keeper)
	return cdc.MustMarshalJSON(genState)
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

func (am AppModule) BeginBlock(ctx sdk.Context, _ abci.RequestBeginBlock) {
	// Daily LP incentive distribution (whitepaper line 507).
	if uint64(ctx.BlockHeight())%types.BlocksPerDay == 0 {
		am.keeper.DistributeLPIncentive(ctx)
	}
}

func (am AppModule) EndBlock(_ sdk.Context, _ abci.RequestEndBlock) []abci.ValidatorUpdate {
	return []abci.ValidatorUpdate{}
}
