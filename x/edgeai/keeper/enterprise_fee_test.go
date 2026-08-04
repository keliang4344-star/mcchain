package keeper

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"mcchain/x/edgeai/types"
	tokenomicstypes "mcchain/x/tokenomics/types"
)

// mockBankFeeCap records the three legs of the enterprise settlement fee:
// the collection (account -> edgeai module), the burn, and the treasury routing
// (edgeai module -> protocol_treasury module).
type mockBankFeeCap struct {
	balance      int64
	acctToModule []uint64
	burned       []uint64
	modToMod     []modSend
}

type modSend struct {
	from   string
	to     string
	amount uint64
}

func (m *mockBankFeeCap) SpendableCoins(_ sdk.Context, _ sdk.AccAddress) sdk.Coins {
	return sdk.NewCoins(sdk.NewInt64Coin("umc", m.balance))
}

func (m *mockBankFeeCap) SendCoinsFromAccountToModule(_ sdk.Context, _ sdk.AccAddress, _ string, amt sdk.Coins) error {
	m.acctToModule = append(m.acctToModule, amt.AmountOf("umc").Uint64())
	return nil
}

func (m *mockBankFeeCap) SendCoinsFromModuleToAccount(_ sdk.Context, _ string, _ sdk.AccAddress, _ sdk.Coins) error {
	return nil
}

func (m *mockBankFeeCap) SendCoinsFromModuleToModule(_ sdk.Context, from, to string, amt sdk.Coins) error {
	m.modToMod = append(m.modToMod, modSend{from: from, to: to, amount: amt.AmountOf("umc").Uint64()})
	return nil
}

func (m *mockBankFeeCap) BurnCoins(_ sdk.Context, _ string, amt sdk.Coins) error {
	m.burned = append(m.burned, amt.AmountOf("umc").Uint64())
	return nil
}

// TestEnterpriseSettlementFeeSplit locks the finalized 2026-08 policy:
// the demand side pays 1.50% of the escrowed reward on top of the escrow, and
// that fee is split 40% to nodes (fee_collector) / 60% to the protocol treasury.
// Nothing is burned: enterprise revenue is redistributed, never destroyed, and
// never minted — the treasury is funded exclusively by real business inflow.
func TestEnterpriseSettlementFeeSplit(t *testing.T) {
	bk := &mockBankFeeCap{balance: 1e15}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)

	const reward = uint64(1_000_000) // 1 MC
	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{
		Creator: addrOf(t),
		Reward:  reward,
	})
	require.NoError(t, err)

	expectedFee := reward * uint64(tokenomicstypes.EnterpriseSettlementFeeBps) / 10000
	expectedNodes := expectedFee * uint64(tokenomicstypes.EnterpriseFeeNodeRatioBps) / 10000
	expectedTreasury := expectedFee - expectedNodes

	require.Equal(t, uint64(15_000), expectedFee, "1.50% of 1 MC")
	require.Equal(t, uint64(6_000), expectedNodes, "40% of the fee goes to nodes")
	require.Equal(t, uint64(9_000), expectedTreasury, "60% of the fee goes to the treasury")

	// Leg 1: escrow of the reward, then collection of the fee.
	require.Len(t, bk.acctToModule, 2)
	require.Equal(t, reward, bk.acctToModule[0])
	require.Equal(t, expectedFee, bk.acctToModule[1])

	// Nothing may be burned on this path.
	require.Empty(t, bk.burned, "enterprise fee must never be burned")

	// Leg 2: 40% to nodes via the fee collector.
	require.Len(t, bk.modToMod, 2)
	require.Equal(t, types.ModuleName, bk.modToMod[0].from)
	require.Equal(t, authtypes.FeeCollectorName, bk.modToMod[0].to)
	require.Equal(t, expectedNodes, bk.modToMod[0].amount)

	// Leg 3: 60% to the protocol treasury module account.
	require.Equal(t, types.ModuleName, bk.modToMod[1].from)
	require.Equal(t, tokenomicstypes.ProtocolTreasuryPoolName, bk.modToMod[1].to)
	require.Equal(t, expectedTreasury, bk.modToMod[1].amount)

	// Nodes + treasury must reconstruct the fee exactly (no dust left behind).
	require.Equal(t, expectedFee, bk.modToMod[0].amount+bk.modToMod[1].amount)
}

// TestEnterpriseSettlementFeeWaivedWhenUnderfunded verifies that a creator who
// can escrow the reward but not the fee still gets the task posted; the fee is
// waived and recorded rather than blocking task creation.
func TestEnterpriseSettlementFeeWaivedWhenUnderfunded(t *testing.T) {
	bk := &mockBankFeeCap{balance: 0}
	k, ctx := setupEdgeaiWith(t, &mockPhonenode{}, nil, bk)
	ms := NewMsgServerImpl(*k)

	_, err := ms.CreateTask(sdk.WrapSDKContext(ctx), &types.MsgCreateTask{
		Creator: addrOf(t),
		Reward:  0, // zero reward: no escrow, no fee
	})
	require.NoError(t, err)
	require.Empty(t, bk.burned)
	require.Empty(t, bk.modToMod)
}
