package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/log"
	tmos "github.com/cometbft/cometbft/libs/os"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	nodeservice "github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/capability"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	"github.com/cosmos/cosmos-sdk/x/crisis"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/evidence"
	evidencekeeper "github.com/cosmos/cosmos-sdk/x/evidence/keeper"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/cosmos-sdk/x/group"
	groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"
	groupmodule "github.com/cosmos/cosmos-sdk/x/group/module"
	"github.com/cosmos/cosmos-sdk/x/mint"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/params"
	paramsclient "github.com/cosmos/cosmos-sdk/x/params/client"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	paramproposal "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/cosmos-sdk/x/upgrade"
	upgradeclient "github.com/cosmos/cosmos-sdk/x/upgrade/client"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ica "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/types"
	icahost "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/host"
	icahostkeeper "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	"github.com/cosmos/ibc-go/v7/modules/apps/transfer"
	ibctransferkeeper "github.com/cosmos/ibc-go/v7/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v7/modules/core"
	ibcclient "github.com/cosmos/ibc-go/v7/modules/core/02-client"
	ibcclientclient "github.com/cosmos/ibc-go/v7/modules/core/02-client/client"
	ibcclienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	ibcporttypes "github.com/cosmos/ibc-go/v7/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v7/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
	solomachine "github.com/cosmos/ibc-go/v7/modules/light-clients/06-solomachine"
	ibctm "github.com/cosmos/ibc-go/v7/modules/light-clients/07-tendermint"
	"github.com/spf13/cast"

	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"

	depinmodule "mcchain/x/depin"
	depinmodulekeeper "mcchain/x/depin/keeper"
	depinmoduletypes "mcchain/x/depin/types"
	dexmodule "mcchain/x/dex"
	dexmodulekeeper "mcchain/x/dex/keeper"
	dexmoduletypes "mcchain/x/dex/types"
	edgeaimodule "mcchain/x/edgeai"
	edgeaimodulekeeper "mcchain/x/edgeai/keeper"
	edgeaimoduletypes "mcchain/x/edgeai/types"
	liquidstakingmodule "mcchain/x/liquidstaking"
	liquidstakingmodulekeeper "mcchain/x/liquidstaking/keeper"
	liquidstakingmoduletypes "mcchain/x/liquidstaking/types"
	mcchainmodule "mcchain/x/mcchain"
	mcchainmodulekeeper "mcchain/x/mcchain/keeper"
	mcchainmoduletypes "mcchain/x/mcchain/types"
	phonenodemodule "mcchain/x/phonenode"
	phonenodemodulekeeper "mcchain/x/phonenode/keeper"
	phonenodemoduletypes "mcchain/x/phonenode/types"
	referralmodule "mcchain/x/referral"
	referralmodulekeeper "mcchain/x/referral/keeper"
	referralmoduletypes "mcchain/x/referral/types"
	tokenomicsmodule "mcchain/x/tokenomics"
	tokenomicsmodulekeeper "mcchain/x/tokenomics/keeper"
	tokenomicsmoduletypes "mcchain/x/tokenomics/types"
	// this line is used by starport scaffolding # stargate/app/moduleImport

	appparams "mcchain/app/params"
	"mcchain/docs"
)

const (
	AccountAddressPrefix = "mc"
	Name                 = "mcchain"
)

// this line is used by starport scaffolding # stargate/wasm/app/enabledProposals

func getGovProposalHandlers() []govclient.ProposalHandler {
	var govProposalHandlers []govclient.ProposalHandler
	// this line is used by starport scaffolding # stargate/app/govProposalHandlers

	govProposalHandlers = append(govProposalHandlers,
		paramsclient.ProposalHandler,
		upgradeclient.LegacyProposalHandler,
		upgradeclient.LegacyCancelProposalHandler,
		ibcclientclient.UpdateClientProposalHandler,
		ibcclientclient.UpgradeProposalHandler,
		// this line is used by starport scaffolding # stargate/app/govProposalHandler
	)

	return govProposalHandlers
}

var (
	// DefaultNodeHome default home directories for the application daemon
	DefaultNodeHome string

	// ModuleBasics defines the module BasicManager is in charge of setting up basic,
	// non-dependant module elements, such as codec registration
	// and genesis verification.
	ModuleBasics = module.NewBasicManager(
		auth.AppModuleBasic{},
		authzmodule.AppModuleBasic{},
		genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
		bank.AppModuleBasic{},
		capability.AppModuleBasic{},
		staking.AppModuleBasic{},
		mint.AppModuleBasic{},
		distr.AppModuleBasic{},
		gov.NewAppModuleBasic(getGovProposalHandlers()),
		params.AppModuleBasic{},
		crisis.AppModuleBasic{},
		slashing.AppModuleBasic{},
		feegrantmodule.AppModuleBasic{},
		groupmodule.AppModuleBasic{},
		ibc.AppModuleBasic{},
		ibctm.AppModuleBasic{},
		solomachine.AppModuleBasic{},
		upgrade.AppModuleBasic{},
		evidence.AppModuleBasic{},
		transfer.AppModuleBasic{},
		ica.AppModuleBasic{},
		vesting.AppModuleBasic{},
		consensus.AppModuleBasic{},
		mcchainmodule.AppModuleBasic{},
		tokenomicsmodule.AppModuleBasic{},
		depinmodule.AppModuleBasic{},
		phonenodemodule.AppModuleBasic{},
		edgeaimodule.AppModuleBasic{},
		dexmodule.AppModuleBasic{},
		referralmodule.AppModuleBasic{},
		liquidstakingmodule.AppModuleBasic{},
		// this line is used by starport scaffolding # stargate/app/moduleBasic
	)

	// module account permissions
	maccPerms = map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		icatypes.ModuleName:            nil,
		// MINT-1 / R1 铸币铁律：mint 模块账户不得持有 Minter 权限。
		// 全链唯一合法铸币入口是 tokenomics（创世一次性铸 TotalSupplyCap）。
		// 配合 ZeroInflationCalculationFn，mint 每区块产出恒为 0 币，
		// keeper.MintCoins 在 Empty() 处短路，永不调用 bank.MintCoins。
		minttypes.ModuleName:           nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
		// Q7：depin 移除 Minter（不再自铸），仅保留 Burner/Staking；统一由 tokenomics 持有 Minter。
		depinmoduletypes.ModuleName:      {authtypes.Burner, authtypes.Staking},
		tokenomicsmoduletypes.ModuleName: {authtypes.Minter},
		// 五池模型（设备激励 55% / 质押安全 15% / 团队 12% / 基金会 13% / 早期开发 5%）：
		// 设备激励池托管于 depin 模块账户（已注册），团队为多签 vesting 账户（非模块账户）；
		// 其余三池为独立模块账户，仅持有资金、无特殊权限。
		tokenomicsmoduletypes.StakingSecurityPoolName: nil,
		tokenomicsmoduletypes.FoundationPoolName:      nil,
		tokenomicsmoduletypes.EarlyDevPoolName:        nil,
		// Protocol treasury (6th address): genesis starts at zero; funded only by
		// enterprise settlement fee回流 and drip-pool A exhaustion renewal.
		// Spent via governance multisig + timelock.
		tokenomicsmoduletypes.ProtocolTreasuryPoolName: nil,
		// 需求方付费（escrow）：edgeai 模块账户托管 creator 托管的 reward，
		// 结算时经 bank 向其拨付 submitter；仅需注册为模块账户，无 Minter/Burner 权限。
		edgeaimoduletypes.ModuleName: nil,
		// DEX 保留 Minter/Burner 仅用于铸造/销毁 LP 份额代币（poolN denom，非 MC、不计入
		// 1B 上限）；MC(umc) 的创世初始流动性由 tokenomics 从基金会 T0 转账预拨，DEX 绝不新铸 MC
		// （白皮书 §24 / 零通胀硬约束）。
		dexmoduletypes.ModuleName:                  {authtypes.Minter, authtypes.Burner},
		referralmoduletypes.ModuleName:             nil, // ecosystem pool rewards are paid via bank
		referralmoduletypes.EcosystemModuleAccount: nil, // ecosystem pool for referral rewards
		// x/liquidstaking holds pooled MC, delegates it to validators and mints the
		// ulmc receipt token. Minter/Burner are scoped to ulmc only: ulmc is a
		// derivative claim on already-bonded MC (same precedent as DEX LP shares),
		// never MC itself, and never counts toward the 1B umc hard cap. Staking is
		// required so the module account can delegate and undelegate.
		liquidstakingmoduletypes.ModuleName: {authtypes.Minter, authtypes.Burner, authtypes.Staking},
		// CosmWasm 合约层：模块账户仅需 Burner（合约存入/销毁；不参与 MC 铸造）。
		wasmtypes.ModuleName: {authtypes.Burner},
		// this line is used by starport scaffolding # stargate/app/maccPerms
	}
)

var (
	_ runtime.AppI            = (*App)(nil)
	_ servertypes.Application = (*App)(nil)
)

// ZeroInflationCalculationFn 是 MC 固定总量链的通胀计算函数：恒定返回 0。
//
// MINT-1 / R1 铸币铁律：MC 总量硬顶 1e15 umc（10 亿 MC），由 tokenomics 在创世
// 一次性铸造完毕，链上绝不允许二次通胀。SDK 的 x/mint 默认使用
// types.DefaultInflationCalculationFn（年化约 7%~20%，目标 13%），若沿用会绕过
// tokenomics 的 cap 直接增发，破坏总量上限。
//
// 相比「仅在 InitChainer 里把 params 清零」，此函数是编译期硬约束：
// 即便未来有人通过治理提案把 InflationMax/InflationMin 改回非零，
// BeginBlocker 拿到的 inflation 依旧是 0，AnnualProvisions 与 BlockProvision 也恒为 0。
func ZeroInflationCalculationFn(_ sdk.Context, _ minttypes.Minter, _ minttypes.Params, _ sdk.Dec) sdk.Dec {
	return sdk.ZeroDec()
}

func init() {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	DefaultNodeHome = filepath.Join(userHomeDir, "."+Name)
}

// App extends an ABCI application, but with most of its parameters exported.
// They are exported for convenience in creating helper functions, as object
// capabilities aren't needed for testing.
type App struct {
	*baseapp.BaseApp

	cdc               *codec.LegacyAmino
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry
	txConfig          client.TxConfig

	invCheckPeriod uint

	// keys to access the substores
	keys    map[string]*storetypes.KVStoreKey
	tkeys   map[string]*storetypes.TransientStoreKey
	memKeys map[string]*storetypes.MemoryStoreKey

	// keepers
	AccountKeeper         authkeeper.AccountKeeper
	AuthzKeeper           authzkeeper.Keeper
	BankKeeper            bankkeeper.Keeper
	CapabilityKeeper      *capabilitykeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	MintKeeper            mintkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	CrisisKeeper          *crisiskeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	ParamsKeeper          paramskeeper.Keeper
	IBCKeeper             *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	EvidenceKeeper        evidencekeeper.Keeper
	TransferKeeper        ibctransferkeeper.Keeper
	ICAHostKeeper         icahostkeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	GroupKeeper           groupkeeper.Keeper
	ConsensusParamsKeeper consensusparamkeeper.Keeper

	// make scoped keepers public for test purposes
	ScopedIBCKeeper      capabilitykeeper.ScopedKeeper
	ScopedTransferKeeper capabilitykeeper.ScopedKeeper
	ScopedICAHostKeeper  capabilitykeeper.ScopedKeeper
	ScopedWasmKeeper     capabilitykeeper.ScopedKeeper

	WasmKeeper wasmkeeper.Keeper

	McchainKeeper mcchainmodulekeeper.Keeper

	DepinKeeper depinmodulekeeper.Keeper

	TokenomicsKeeper tokenomicsmodulekeeper.Keeper

	PhonenodeKeeper phonenodemodulekeeper.Keeper

	EdgeaiKeeper   edgeaimodulekeeper.Keeper
	DexKeeper      dexmodulekeeper.Keeper
	ReferralKeeper referralmodulekeeper.Keeper

	LiquidStakingKeeper liquidstakingmodulekeeper.Keeper
	// this line is used by starport scaffolding # stargate/app/keeperDeclaration

	// mm is the module manager
	mm *module.Manager

	// sm is the simulation manager
	sm           *module.SimulationManager
	configurator module.Configurator
}

// New returns a reference to an initialized blockchain app
func New(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	skipUpgradeHeights map[int64]bool,
	homePath string,
	invCheckPeriod uint,
	encodingConfig appparams.EncodingConfig,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *App {
	appCodec := encodingConfig.Marshaler
	cdc := encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry
	txConfig := encodingConfig.TxConfig

	bApp := baseapp.NewBaseApp(
		Name,
		logger,
		db,
		encodingConfig.TxConfig.TxDecoder(),
		baseAppOptions...,
	)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	keys := sdk.NewKVStoreKeys(
		authtypes.StoreKey, authz.ModuleName, banktypes.StoreKey, stakingtypes.StoreKey,
		crisistypes.StoreKey, minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, ibcexported.StoreKey, upgradetypes.StoreKey,
		feegrant.StoreKey, evidencetypes.StoreKey, ibctransfertypes.StoreKey, icahosttypes.StoreKey,
		capabilitytypes.StoreKey, group.StoreKey, icacontrollertypes.StoreKey, consensusparamtypes.StoreKey,
		mcchainmoduletypes.StoreKey,
		tokenomicsmoduletypes.StoreKey,
		depinmoduletypes.StoreKey,
		phonenodemoduletypes.StoreKey,
		edgeaimoduletypes.StoreKey,
		dexmoduletypes.StoreKey,
		referralmoduletypes.StoreKey,
		liquidstakingmoduletypes.StoreKey,
		wasmtypes.StoreKey,
		// this line is used by starport scaffolding # stargate/app/storeKey
	)
	tkeys := sdk.NewTransientStoreKeys(paramstypes.TStoreKey)
	memKeys := sdk.NewMemoryStoreKeys(capabilitytypes.MemStoreKey, edgeaimoduletypes.MemStoreKey)

	app := &App{
		BaseApp:           bApp,
		cdc:               cdc,
		appCodec:          appCodec,
		interfaceRegistry: interfaceRegistry,
		txConfig:          encodingConfig.TxConfig,
		invCheckPeriod:    invCheckPeriod,
		keys:              keys,
		tkeys:             tkeys,
		memKeys:           memKeys,
	}

	app.ParamsKeeper = initParamsKeeper(
		appCodec,
		cdc,
		keys[paramstypes.StoreKey],
		tkeys[paramstypes.TStoreKey],
	)

	// set the BaseApp's parameter store
	app.ConsensusParamsKeeper = consensusparamkeeper.NewKeeper(appCodec, keys[consensusparamtypes.StoreKey], authtypes.NewModuleAddress(govtypes.ModuleName).String())
	bApp.SetParamStore(&app.ConsensusParamsKeeper)

	// add capability keeper and ScopeToModule for ibc module
	app.CapabilityKeeper = capabilitykeeper.NewKeeper(
		appCodec,
		keys[capabilitytypes.StoreKey],
		memKeys[capabilitytypes.MemStoreKey],
	)

	// grant capabilities for the ibc and ibc-transfer modules
	scopedIBCKeeper := app.CapabilityKeeper.ScopeToModule(ibcexported.ModuleName)
	scopedICAControllerKeeper := app.CapabilityKeeper.ScopeToModule(icacontrollertypes.SubModuleName)
	scopedTransferKeeper := app.CapabilityKeeper.ScopeToModule(ibctransfertypes.ModuleName)
	scopedICAHostKeeper := app.CapabilityKeeper.ScopeToModule(icahosttypes.SubModuleName)
	scopedWasmKeeper := app.CapabilityKeeper.ScopeToModule(wasmtypes.ModuleName)
	// this line is used by starport scaffolding # stargate/app/scopedKeeper

	// add keepers
	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec,
		keys[authtypes.StoreKey],
		authtypes.ProtoBaseAccount,
		maccPerms,
		sdk.Bech32PrefixAccAddr,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.AuthzKeeper = authzkeeper.NewKeeper(
		keys[authz.ModuleName],
		appCodec,
		app.MsgServiceRouter(),
		app.AccountKeeper,
	)

	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		keys[banktypes.StoreKey],
		app.AccountKeeper,
		app.BlockedModuleAccountAddrs(),
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec,
		keys[stakingtypes.StoreKey],
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(
		appCodec,
		keys[feegrant.StoreKey],
		app.AccountKeeper,
	)

	app.MintKeeper = mintkeeper.NewKeeper(
		appCodec,
		keys[minttypes.StoreKey],
		app.StakingKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.DistrKeeper = distrkeeper.NewKeeper(
		appCodec,
		keys[distrtypes.StoreKey],
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		cdc,
		keys[slashingtypes.StoreKey],
		app.StakingKeeper,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app.CrisisKeeper = crisiskeeper.NewKeeper(
		appCodec,
		keys[crisistypes.StoreKey],
		invCheckPeriod,
		app.BankKeeper,
		authtypes.FeeCollectorName,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	groupConfig := group.DefaultConfig()
	/*
		Example of setting group params:
		groupConfig.MaxMetadataLen = 1000
	*/
	app.GroupKeeper = groupkeeper.NewKeeper(
		keys[group.StoreKey],
		appCodec,
		app.MsgServiceRouter(),
		app.AccountKeeper,
		groupConfig,
	)

	app.UpgradeKeeper = upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		keys[upgradetypes.StoreKey],
		appCodec,
		homePath,
		app.BaseApp,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	// 注册软件升级处理器（A3）：无处理器时，通过的 SoftwareUpgrade 治理提案会在目标高度
	// 停机且无迁移可执行，导致不可逆停链。默认处理器运行全部模块迁移。
	app.RegisterUpgradeHandlers()

	// UPG-1 修复：升级存储加载器（Store Loader）。缺少它时，任何「新增/删除模块 store」
	// 的软件升级会让全网节点在目标高度因 store 版本不匹配而崩溃且不可恢复。
	// 必须早于 baseapp 加载最新版本（LoadLatestVersion）之前设置。
	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Sprintf("upgrade: failed to read upgrade info from disk: %v", err))
	}
	if storeUpgrades, ok := StoreUpgradesByUpgradeName[upgradeInfo.Name]; ok &&
		!app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, storeUpgrades))
	}

	// ... other modules keepers

	// Create IBC Keeper
	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec, keys[ibcexported.StoreKey],
		app.GetSubspace(ibcexported.ModuleName),
		app.StakingKeeper,
		app.UpgradeKeeper,
		scopedIBCKeeper,
	)

	// Create Transfer Keepers
	app.TransferKeeper = ibctransferkeeper.NewKeeper(
		appCodec,
		keys[ibctransfertypes.StoreKey],
		app.GetSubspace(ibctransfertypes.ModuleName),
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.ChannelKeeper,
		&app.IBCKeeper.PortKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		scopedTransferKeeper,
	)
	transferModule := transfer.NewAppModule(app.TransferKeeper)
	transferIBCModule := transfer.NewIBCModule(app.TransferKeeper)

	app.ICAHostKeeper = icahostkeeper.NewKeeper(
		appCodec, keys[icahosttypes.StoreKey],
		app.GetSubspace(icahosttypes.SubModuleName),
		app.IBCKeeper.ChannelKeeper,
		app.IBCKeeper.ChannelKeeper,
		&app.IBCKeeper.PortKeeper,
		app.AccountKeeper,
		scopedICAHostKeeper,
		app.MsgServiceRouter(),
	)
	icaControllerKeeper := icacontrollerkeeper.NewKeeper(
		appCodec, keys[icacontrollertypes.StoreKey],
		app.GetSubspace(icacontrollertypes.SubModuleName),
		app.IBCKeeper.ChannelKeeper, // may be replaced with middleware such as ics29 fee
		app.IBCKeeper.ChannelKeeper, &app.IBCKeeper.PortKeeper,
		scopedICAControllerKeeper, app.MsgServiceRouter(),
	)
	icaModule := ica.NewAppModule(&icaControllerKeeper, &app.ICAHostKeeper)
	icaHostIBCModule := icahost.NewIBCModule(app.ICAHostKeeper)

	// CosmWasm keeper: smart-contract layer (x/wasm). The keeper and module are
	// only wired on CGO builds (wasmvm linked); on non-CGO builds the wasm module
	// is skipped entirely (see wasm_setup_{cgo,nocgo}.go).
	if err := setupWasmKeeper(app, homePath, appOpts, keys, scopedWasmKeeper); err != nil {
		panic(fmt.Sprintf("error setting up wasm keeper: %v", err))
	}

	// Create evidence Keeper for to register the IBC light client misbehaviour evidence route
	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec,
		keys[evidencetypes.StoreKey],
		app.StakingKeeper,
		app.SlashingKeeper,
	)
	// If evidence needs to be handled for the app, set routes in router here and seal
	app.EvidenceKeeper = *evidenceKeeper

	govConfig := govtypes.DefaultConfig()
	govKeeper := govkeeper.NewKeeper(
		appCodec,
		keys[govtypes.StoreKey],
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		app.MsgServiceRouter(),
		govConfig,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	govRouter := govv1beta1.NewRouter()
	govRouter.
		AddRoute(govtypes.RouterKey, govv1beta1.ProposalHandler).
		AddRoute(paramproposal.RouterKey, params.NewParamChangeProposalHandler(app.ParamsKeeper)).
		AddRoute(upgradetypes.RouterKey, upgrade.NewSoftwareUpgradeProposalHandler(app.UpgradeKeeper)).
		AddRoute(ibcclienttypes.RouterKey, ibcclient.NewClientProposalHandler(app.IBCKeeper.ClientKeeper))
	govKeeper.SetLegacyRouter(govRouter)

	app.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(
		// register the governance hooks
		),
	)

	app.McchainKeeper = *mcchainmodulekeeper.NewKeeper(
		appCodec,
		keys[mcchainmoduletypes.StoreKey],
		keys[mcchainmoduletypes.MemStoreKey],
		app.GetSubspace(mcchainmoduletypes.ModuleName),
	)
	mcchainModule := mcchainmodule.NewAppModule(appCodec, app.McchainKeeper, app.AccountKeeper, app.BankKeeper)

	// P2/Q5 (接线顺序铁律): PhonenodeKeeper MUST be created before DepinKeeper
	// because the depin keeper depends on the phonenode keeper (depin→phonenode
	//关联校验).
	app.PhonenodeKeeper = *phonenodemodulekeeper.NewKeeper(
		appCodec,
		keys[phonenodemoduletypes.StoreKey],
		keys[phonenodemoduletypes.MemStoreKey],
		app.GetSubspace(phonenodemoduletypes.ModuleName),
		app.BankKeeper,
		app.StakingKeeper,
		app.SlashingKeeper,
	)
	phonenodeModule := phonenodemodule.NewAppModule(appCodec, app.PhonenodeKeeper, app.AccountKeeper, app.BankKeeper)

	app.DepinKeeper = *depinmodulekeeper.NewKeeper(
		appCodec,
		keys[depinmoduletypes.StoreKey],
		keys[depinmoduletypes.MemStoreKey],
		app.GetSubspace(depinmoduletypes.ModuleName),

		app.BankKeeper,
		app.PhonenodeKeeper,
		nil, // referralKeeper：referral 在其后创建，稍后经 SetReferralKeeper 接线
	)
	depinModule := depinmodule.NewAppModule(appCodec, app.DepinKeeper, app.AccountKeeper, app.BankKeeper)

	// B3/edgeai：依赖 phonenode (IsAttested/SlashIfBad) + bank (payout)，genesis 排在 depin/phonenode 后
	app.EdgeaiKeeper = *edgeaimodulekeeper.NewKeeper(
		appCodec,
		keys[edgeaimoduletypes.StoreKey],
		keys[edgeaimoduletypes.MemStoreKey],
		app.GetSubspace(edgeaimoduletypes.ModuleName),
		app.PhonenodeKeeper,
		app.BankKeeper,
		app.DepinKeeper,
		nil, // referralKeeper：referral 在其后创建，稍后经 SetReferralKeeper 接线
	)
	edgeaiModule := edgeaimodule.NewAppModule(appCodec, app.EdgeaiKeeper, app.AccountKeeper, app.BankKeeper)

	// B1/tokenomics：唯一持 Minter 的「发行与分配总账」模块。
	// 必须在 depin 模块之前创建并注册（genesis 顺序铁律：tokenomics 先于 depin）。
	app.TokenomicsKeeper = *tokenomicsmodulekeeper.NewKeeper(
		appCodec,
		keys[tokenomicsmoduletypes.StoreKey],
		app.AccountKeeper,
		app.BankKeeper,
	)
	tokenomicsModule := tokenomicsmodule.NewAppModule(appCodec, app.TokenomicsKeeper, app.AccountKeeper, app.BankKeeper)

	// x/dex: AMM swap 模块。托管池资产于 dex 模块账户，需 Minter/Burner 权限铸造/销毁 LP token。
	// 依赖 BankKeeper、AccountKeeper（模块账户托管）。
	app.DexKeeper = *dexmodulekeeper.NewKeeper(
		appCodec,
		keys[dexmoduletypes.StoreKey],
		app.GetSubspace(dexmoduletypes.ModuleName),
		app.BankKeeper,
		app.AccountKeeper,
	)
	dexModule := dexmodule.NewAppModule(appCodec, app.DexKeeper, app.AccountKeeper, app.BankKeeper)

	// x/referral: 推荐裂变模块。
	// 依赖 BankKeeper（生态基金支付）、PhonenodeKeeper（防女巫）。
	app.ReferralKeeper = *referralmodulekeeper.NewKeeper(
		appCodec,
		keys[referralmoduletypes.StoreKey],
		app.GetSubspace(referralmoduletypes.ModuleName),
		app.BankKeeper,
		app.PhonenodeKeeper,
	)
	referralModule := referralmodule.NewAppModule(appCodec, app.ReferralKeeper, app.AccountKeeper, app.BankKeeper)

	// x/liquidstaking: 流动性质押。质押的 MC 由模块账户代为委托给验证人，
	// 用户拿到可转让的 ulmc 凭证；赎回时销毁凭证、走 staking 解绑期后提取。
	// 质押奖励由 BeginBlock 周期性复投，体现为 ulmc/umc 兑换率上升，不增发 MC。
	app.LiquidStakingKeeper = *liquidstakingmodulekeeper.NewKeeper(
		appCodec,
		keys[liquidstakingmoduletypes.StoreKey],
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		app.DistrKeeper,
	)
	liquidStakingModule := liquidstakingmodule.NewAppModule(appCodec, app.LiquidStakingKeeper, app.AccountKeeper, app.BankKeeper)

	// 后接线：referral 创建在 depin / edgeai 之后，通过 Setter 完成跨模块 hook。
	app.DepinKeeper.SetReferralKeeper(app.ReferralKeeper)
	app.EdgeaiKeeper.SetReferralKeeper(app.ReferralKeeper)

	// this line is used by starport scaffolding # stargate/app/keeperDefinition

	/**** IBC Routing ****/

	// Sealing prevents other modules from creating scoped sub-keepers
	app.CapabilityKeeper.Seal()

	// Create static IBC router, add transfer route, then set and seal it
	ibcRouter := ibcporttypes.NewRouter()
	ibcRouter.AddRoute(icahosttypes.SubModuleName, icaHostIBCModule).
		AddRoute(ibctransfertypes.ModuleName, transferIBCModule)
	// this line is used by starport scaffolding # ibc/app/router
	app.IBCKeeper.SetRouter(ibcRouter)

	/**** Module Hooks ****/

	// register hooks after all modules have been initialized

	app.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(
			// insert staking hooks receivers here
			app.DistrKeeper.Hooks(),
			app.SlashingKeeper.Hooks(),
			// 流动性质押必须感知罚没：验证人被罚后，模块代持的委托同比例缩水。
			// 不接这个 hook，ulmc/umc 兑换率会停留在罚没前的高位，先赎回的人
			// 按虚高汇率提走本不存在的本金，最后持有者承担全部损失（挤兑）。
			app.LiquidStakingKeeper.Hooks(),
		),
	)

	/**** Module Options ****/

	// NOTE: we may consider parsing `appOpts` inside module constructors. For the moment
	// we prefer to be more strict in what arguments the modules expect.
	skipGenesisInvariants := cast.ToBool(appOpts.Get(crisis.FlagSkipGenesisInvariants))

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.

	app.mm = module.NewManager(
		genutil.NewAppModule(
			app.AccountKeeper,
			app.StakingKeeper,
			app.BaseApp.DeliverTx,
			encodingConfig.TxConfig,
		),
		auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
		authzmodule.NewAppModule(appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, app.GetSubspace(banktypes.ModuleName)),
		capability.NewAppModule(appCodec, *app.CapabilityKeeper, false),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		groupmodule.NewAppModule(appCodec, app.GroupKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(govtypes.ModuleName)),
		// MINT-1：传入硬编码零通胀计算函数（传 nil 会退回 SDK 默认 ~13% 通胀）。
		mint.NewAppModule(appCodec, app.MintKeeper, app.AccountKeeper, ZeroInflationCalculationFn, app.GetSubspace(minttypes.ModuleName)),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(slashingtypes.ModuleName)),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, app.GetSubspace(distrtypes.ModuleName)),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName)),
		upgrade.NewAppModule(app.UpgradeKeeper),
		evidence.NewAppModule(app.EvidenceKeeper),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		ibc.NewAppModule(app.IBCKeeper),
		params.NewAppModule(app.ParamsKeeper),
		transferModule,
		icaModule,
		mcchainModule,
		tokenomicsModule,
		depinModule,
		phonenodeModule,
		edgeaiModule,
		dexModule,
		referralModule,
		liquidStakingModule,
		// this line is used by starport scaffolding # stargate/app/appModule

		crisis.NewAppModule(app.CrisisKeeper, skipGenesisInvariants, app.GetSubspace(crisistypes.ModuleName)), // always be last to make sure that it checks for all invariants and not only part of them
	)
	// CosmWasm 模块仅在 CGO 构建注册（wasmvm 可链接）；非 CGO 构建跳过，
	// ModuleManager 的顺序表中对应条目会被存在性检查安全忽略。
	if wm := wasmAppModule(app); wm != nil {
		app.mm.Modules[wm.Name()] = wm
	}

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	// NOTE: staking module is required if HistoricalEntries param > 0
	app.mm.SetOrderBeginBlockers(
		// upgrades should be run first
		upgradetypes.ModuleName,
		capabilitytypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		govtypes.ModuleName,
		crisistypes.ModuleName,
		dexmoduletypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibcexported.ModuleName,
		icatypes.ModuleName,
		wasmtypes.ModuleName,
		// MINT-1：mint 必须保留在 BeginBlockers 中——SDK 的 assertNoForgottenModules
		// 要求「所有已注册模块」都出现在顺序表里，遗漏会在 New() 阶段 panic。
		// 零通胀由两道物理约束保证，而非靠从顺序表里删除：
		//   ① mint.NewAppModule 传入 ZeroInflationCalculationFn（硬编码返回 0，
		//      治理改 params 也无法产生通胀）；
		//   ② maccPerms 中 mint 模块账户已剥夺 authtypes.Minter 权限。
		// BeginBlocker 计算出 0 币后，keeper.MintCoins 因 newCoins.Empty() 直接短路返回，
		// 永不触达 bank.MintCoins，故剥权不会导致 panic。
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		group.ModuleName,
		paramstypes.ModuleName,
		vestingtypes.ModuleName,
		consensusparamtypes.ModuleName,
		mcchainmoduletypes.ModuleName,
		tokenomicsmoduletypes.ModuleName,
		depinmoduletypes.ModuleName,
		phonenodemoduletypes.ModuleName,
		edgeaimoduletypes.ModuleName,
		referralmoduletypes.ModuleName,
		liquidstakingmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/beginBlockers
	)

	app.mm.SetOrderEndBlockers(
		crisistypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibcexported.ModuleName,
		icatypes.ModuleName,
		wasmtypes.ModuleName,
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		evidencetypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		group.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		consensusparamtypes.ModuleName,
		mcchainmoduletypes.ModuleName,
		tokenomicsmoduletypes.ModuleName,
		depinmoduletypes.ModuleName,
		phonenodemoduletypes.ModuleName,
		edgeaimoduletypes.ModuleName,
		dexmoduletypes.ModuleName,
		referralmoduletypes.ModuleName,
		liquidstakingmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/endBlockers
	)

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// NOTE: Capability module must occur first so that it can initialize any capabilities
	// so that other modules that want to create or claim capabilities afterwards in InitChain
	// can do so safely.
	genesisModuleOrder := []string{
		capabilitytypes.ModuleName,
		authtypes.ModuleName,
		banktypes.ModuleName,
		referralmoduletypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		minttypes.ModuleName,
		crisistypes.ModuleName,
		// tokenomics 必须在 genutil 之前：genutil 处理 gentx 的自抵押时需要验证人
		// 账户已有余额，而团队多签(TeamAddress)的余额由 tokenomics 在创世一次性铸造并拨付。
		// 若 tokenomics 晚于 genutil，则团队多签作创世验证人时会因余额不足而 InitChain panic。
		// 同时必须仍在 depin 之前（genesis 顺序铁律）。
		tokenomicsmoduletypes.ModuleName,
		genutiltypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibcexported.ModuleName,
		icatypes.ModuleName,
		wasmtypes.ModuleName,
		evidencetypes.ModuleName,
		authz.ModuleName,
		feegrant.ModuleName,
		group.ModuleName,
		paramstypes.ModuleName,
		upgradetypes.ModuleName,
		vestingtypes.ModuleName,
		consensusparamtypes.ModuleName,
		mcchainmoduletypes.ModuleName,
		depinmoduletypes.ModuleName,
		phonenodemoduletypes.ModuleName,
		edgeaimoduletypes.ModuleName,
		dexmoduletypes.ModuleName,
		liquidstakingmoduletypes.ModuleName,
		// this line is used by starport scaffolding # stargate/app/initGenesis
	}
	// On non-CGO builds the wasm module is never registered (wasmvm cannot be
	// linked), so it must be dropped from every ordering table as well.
	// ExportGenesisForModules performs a hard existence check against the
	// export order and panics on an unknown module, which would make state
	// export impossible on non-CGO binaries.
	if _, ok := app.mm.Modules[wasmtypes.ModuleName]; !ok {
		genesisModuleOrder = withoutModule(genesisModuleOrder, wasmtypes.ModuleName)
		app.mm.OrderBeginBlockers = withoutModule(app.mm.OrderBeginBlockers, wasmtypes.ModuleName)
		app.mm.OrderEndBlockers = withoutModule(app.mm.OrderEndBlockers, wasmtypes.ModuleName)
	}

	app.mm.SetOrderInitGenesis(genesisModuleOrder...)
	app.mm.SetOrderExportGenesis(genesisModuleOrder...)

	// A3 创世顺序铁律断言：tokenomics → depin → phonenode → edgeai。
	// 顺序错误在 InitChain 之前 fail-fast 并给出可读错误，避免静默 InitChain panic。
	{
		idx := make(map[string]int, len(genesisModuleOrder))
		for i, name := range genesisModuleOrder {
			idx[name] = i
		}
		order := []string{
			tokenomicsmoduletypes.ModuleName,
			depinmoduletypes.ModuleName,
			phonenodemoduletypes.ModuleName,
			edgeaimoduletypes.ModuleName,
		}
		for i := 1; i < len(order); i++ {
			a, b := order[i-1], order[i]
			ia, oka := idx[a]
			ib, okb := idx[b]
			if !oka {
				panic(fmt.Sprintf("mcchain: required genesis module %q missing from init genesis order", a))
			}
			if !okb {
				panic(fmt.Sprintf("mcchain: required genesis module %q missing from init genesis order", b))
			}
			if ia >= ib {
				panic(fmt.Sprintf("mcchain: genesis order violation: %q must be initialized before %q", a, b))
			}
		}
	}

	// Uncomment if you want to set a custom migration order here.
	// app.mm.SetOrderMigrations(custom order)

	app.mm.RegisterInvariants(app.CrisisKeeper)
	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	app.mm.RegisterServices(app.configurator)

	autocliv1.RegisterQueryServer(app.GRPCQueryRouter(), runtimeservices.NewAutoCLIQueryService(app.mm.Modules))
	reflectionSvc, err := runtimeservices.NewReflectionService()
	if err != nil {
		panic(err)
	}
	reflectionv1.RegisterReflectionServiceServer(app.GRPCQueryRouter(), reflectionSvc)

	// create the simulation manager and define the order of the modules for deterministic simulations
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, app.GetSubspace(authtypes.ModuleName)),
		// Validator admission on MobileChain enforces a 30k MC minimum
		// self-delegation (see MinSelfDelegationDecorator). The stock SDK
		// simulation always proposes a self-delegation floor of 1, which the
		// ante handler rejects, so staking would never be exercised.
		stakingtypes.ModuleName: newMCStakingSimModule(
			staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, app.GetSubspace(stakingtypes.ModuleName)),
			app.AccountKeeper,
			app.BankKeeper,
			app.StakingKeeper,
		),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.mm.Modules, overrideModules)
	app.sm.RegisterStoreDecoders()

	// initialize stores
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	app.MountMemoryStores(memKeys)

	// initialize BaseApp
	anteHandler, err := ante.NewAnteHandler(
		ante.HandlerOptions{
			AccountKeeper:   app.AccountKeeper,
			BankKeeper:      app.BankKeeper,
			SignModeHandler: encodingConfig.TxConfig.SignModeHandler(),
			FeegrantKeeper:  app.FeeGrantKeeper,
			SigGasConsumer:  ante.DefaultSigVerificationGasConsumer,
		},
	)
	if err != nil {
		panic(fmt.Errorf("failed to create AnteHandler: %w", err))
	}

	// P0/Q1: wrap the default ante handler with a chain-wide minimum self
	// delegation decorator. Every (non-genesis) validator must self-delegate at
	// least MinSelfDelegationLowerBound (3e10 umc == 30k MC). Genesis validators
	// are enforced separately in InitChainer because they bypass the ante chain.
	msd := MinSelfDelegationDecorator{}
	app.SetAnteHandler(func(ctx sdk.Context, tx sdk.Tx, sim bool) (sdk.Context, error) {
		return msd.AnteHandle(ctx, tx, sim, anteHandler)
	})
	app.SetInitChainer(app.InitChainer)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			tmos.Exit(err.Error())
		}
	}

	app.ScopedIBCKeeper = scopedIBCKeeper
	app.ScopedTransferKeeper = scopedTransferKeeper
	// this line is used by starport scaffolding # stargate/app/beforeInitReturn

	// T2 生产预言机强制（P0③）：主网/生产必须启用 TeeOracle 做链上真实验签，
	// 且 MC_ORACLE_PUBKEY 必须设置（= 33 字节压缩 secp256k1 公钥的 base64）；
	// 未设置则直接 panic，杜绝「默认 SoftOracle 静默放行任意 attestation」的隐患
	// （SoftOracle 仅校验 challenge+signature 非空，等于来者不拒）。
	// 仅本地单节点开发/测试可通过显式设置 MC_ORACLE_ALLOW_SOFT=1 退回 SoftOracle，并大声告警。
	// 链下签名服务见 cmd/oracle（P0② 已强制：签名前先验证真实设备硬件 attestation）；公钥由运营离线保管。
	envPub := os.Getenv("MC_ORACLE_PUBKEY")
	if envPub == "" {
		if os.Getenv("MC_ORACLE_ALLOW_SOFT") == "1" {
			fmt.Fprintf(os.Stderr, "[WARN] MC_ORACLE_PUBKEY 未设置，已按 MC_ORACLE_ALLOW_SOFT=1 退回 SoftOracle（仅限本地开发，生产必须启用 TeeOracle）\n")
		} else {
			panic(fmt.Errorf("MC_ORACLE_PUBKEY must be set for production: a 33-byte compressed secp256k1 pubkey (base64); set MC_ORACLE_ALLOW_SOFT=1 only for local single-node dev"))
		}
	} else {
		bz, berr := base64.StdEncoding.DecodeString(envPub)
		if berr != nil || len(bz) != 33 {
			panic(fmt.Errorf("MC_ORACLE_PUBKEY must be base64 of 33-byte compressed secp256k1 pubkey: %w", berr))
		}
		depinmoduletypes.SetOracle(depinmoduletypes.NewTeeOracle(depinmoduletypes.NewSecp256k1PubKey(bz)))
	}

	return app
}

// withoutModule returns a copy of order with every occurrence of name removed.
// It is used to strip conditionally compiled modules (currently x/wasm) from
// the module manager ordering tables.
func withoutModule(order []string, name string) []string {
	filtered := make([]string, 0, len(order))
	for _, moduleName := range order {
		if moduleName == name {
			continue
		}
		filtered = append(filtered, moduleName)
	}
	return filtered
}

// Name returns the name of the App
func (app *App) Name() string { return app.BaseApp.Name() }

// BeginBlocker application updates every begin block
func (app *App) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	return app.mm.BeginBlock(ctx, req)
}

// EndBlocker application updates every end block
func (app *App) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	return app.mm.EndBlock(ctx, req)
}

// InitChainer application update at chain initialization
func (app *App) InitChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	var genesisState GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}
	app.UpgradeKeeper.SetModuleVersionMap(ctx, app.mm.GetVersionMap())
	res := app.mm.InitGenesis(ctx, app.appCodec, genesisState)

	// ---- P0: post-genesis overrides (must run AFTER InitGenesis) ----

	// Q8: force the staking BondDenom to "umc" regardless of what the genesis
	// file may contain, so it stays consistent with config.yml accounts.
	p := app.StakingKeeper.GetParams(ctx)
	p.BondDenom = "umc"
	app.StakingKeeper.SetParams(ctx, p)

	// P0/R1: 固定总量链——tokenomics 模块一次性铸造并强约束 cap(1e15 umc)，
	// 链上绝不允许二次通胀。mint 模块默认 inflation≈13%，若持有 Minter 权限
	// 便可绕过 tokenomics 的 cap 直接铸币，故此处在创世把通胀参数清零。
	//
	// MINT-1 更正：InitChainer 仅在**创世**执行一次，并非"每次启动兜底"
	// （旧注释如此描述，是错误的）。参数清零因此只是第一道防线——真正的
	// 硬约束是 maccPerms 中 mint 模块已**不再持有 Minter 权限**
	// （见本文件 `minttypes.ModuleName: nil`）：即便日后治理把 InflationMax
	// 改回非零，mint.BeginBlock 的 MintCoins 也会因缺少 Minter 权限而失败，
	// 10 亿 MC 硬顶不会被击穿。
	//
	// 注意：GoalBonded 绝不能为 0——mint.BeginBlock 会算 bondedRatio/GoalBonded，
	// 归零将在首区块除零 panic 导致链 halt。仅清零通胀上下限与本区块通胀率。
	mp := app.MintKeeper.GetParams(ctx)
	mp.InflationRateChange = sdk.ZeroDec()
	mp.InflationMax = sdk.ZeroDec()
	mp.InflationMin = sdk.ZeroDec()
	app.MintKeeper.SetParams(ctx, mp)
	minter := app.MintKeeper.GetMinter(ctx)
	minter.Inflation = sdk.ZeroDec()
	minter.AnnualProvisions = sdk.ZeroDec()
	app.MintKeeper.SetMinter(ctx, minter)

	// P0/WASM-1：将 CosmWasm 合约上传/实例化权限收敛到治理账户，禁止任意地址
	// 随意上传/实例化合约（默认 AllowEverybody 是上线阻断级风险：任何人可部署恶意合约）。
	// 仅在 wasm 模块实际注册时（CGO 构建）执行；非 CGO 构建未注册 wasm 模块，跳过。
	if _, ok := app.mm.Modules[wasmtypes.ModuleName]; ok {
		wp := app.WasmKeeper.GetParams(ctx)
		govAddr := authtypes.NewModuleAddress(govtypes.ModuleName)
		// CodeUploadAccess：仅治理模块账户可上传合约字节码。
		wp.CodeUploadAccess = wasmtypes.AccessTypeAnyOfAddresses.With(govAddr)
		// InstantiateDefaultPermission：wasmd 以 `perm.With(creator)` 派生每份代码的
		// 默认实例化权限。设为 AnyOfAddresses 即「仅上传者本人可实例化」；
		// 由于上传者只能是治理账户，实例化同样收敛到治理。
		wp.InstantiateDefaultPermission = wasmtypes.AccessTypeAnyOfAddresses
		if err := app.WasmKeeper.SetParams(ctx, wp); err != nil {
			panic(fmt.Sprintf("wasm: failed to restrict upload/instantiate to gov: %v", err))
		}
	}

	// Q1/C: genesis validators are created by InitGenesis and therefore bypass
	// the ante decorator. Lift any validator whose MinSelfDelegation is below
	// the chain-wide floor up to that floor, so the same acceptance rule holds
	// for genesis validators as for every later MsgCreateValidator.
	//
	// L-3 更正：旧注释写的下限是 100000000000（10 万 MC），与实际常量不符。
	// 唯一权威值是 MinSelfDelegationLowerBound = 30_000_000_000 umc（3 万 MC），
	// 定义于 app/ante.go，白皮书 §A.6 同值。此处不再复述字面量，直接引用常量，
	// 避免注释与代码二次漂移。
	for _, v := range app.StakingKeeper.GetAllValidators(ctx) {
		if v.MinSelfDelegation.LT(sdk.NewInt(MinSelfDelegationLowerBound)) {
			v.MinSelfDelegation = sdk.NewInt(MinSelfDelegationLowerBound)
			app.StakingKeeper.SetValidator(ctx, v)
		}
	}

	return res
}

// Configurator get app configurator
func (app *App) Configurator() module.Configurator {
	return app.configurator
}

// UpgradeName 是首个计划内软件升级的规范升级名。
// 未来每次升级都必须用同一机制（RegisterUpgradeHandlers）注册对应处理器。
const UpgradeName = "mcchain-v1"

// StoreUpgradesByUpgradeName 按升级名登记该次升级涉及的 KVStore 增删改。
// UPG-1：SDK 的 store 版本在升级时若与 app 注册的 store key 集合不一致，
// 节点会在目标高度 panic 且无法恢复。凡是「新增/删除/重命名模块 store」的升级，
// 都必须在此登记，NewApp 会据此安装 upgradetypes.UpgradeStoreLoader。
// mcchain-v1 为首个升级，不涉及 store 变更，故为空集合（登记本身即启用加载器路径）。
var StoreUpgradesByUpgradeName = map[string]*storetypes.StoreUpgrades{
	UpgradeName: {},
}

// RegisterUpgradeHandlers 注册计划内软件升级的处理器。
// 修复（A3）：此前全仓无任何 SetUpgradeHandler，一旦 SoftwareUpgrade 治理提案通过，
// 链在目标高度停机却无处理器可执迁移，导致不可逆停链。默认处理器运行全部模块迁移，
// 使链在升级时可安全执行模块版本迁移而非永久 halt。
func (app *App) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName,
		func(ctx sdk.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			return app.mm.RunMigrations(ctx, app.configurator, fromVM)
		},
	)
}

// LoadHeight loads a particular height
func (app *App) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// ModuleAccountAddrs returns all the app's module account addresses.
func (app *App) ModuleAccountAddrs() map[string]bool {
	modAccAddrs := make(map[string]bool)
	for acc := range maccPerms {
		modAccAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	return modAccAddrs
}

// BlockedModuleAccountAddrs returns all the app's blocked module account
// addresses.
func (app *App) BlockedModuleAccountAddrs() map[string]bool {
	modAccAddrs := app.ModuleAccountAddrs()
	delete(modAccAddrs, authtypes.NewModuleAddress(govtypes.ModuleName).String())
	// x/liquidstaking delegates on behalf of users, so x/distribution must be able
	// to pay its staking rewards to the module account. Keeping it in the blocked
	// set would make reward withdrawal fail.
	delete(modAccAddrs, authtypes.NewModuleAddress(liquidstakingmoduletypes.ModuleName).String())

	return modAccAddrs
}

// LegacyAmino returns SimApp's amino codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *App) LegacyAmino() *codec.LegacyAmino {
	return app.cdc
}

// AppCodec returns an app codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *App) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns an InterfaceRegistry
func (app *App) InterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

// TxConfig returns SimApp's TxConfig
func (app *App) TxConfig() client.TxConfig {
	return app.txConfig
}

// GetKey returns the KVStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *App) GetKey(storeKey string) *storetypes.KVStoreKey {
	return app.keys[storeKey]
}

// GetTKey returns the TransientStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *App) GetTKey(storeKey string) *storetypes.TransientStoreKey {
	return app.tkeys[storeKey]
}

// GetMemKey returns the MemStoreKey for the provided mem key.
//
// NOTE: This is solely used for testing purposes.
func (app *App) GetMemKey(storeKey string) *storetypes.MemoryStoreKey {
	return app.memKeys[storeKey]
}

// GetSubspace returns a param subspace for a given module name.
//
// NOTE: This is solely to be used for testing purposes.
func (app *App) GetSubspace(moduleName string) paramstypes.Subspace {
	subspace, _ := app.ParamsKeeper.GetSubspace(moduleName)
	return subspace
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *App) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	// Register new tx routes from grpc-gateway.
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register new tendermint queries routes from grpc-gateway.
	tmservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)
	// Register node gRPC service for grpc-gateway.
	nodeservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register grpc-gateway routes for all modules.
	ModuleBasics.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register app's OpenAPI routes.
	docs.RegisterOpenAPIService(Name, apiSvr.Router)
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *App) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.BaseApp.GRPCQueryRouter(), clientCtx, app.BaseApp.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *App) RegisterTendermintService(clientCtx client.Context) {
	tmservice.RegisterTendermintService(
		clientCtx,
		app.BaseApp.GRPCQueryRouter(),
		app.interfaceRegistry,
		app.Query,
	)
}

// RegisterNodeService implements the Application.RegisterNodeService method.
func (app *App) RegisterNodeService(clientCtx client.Context) {
	nodeservice.RegisterNodeService(clientCtx, app.GRPCQueryRouter())
}

// initParamsKeeper init params keeper and its subspaces
func initParamsKeeper(appCodec codec.BinaryCodec, legacyAmino *codec.LegacyAmino, key, tkey storetypes.StoreKey) paramskeeper.Keeper {
	paramsKeeper := paramskeeper.NewKeeper(appCodec, legacyAmino, key, tkey)

	paramsKeeper.Subspace(authtypes.ModuleName)
	paramsKeeper.Subspace(banktypes.ModuleName)
	paramsKeeper.Subspace(stakingtypes.ModuleName)
	paramsKeeper.Subspace(minttypes.ModuleName)
	paramsKeeper.Subspace(distrtypes.ModuleName)
	paramsKeeper.Subspace(slashingtypes.ModuleName)
	paramsKeeper.Subspace(govtypes.ModuleName).WithKeyTable(govv1.ParamKeyTable()) //nolint:staticcheck
	paramsKeeper.Subspace(crisistypes.ModuleName)
	paramsKeeper.Subspace(ibctransfertypes.ModuleName)
	paramsKeeper.Subspace(ibcexported.ModuleName)
	paramsKeeper.Subspace(icacontrollertypes.SubModuleName)
	paramsKeeper.Subspace(icahosttypes.SubModuleName)
	paramsKeeper.Subspace(mcchainmoduletypes.ModuleName)
	paramsKeeper.Subspace(depinmoduletypes.ModuleName)
	paramsKeeper.Subspace(phonenodemoduletypes.ModuleName)
	paramsKeeper.Subspace(edgeaimoduletypes.ModuleName)
	paramsKeeper.Subspace(dexmoduletypes.ModuleName)
	paramsKeeper.Subspace(referralmoduletypes.ModuleName)
	paramsKeeper.Subspace(liquidstakingmoduletypes.ModuleName)
	paramsKeeper.Subspace(wasmtypes.ModuleName)
	// this line is used by starport scaffolding # stargate/app/paramSubspace

	return paramsKeeper
}

// SimulationManager returns the app SimulationManager
func (app *App) SimulationManager() *module.SimulationManager {
	return app.sm
}

// ModuleManager returns the app ModuleManager
func (app *App) ModuleManager() *module.Manager {
	return app.mm
}
