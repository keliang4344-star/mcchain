package phonenode

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	"mcchain/testutil/sample"
	phonenodesimulation "mcchain/x/phonenode/simulation"
	"mcchain/x/phonenode/types"
)

// avoid unused import issue
var (
	_ = sample.AccAddress
	_ = phonenodesimulation.FindAccount
	_ = simulation.MsgEntryKind
	_ = baseapp.Paramspace
	_ = rand.Rand{}
)

const (
	opWeightMsgSubmitStateProof = "op_weight_msg_submit_state_proof"
	// TODO: Determine the simulation weight value
	defaultWeightMsgSubmitStateProof int = 100

	opWeightMsgRegisterNode = "op_weight_msg_register_node"
	// TODO: Determine the simulation weight value
	defaultWeightMsgRegisterNode int = 100

	// this line is used by starport scaffolding # simapp/module/const
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	phonenodeGenesis := types.GenesisState{
		Params: types.DefaultParams(),
		// this line is used by starport scaffolding # simapp/module/genesisState
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&phonenodeGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry) {}

// ProposalContents doesn't return any content functions for governance proposals.
func (AppModule) ProposalContents(_ module.SimulationState) []simtypes.WeightedProposalContent {
	return nil
}

// WeightedOperations returns the all the gov module operations with their respective weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgSubmitStateProof int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgSubmitStateProof, &weightMsgSubmitStateProof, nil,
		func(_ *rand.Rand) {
			weightMsgSubmitStateProof = defaultWeightMsgSubmitStateProof
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgSubmitStateProof,
		phonenodesimulation.SimulateMsgSubmitStateProof(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgRegisterNode int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgRegisterNode, &weightMsgRegisterNode, nil,
		func(_ *rand.Rand) {
			weightMsgRegisterNode = defaultWeightMsgRegisterNode
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgRegisterNode,
		phonenodesimulation.SimulateMsgRegisterNode(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	// this line is used by starport scaffolding # simapp/module/operation

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{
		simulation.NewWeightedProposalMsg(
			opWeightMsgSubmitStateProof,
			defaultWeightMsgSubmitStateProof,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				phonenodesimulation.SimulateMsgSubmitStateProof(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		simulation.NewWeightedProposalMsg(
			opWeightMsgRegisterNode,
			defaultWeightMsgRegisterNode,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				phonenodesimulation.SimulateMsgRegisterNode(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		// this line is used by starport scaffolding # simapp/module/OpMsg
	}
}
