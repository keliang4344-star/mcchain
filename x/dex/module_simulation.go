package dex

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	dexsimulation "mcchain/x/dex/simulation"
	"mcchain/x/dex/types"
)

const (
	opWeightMsgCreatePool             = "op_weight_msg_create_pool"
	defaultWeightMsgCreatePool int    = 100

	opWeightMsgAddLiquidity             = "op_weight_msg_add_liquidity"
	defaultWeightMsgAddLiquidity int    = 100

	opWeightMsgRemoveLiquidity             = "op_weight_msg_remove_liquidity"
	defaultWeightMsgRemoveLiquidity int    = 100

	opWeightMsgSwapExactIn             = "op_weight_msg_swap_exact_in"
	defaultWeightMsgSwapExactIn int    = 100

	opWeightMsgSubmitSettlementBatch             = "op_weight_msg_submit_settlement_batch"
	defaultWeightMsgSubmitSettlementBatch int    = 100

	opWeightMsgFinalizeSettlementBatch             = "op_weight_msg_finalize_settlement_batch"
	defaultWeightMsgFinalizeSettlementBatch int    = 100
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	dexGenesis := types.DefaultGenesis()
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(dexGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry) {}

// WeightedOperations returns the dex module operations with their weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgCreatePool int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgCreatePool, &weightMsgCreatePool, nil,
		func(_ *rand.Rand) { weightMsgCreatePool = defaultWeightMsgCreatePool })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgCreatePool,
		dexsimulation.SimulateMsgCreatePool(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgAddLiquidity int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgAddLiquidity, &weightMsgAddLiquidity, nil,
		func(_ *rand.Rand) { weightMsgAddLiquidity = defaultWeightMsgAddLiquidity })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgAddLiquidity,
		dexsimulation.SimulateMsgAddLiquidity(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgRemoveLiquidity int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgRemoveLiquidity, &weightMsgRemoveLiquidity, nil,
		func(_ *rand.Rand) { weightMsgRemoveLiquidity = defaultWeightMsgRemoveLiquidity })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgRemoveLiquidity,
		dexsimulation.SimulateMsgRemoveLiquidity(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgSwapExactIn int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgSwapExactIn, &weightMsgSwapExactIn, nil,
		func(_ *rand.Rand) { weightMsgSwapExactIn = defaultWeightMsgSwapExactIn })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgSwapExactIn,
		dexsimulation.SimulateMsgSwapExactIn(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgSubmitSettlementBatch int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgSubmitSettlementBatch, &weightMsgSubmitSettlementBatch, nil,
		func(_ *rand.Rand) { weightMsgSubmitSettlementBatch = defaultWeightMsgSubmitSettlementBatch })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgSubmitSettlementBatch,
		dexsimulation.SimulateMsgSubmitSettlementBatch(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgFinalizeSettlementBatch int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgFinalizeSettlementBatch, &weightMsgFinalizeSettlementBatch, nil,
		func(_ *rand.Rand) { weightMsgFinalizeSettlementBatch = defaultWeightMsgFinalizeSettlementBatch })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgFinalizeSettlementBatch,
		dexsimulation.SimulateMsgFinalizeSettlementBatch(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
