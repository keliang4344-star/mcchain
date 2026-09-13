package keeper_test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"mcchain/x/phonenode/keeper"
	"mcchain/x/phonenode/types"
)

// TestUpdateNodeAllowance_GovernanceGated 验证节点资本津贴治理旋钮：
// 仅治理模块账户可调，非治理账户被拒且配置不被改写。
func TestUpdateNodeAllowance_GovernanceGated(t *testing.T) {
	bank := newMockBankAllowance()
	k, ctx := newKeeperWithBank(t, bank)

	ms := keeper.NewMsgServerImpl(*k)
	govAddr := keeper.GovAuthority()

	// 1) 治理模块账户可调，配置写入成功。
	_, err := ms.UpdateNodeAllowance(ctx, types.NewMsgUpdateNodeAllowance(govAddr, true, 50_000_000))
	require.NoError(t, err)
	cfg := k.GetNodeAllowanceConfig(ctx)
	require.True(t, cfg.Enabled)
	require.Equal(t, uint64(50_000_000), cfg.PerDay)

	// 2) 非治理账户必须被拒（ErrUnauthorized）。
	random := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
	_, err = ms.UpdateNodeAllowance(ctx, types.NewMsgUpdateNodeAllowance(random, false, 0))
	require.Error(t, err)

	// 3) 配置未被非治理账户改写，仍保持治理设定值。
	cfg2 := k.GetNodeAllowanceConfig(ctx)
	require.True(t, cfg2.Enabled, "非治理账户不得改写津贴配置")
	require.Equal(t, uint64(50_000_000), cfg2.PerDay)
}
