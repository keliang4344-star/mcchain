package referral

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	sdk "github.com/cosmos/cosmos-sdk/types"
	referralsimulation "mcchain/x/referral/simulation"
	"mcchain/x/referral/types"
)

const (
	opWeightMsgCreateReferral             = "op_weight_msg_create_referral"
	defaultWeightMsgCreateReferral int    = 100

	opWeightMsgClaimReferralReward             = "op_weight_msg_claim_referral_reward"
	defaultWeightMsgClaimReferralReward int    = 100
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	referralGenesis := types.DefaultGenesis()
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(referralGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry) {}

// WeightedOperations returns the referral module operations with their weights.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	operations := make([]simtypes.WeightedOperation, 0)

	var weightMsgCreateReferral int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgCreateReferral, &weightMsgCreateReferral, nil,
		func(_ *rand.Rand) { weightMsgCreateReferral = defaultWeightMsgCreateReferral })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgCreateReferral,
		referralsimulation.SimulateMsgCreateReferral(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	var weightMsgClaimReferralReward int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsgClaimReferralReward, &weightMsgClaimReferralReward, nil,
		func(_ *rand.Rand) { weightMsgClaimReferralReward = defaultWeightMsgClaimReferralReward })
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsgClaimReferralReward,
		referralsimulation.SimulateMsgClaimReferralReward(am.accountKeeper, am.bankKeeper, am.keeper),
	))

	return operations
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(_ module.SimulationState) []simtypes.WeightedProposalMsg {
	return nil
}
