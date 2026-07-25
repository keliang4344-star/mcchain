package depin

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	"mcchain/testutil/sample"
	depinsimulation "mcchain/x/depin/simulation"
	"mcchain/x/depin/types"
)

// avoid unused import issue
var (
	_ = sample.AccAddress
	_ = depinsimulation.FindAccount
	_ = simulation.MsgEntryKind
	_ = baseapp.Paramspace
	_ = rand.Rand{}
)

const (
	opWeightMsgRegisterDevice = "op_weight_msg_register_device"
	// TODO: Determine the simulation weight value
	defaultWeightMsgRegisterDevice int = 100

	opWeightMsgAttestDevice = "op_weight_msg_attest_device"
	// TODO: Determine the simulation weight value
	defaultWeightMsgAttestDevice int = 100

	opWeightMsgSubmitContribution = "op_weight_msg_submit_contribution"
	// TODO: Determine the simulation weight value
	defaultWeightMsgSubmitContribution int = 100

	// this line is used by starport scaffolding # simapp/module/const
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	depinGenesis := types.GenesisState{
		Params: types.DefaultParams(),
		// this line is used by starport scaffolding # simapp/module/genesisState
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&depinGenesis)
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

	var weightMsgRegisterDevice int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgRegisterDevice, &weightMsgRegisterDevice, nil,
		func(_ *rand.Rand) {
			weightMsgRegisterDevice = defaultWeightMsgRegisterDevice
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgRegisterDevice,
		depinsimulation.SimulateMsgRegisterDevice(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgAttestDevice int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgAttestDevice, &weightMsgAttestDevice, nil,
		func(_ *rand.Rand) {
			weightMsgAttestDevice = defaultWeightMsgAttestDevice
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgAttestDevice,
		depinsimulation.SimulateMsgAttestDevice(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgSubmitContribution int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgSubmitContribution, &weightMsgSubmitContribution, nil,
		func(_ *rand.Rand) {
			weightMsgSubmitContribution = defaultWeightMsgSubmitContribution
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgSubmitContribution,
		depinsimulation.SimulateMsgSubmitContribution(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	// this line is used by starport scaffolding # simapp/module/operation

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{
		simulation.NewWeightedProposalMsg(
			opWeightMsgRegisterDevice,
			defaultWeightMsgRegisterDevice,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				depinsimulation.SimulateMsgRegisterDevice(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		simulation.NewWeightedProposalMsg(
			opWeightMsgAttestDevice,
			defaultWeightMsgAttestDevice,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				depinsimulation.SimulateMsgAttestDevice(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		simulation.NewWeightedProposalMsg(
			opWeightMsgSubmitContribution,
			defaultWeightMsgSubmitContribution,
			func(r *rand.Rand, ctx sdk.Context, accs []simtypes.Account) sdk.Msg {
				depinsimulation.SimulateMsgSubmitContribution(am.accountKeeper, am.bankKeeper, am.keeper)
				return nil
			},
		),
		// this line is used by starport scaffolding # simapp/module/OpMsg
	}
}
