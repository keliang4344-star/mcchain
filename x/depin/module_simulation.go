package depin

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	depinsimulation "mcchain/x/depin/simulation"
	"mcchain/x/depin/types"
)

var _ module.AppModuleSimulation = AppModule{}

const (
	opWeightMsgRegisterDevice     = "op_weight_msg_register_device"
	defaultWeightMsgRegisterDevice = 100

	opWeightMsgAttestDevice     = "op_weight_msg_attest_device"
	defaultWeightMsgAttestDevice = 100

	opWeightMsgSubmitContribution     = "op_weight_msg_submit_contribution"
	defaultWeightMsgSubmitContribution = 100
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	depinGenesis := types.DefaultGenesis()
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(depinGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry) {}

// WeightedOperations returns the depin module operations with their weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgRegisterDevice int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgRegisterDevice, &weightMsgRegisterDevice, nil,
		func(_ *rand.Rand) { weightMsgRegisterDevice = defaultWeightMsgRegisterDevice })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgRegisterDevice,
		depinsimulation.SimulateMsgRegisterDevice(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgAttestDevice int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgAttestDevice, &weightMsgAttestDevice, nil,
		func(_ *rand.Rand) { weightMsgAttestDevice = defaultWeightMsgAttestDevice })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgAttestDevice,
		depinsimulation.SimulateMsgAttestDevice(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgSubmitContribution int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgSubmitContribution, &weightMsgSubmitContribution, nil,
		func(_ *rand.Rand) { weightMsgSubmitContribution = defaultWeightMsgSubmitContribution })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgSubmitContribution,
		depinsimulation.SimulateMsgSubmitContribution(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
