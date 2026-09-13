package app

import (
	"math/rand"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingsim "github.com/cosmos/cosmos-sdk/x/staking/simulation"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// mcStakingSimModule adapts the SDK staking simulation to MobileChain's
// validator admission policy.
//
// MinSelfDelegationDecorator rejects any MsgCreateValidator whose
// MinSelfDelegation is below MinSelfDelegationLowerBound (30k MC). The SDK
// randomized operation hard-codes a self-delegation floor of 1, so every
// simulated validator creation would be rejected by the ante handler and the
// simulation could never exercise the staking module. This wrapper keeps every
// other staking operation untouched and only swaps in a MsgCreateValidator
// generator that respects the chain-wide floor.
type mcStakingSimModule struct {
	staking.AppModule

	ak stakingtypes.AccountKeeper
	bk stakingtypes.BankKeeper
	k  *stakingkeeper.Keeper
}

var _ module.AppModuleSimulation = mcStakingSimModule{}

// newMCStakingSimModule builds the staking simulation override.
func newMCStakingSimModule(
	m staking.AppModule,
	ak stakingtypes.AccountKeeper,
	bk stakingtypes.BankKeeper,
	k *stakingkeeper.Keeper,
) mcStakingSimModule {
	return mcStakingSimModule{AppModule: m, ak: ak, bk: bk, k: k}
}

// WeightedOperations mirrors the SDK staking weights and replaces the
// create-validator operation with the MobileChain compliant variant.
func (m mcStakingSimModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	var (
		weightMsgCreateValidator           int
		weightMsgEditValidator             int
		weightMsgDelegate                  int
		weightMsgUndelegate                int
		weightMsgBeginRedelegate           int
		weightMsgCancelUnbondingDelegation int
	)

	simState.AppParams.GetOrGenerate(simState.Cdc, stakingsim.OpWeightMsgCreateValidator, &weightMsgCreateValidator, nil,
		func(_ *rand.Rand) { weightMsgCreateValidator = stakingsim.DefaultWeightMsgCreateValidator },
	)
	simState.AppParams.GetOrGenerate(simState.Cdc, stakingsim.OpWeightMsgEditValidator, &weightMsgEditValidator, nil,
		func(_ *rand.Rand) { weightMsgEditValidator = stakingsim.DefaultWeightMsgEditValidator },
	)
	simState.AppParams.GetOrGenerate(simState.Cdc, stakingsim.OpWeightMsgDelegate, &weightMsgDelegate, nil,
		func(_ *rand.Rand) { weightMsgDelegate = stakingsim.DefaultWeightMsgDelegate },
	)
	simState.AppParams.GetOrGenerate(simState.Cdc, stakingsim.OpWeightMsgUndelegate, &weightMsgUndelegate, nil,
		func(_ *rand.Rand) { weightMsgUndelegate = stakingsim.DefaultWeightMsgUndelegate },
	)
	simState.AppParams.GetOrGenerate(simState.Cdc, stakingsim.OpWeightMsgBeginRedelegate, &weightMsgBeginRedelegate, nil,
		func(_ *rand.Rand) { weightMsgBeginRedelegate = stakingsim.DefaultWeightMsgBeginRedelegate },
	)
	simState.AppParams.GetOrGenerate(simState.Cdc, stakingsim.OpWeightMsgCancelUnbondingDelegation, &weightMsgCancelUnbondingDelegation, nil,
		func(_ *rand.Rand) {
			weightMsgCancelUnbondingDelegation = stakingsim.DefaultWeightMsgCancelUnbondingDelegation
		},
	)

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(
			weightMsgCreateValidator,
			simulateMsgCreateValidatorAboveFloor(m.ak, m.bk, m.k),
		),
		simulation.NewWeightedOperation(
			weightMsgEditValidator,
			stakingsim.SimulateMsgEditValidator(m.ak, m.bk, m.k),
		),
		simulation.NewWeightedOperation(
			weightMsgDelegate,
			stakingsim.SimulateMsgDelegate(m.ak, m.bk, m.k),
		),
		simulation.NewWeightedOperation(
			weightMsgUndelegate,
			stakingsim.SimulateMsgUndelegate(m.ak, m.bk, m.k),
		),
		simulation.NewWeightedOperation(
			weightMsgBeginRedelegate,
			stakingsim.SimulateMsgBeginRedelegate(m.ak, m.bk, m.k),
		),
		simulation.NewWeightedOperation(
			weightMsgCancelUnbondingDelegation,
			stakingsim.SimulateMsgCancelUnbondingDelegate(m.ak, m.bk, m.k),
		),
	}
}

// simulateMsgCreateValidatorAboveFloor generates a MsgCreateValidator whose
// MinSelfDelegation equals the chain-wide floor and whose self-delegation is at
// least that floor, so the message survives MinSelfDelegationDecorator and the
// staking keeper's own self-delegation check.
func simulateMsgCreateValidatorAboveFloor(
	ak stakingtypes.AccountKeeper,
	bk stakingtypes.BankKeeper,
	k *stakingkeeper.Keeper,
) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		address := sdk.ValAddress(simAccount.Address)

		// ensure the validator doesn't exist already
		if _, found := k.GetValidator(ctx, address); found {
			return simtypes.NoOpMsg(stakingtypes.ModuleName, stakingtypes.TypeMsgCreateValidator, "validator already exists"), nil, nil
		}

		denom := k.GetParams(ctx).BondDenom
		floor := math.NewInt(MinSelfDelegationLowerBound)

		balance := bk.GetBalance(ctx, simAccount.Address, denom).Amount
		if balance.LT(floor) {
			return simtypes.NoOpMsg(
				stakingtypes.ModuleName,
				stakingtypes.TypeMsgCreateValidator,
				"balance below the chain minimum self delegation",
			), nil, nil
		}

		// Draw a self-delegation in [floor, balance].
		amount := floor
		if balance.GT(floor) {
			extra, err := simtypes.RandPositiveInt(r, balance.Sub(floor))
			if err != nil {
				return simtypes.NoOpMsg(
					stakingtypes.ModuleName,
					stakingtypes.TypeMsgCreateValidator,
					"unable to generate positive amount",
				), nil, err
			}
			amount = floor.Add(extra)
		}

		selfDelegation := sdk.NewCoin(denom, amount)

		account := ak.GetAccount(ctx, simAccount.Address)
		spendable := bk.SpendableCoins(ctx, account.GetAddress())

		var (
			fees sdk.Coins
			err  error
		)

		coins, hasNeg := spendable.SafeSub(selfDelegation)
		if !hasNeg {
			fees, err = simtypes.RandomFees(r, ctx, coins)
			if err != nil {
				return simtypes.NoOpMsg(
					stakingtypes.ModuleName,
					stakingtypes.TypeMsgCreateValidator,
					"unable to generate fees",
				), nil, err
			}
		}

		description := stakingtypes.NewDescription(
			simtypes.RandStringOfLength(r, 10),
			simtypes.RandStringOfLength(r, 10),
			simtypes.RandStringOfLength(r, 10),
			simtypes.RandStringOfLength(r, 10),
			simtypes.RandStringOfLength(r, 10),
		)

		maxCommission := sdk.NewDecWithPrec(int64(simtypes.RandIntBetween(r, 0, 100)), 2)
		commission := stakingtypes.NewCommissionRates(
			simtypes.RandomDecAmount(r, maxCommission),
			maxCommission,
			simtypes.RandomDecAmount(r, maxCommission),
		)

		msg, err := stakingtypes.NewMsgCreateValidator(
			address,
			simAccount.ConsKey.PubKey(),
			selfDelegation,
			description,
			commission,
			floor,
		)
		if err != nil {
			return simtypes.NoOpMsg(
				stakingtypes.ModuleName,
				stakingtypes.TypeMsgCreateValidator,
				"unable to create CreateValidator message",
			), nil, err
		}

		txCtx := simulation.OperationInput{
			R:             r,
			App:           app,
			TxGen:         MakeEncodingConfig().TxConfig,
			Cdc:           nil,
			Msg:           msg,
			MsgType:       msg.Type(),
			Context:       ctx,
			SimAccount:    simAccount,
			AccountKeeper: ak,
			ModuleName:    stakingtypes.ModuleName,
		}

		return simulation.GenAndDeliverTx(txCtx, fees)
	}
}
