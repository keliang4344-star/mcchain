package types

import (
	"strings"
	"testing"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"
	"mcchain/testutil/sample"
)

func TestMsgSubmitContribution_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgSubmitContribution
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgSubmitContribution{
				Creator: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: MsgSubmitContribution{
				Creator: sample.AccAddress(),
				TaskId:  "task-0001",
			},
		},
		{
			// task_id 直接拼成 KVStore 键，空值必须拒绝。
			name: "empty task id",
			msg: MsgSubmitContribution{
				Creator: sample.AccAddress(),
			},
			err: sdkerrors.ErrInvalidRequest,
		},
		{
			// 超长 task_id 会造成确定性状态膨胀，长度必须设上界。
			name: "oversized task id",
			msg: MsgSubmitContribution{
				Creator: sample.AccAddress(),
				TaskId:  strings.Repeat("t", MaxContributionTaskIDLength+1),
			},
			err: sdkerrors.ErrInvalidRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.ValidateBasic()
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}
