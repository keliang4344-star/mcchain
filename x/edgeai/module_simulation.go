package edgeai

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	edgeaisimulation "mcchain/x/edgeai/simulation"
	"mcchain/x/edgeai/types"
)

var _ module.AppModuleSimulation = AppModule{}

const (
	opWeightMsgCreateTask     = "op_weight_msg_create_task"
	defaultWeightMsgCreateTask = 100

	opWeightMsgSubmitResult     = "op_weight_msg_submit_result"
	defaultWeightMsgSubmitResult = 100

	opWeightMsgOpenDispute     = "op_weight_msg_open_dispute"
	defaultWeightMsgOpenDispute = 100

	opWeightMsgResolveDispute     = "op_weight_msg_resolve_dispute"
	defaultWeightMsgResolveDispute = 100
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	edgeaiGenesis := types.DefaultGenesis()
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(edgeaiGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry) {}

// WeightedOperations returns the edgeai module operations with their weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgCreateTask int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgCreateTask, &weightMsgCreateTask, nil,
		func(_ *rand.Rand) { weightMsgCreateTask = defaultWeightMsgCreateTask })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgCreateTask,
		edgeaisimulation.SimulateMsgCreateTask(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgSubmitResult int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgSubmitResult, &weightMsgSubmitResult, nil,
		func(_ *rand.Rand) { weightMsgSubmitResult = defaultWeightMsgSubmitResult })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgSubmitResult,
		edgeaisimulation.SimulateMsgSubmitResult(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgOpenDispute int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgOpenDispute, &weightMsgOpenDispute, nil,
		func(_ *rand.Rand) { weightMsgOpenDispute = defaultWeightMsgOpenDispute })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgOpenDispute,
		edgeaisimulation.SimulateMsgOpenDispute(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgResolveDispute int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgResolveDispute, &weightMsgResolveDispute, nil,
		func(_ *rand.Rand) { weightMsgResolveDispute = defaultWeightMsgResolveDispute })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgResolveDispute,
		edgeaisimulation.SimulateMsgResolveDispute(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
