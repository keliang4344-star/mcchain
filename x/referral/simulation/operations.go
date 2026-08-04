package simulation

import (
	"math/rand"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"mcchain/x/referral/keeper"
	"mcchain/x/referral/types"
)

// SimulateMsgCreateReferral generates a MsgCreateReferral.
func SimulateMsgCreateReferral(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		inviter, _ := simtypes.RandomAcc(r, accs)
		invitee, _ := simtypes.RandomAcc(r, accs)
		if inviter.Address.Equals(invitee.Address) {
			invitee = accs[(r.Intn(len(accs)-1)+1)%len(accs)]
		}
		msg := &types.MsgCreateReferral{
			Inviter:    inviter.Address.String(),
			Invitee:    invitee.Address.String(),
			InviteCode: simtypes.RandStringOfLength(r, 12),
		}
		return genDeliver(r, app, ctx, inviter, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}

// SimulateMsgClaimReferralReward generates a MsgClaimReferralReward.
func SimulateMsgClaimReferralReward(
	ak types.AccountKeeper,
	bk types.BankKeeper,
	k keeper.Keeper,
) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		claimer, _ := simtypes.RandomAcc(r, accs)
		msg := &types.MsgClaimReferralReward{
			Claimer: claimer.Address.String(),
		}
		return genDeliver(r, app, ctx, claimer, ak, bk, msg, msg.Type(), sdk.Coins{})
	}
}
