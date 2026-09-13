package types

import (
	"strings"
	"testing"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"
	"mcchain/testutil/sample"
)

func TestMsgAttestDevice_ValidateBasic(t *testing.T) {
	tests := []struct {
		name string
		msg  MsgAttestDevice
		err  error
	}{
		{
			name: "invalid address",
			msg: MsgAttestDevice{
				Creator: "invalid_address",
			},
			err: sdkerrors.ErrInvalidAddress,
		}, {
			name: "valid address",
			msg: MsgAttestDevice{
				Creator:   sample.AccAddress(),
				Challenge: "challenge-abc",
			},
		},
		{
			// challenge 会进入留痕键与一次性消费索引，空值必须拒绝。
			name: "empty challenge",
			msg: MsgAttestDevice{
				Creator: sample.AccAddress(),
			},
			err: sdkerrors.ErrInvalidRequest,
		},
		{
			// 超长 challenge 会把键与历史撑大，长度必须设上界。
			name: "oversized challenge",
			msg: MsgAttestDevice{
				Creator:   sample.AccAddress(),
				Challenge: strings.Repeat("a", MaxAttestChallengeLength+1),
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
