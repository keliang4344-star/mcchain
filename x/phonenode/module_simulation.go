package phonenode

import (
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	phonenodesimulation "mcchain/x/phonenode/simulation"
	"mcchain/x/phonenode/types"
)

var _ module.AppModuleSimulation = AppModule{}

const (
	opWeightMsgRegisterNode     = "op_weight_msg_register_node"
	defaultWeightMsgRegisterNode = 100

	opWeightMsgSubmitStateProof     = "op_weight_msg_submit_state_proof"
	defaultWeightMsgSubmitStateProof = 100
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	phonenodeGenesis := types.DefaultGenesis()
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(phonenodeGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry) {}

// WeightedOperations returns the phonenode module operations with their weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgRegisterNode int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgRegisterNode, &weightMsgRegisterNode, nil,
		func(_ *rand.Rand) { weightMsgRegisterNode = defaultWeightMsgRegisterNode })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgRegisterNode,
		phonenodesimulation.SimulateMsgRegisterNode(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgSubmitStateProof int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgSubmitStateProof, &weightMsgSubmitStateProof, nil,
		func(_ *rand.Rand) { weightMsgSubmitStateProof = defaultWeightMsgSubmitStateProof })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgSubmitStateProof,
		phonenodesimulation.SimulateMsgSubmitStateProof(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
