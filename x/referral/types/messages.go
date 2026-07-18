package types

// Referral represents a single referral record.
// Mirrors the proto message defined in proto/mcchain/referral/query.proto.
type Referral struct {
	ReferralId uint64 `json:"referral_id"`
	Inviter    string `json:"inviter"`
	Invitee    string `json:"invitee"`
	InviteCode string `json:"invite_code"`
	CreatedAt  int64  `json:"created_at"` // block height when created
	Status     string `json:"status"`     // "active" or "claimed"
}

func (m *Referral) Reset()         { *m = Referral{} }
func (m *Referral) String() string { return "Referral" }
func (*Referral) ProtoMessage()    {}

// MsgCreateReferral is the message to create a new referral.
// Mirrors proto/mcchain/referral/tx.proto.
type MsgCreateReferral struct {
	Inviter    string `json:"inviter"`
	Invitee    string `json:"invitee"`
	InviteCode string `json:"invite_code"`
}

func (m *MsgCreateReferral) Reset()         { *m = MsgCreateReferral{} }
func (m *MsgCreateReferral) String() string { return "MsgCreateReferral" }
func (m *MsgCreateReferral) ProtoMessage()  {}

// MsgCreateReferralResponse is the response for MsgCreateReferral.
type MsgCreateReferralResponse struct {
	ReferralId uint64 `json:"referral_id"`
}

func (m *MsgCreateReferralResponse) Reset()         { *m = MsgCreateReferralResponse{} }
func (m *MsgCreateReferralResponse) String() string { return "MsgCreateReferralResponse" }
func (m *MsgCreateReferralResponse) ProtoMessage()  {}

// MsgClaimReferralReward is the message to claim accumulated referral rewards.
type MsgClaimReferralReward struct {
	Claimer string `json:"claimer"`
}

func (m *MsgClaimReferralReward) Reset()         { *m = MsgClaimReferralReward{} }
func (m *MsgClaimReferralReward) String() string { return "MsgClaimReferralReward" }
func (m *MsgClaimReferralReward) ProtoMessage()  {}

// MsgClaimReferralRewardResponse is the response for MsgClaimReferralReward.
type MsgClaimReferralRewardResponse struct {
	Amount string `json:"amount"` // total reward claimed, in umc
}

func (m *MsgClaimReferralRewardResponse) Reset()         { *m = MsgClaimReferralRewardResponse{} }
func (m *MsgClaimReferralRewardResponse) String() string { return "MsgClaimReferralRewardResponse" }
func (m *MsgClaimReferralRewardResponse) ProtoMessage()  {}

// Ensure the message types implement sdk.Msg interface.
// The actual ValidateBasic / GetSigners will panic until proto is generated,
// but the interface assertion serves as a compile-time check.
var (
	_ codecProtoMarshaler = (*MsgCreateReferral)(nil)
	_ codecProtoMarshaler = (*MsgCreateReferralResponse)(nil)
	_ codecProtoMarshaler = (*MsgClaimReferralReward)(nil)
	_ codecProtoMarshaler = (*MsgClaimReferralRewardResponse)(nil)
)

// codecProtoMarshaler is a local alias for codec.ProtoMarshaler to avoid import cycles.
// In practice, msgs implement sdk.Msg, which embeds proto.Message.
type codecProtoMarshaler interface {
	ProtoMessage()
	Reset()
	String() string
}
