package liquidstaking

import (
	"encoding/json"
	"math/rand"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	sdk "github.com/cosmos/cosmos-sdk/types"
	liquidstakingtypes "mcchain/x/liquidstaking/types"
	liquidstakingsimulation "mcchain/x/liquidstaking/simulation"
)

const (
	opWeightMsgLiquidStake             = "op_weight_msg_liquid_stake"
	defaultWeightMsgLiquidStake int    = 100

	opWeightMsgLiquidUnstake             = "op_weight_msg_liquid_unstake"
	defaultWeightMsgLiquidUnstake int    = 100

	opWeightMsgClaimMatured             = "op_weight_msg_claim_matured"
	defaultWeightMsgClaimMatured int    = 100
)

// GenerateGenesisState creates a randomized GenState of the module.
// liquidstaking's GenesisState is a plain JSON struct (not a proto message),
// so it is marshaled with the standard library.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	gs := liquidstakingtypes.DefaultGenesis()
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(err)
	}
	simState.GenState[liquidstakingtypes.ModuleName] = bz
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry) {}

// WeightedOperations returns the liquidstaking module operations with weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgLiquidStake int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgLiquidStake, &weightMsgLiquidStake, nil,
		func(_ *rand.Rand) { weightMsgLiquidStake = defaultWeightMsgLiquidStake })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgLiquidStake,
		liquidstakingsimulation.SimulateMsgLiquidStake(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgLiquidUnstake int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgLiquidUnstake, &weightMsgLiquidUnstake, nil,
		func(_ *rand.Rand) { weightMsgLiquidUnstake = defaultWeightMsgLiquidUnstake })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgLiquidUnstake,
		liquidstakingsimulation.SimulateMsgLiquidUnstake(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgClaimMatured int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgClaimMatured, &weightMsgClaimMatured, nil,
		func(_ *rand.Rand) { weightMsgClaimMatured = defaultWeightMsgClaimMatured })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgClaimMatured,
		liquidstakingsimulation.SimulateMsgClaimMatured(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
