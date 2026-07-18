package types

import (
	context "context"

	grpc1 "github.com/cosmos/gogoproto/grpc"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// MsgServer is the server API for Msg service.
type MsgServer interface {
	CreateReferral(context.Context, *MsgCreateReferral) (*MsgCreateReferralResponse, error)
	ClaimReferralReward(context.Context, *MsgClaimReferralReward) (*MsgClaimReferralRewardResponse, error)
}

// QueryServer is the server API for Query service.
type QueryServer interface {
	Referral(context.Context, *QueryReferralRequest) (*QueryReferralResponse, error)
	ReferralsByInviter(context.Context, *QueryReferralsByInviterRequest) (*QueryReferralsByInviterResponse, error)
	PendingRewards(context.Context, *QueryPendingRewardsRequest) (*QueryPendingRewardsResponse, error)
}

// ---- Query request/response types (match proto/mcchain/referral/query.proto) ----

type QueryReferralRequest struct {
	ReferralId uint64
}

func (m *QueryReferralRequest) Reset()         { *m = QueryReferralRequest{} }
func (m *QueryReferralRequest) String() string { return "QueryReferralRequest" }
func (*QueryReferralRequest) ProtoMessage()    {}

type QueryReferralResponse struct {
	Referral Referral
}

func (m *QueryReferralResponse) Reset()         { *m = QueryReferralResponse{} }
func (m *QueryReferralResponse) String() string { return "QueryReferralResponse" }
func (*QueryReferralResponse) ProtoMessage()    {}

type QueryReferralsByInviterRequest struct {
	Inviter string
}

func (m *QueryReferralsByInviterRequest) Reset()         { *m = QueryReferralsByInviterRequest{} }
func (m *QueryReferralsByInviterRequest) String() string { return "QueryReferralsByInviterRequest" }
func (*QueryReferralsByInviterRequest) ProtoMessage()    {}

type QueryReferralsByInviterResponse struct {
	Referrals []Referral
}

func (m *QueryReferralsByInviterResponse) Reset()         { *m = QueryReferralsByInviterResponse{} }
func (m *QueryReferralsByInviterResponse) String() string { return "QueryReferralsByInviterResponse" }
func (*QueryReferralsByInviterResponse) ProtoMessage()    {}

type QueryPendingRewardsRequest struct {
	Claimer string
}

func (m *QueryPendingRewardsRequest) Reset()         { *m = QueryPendingRewardsRequest{} }
func (m *QueryPendingRewardsRequest) String() string { return "QueryPendingRewardsRequest" }
func (*QueryPendingRewardsRequest) ProtoMessage()    {}

type QueryPendingRewardsResponse struct {
	Amount string
}

func (m *QueryPendingRewardsResponse) Reset()         { *m = QueryPendingRewardsResponse{} }
func (m *QueryPendingRewardsResponse) String() string { return "QueryPendingRewardsResponse" }
func (*QueryPendingRewardsResponse) ProtoMessage()    {}

// ---- Server registration functions ----

var _Msg_serviceDesc = grpc.ServiceDesc{
	ServiceName: "mcchain.referral.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateReferral",
			Handler:    _Msg_CreateReferral_Handler,
		},
		{
			MethodName: "ClaimReferralReward",
			Handler:    _Msg_ClaimReferralReward_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "mcchain/referral/tx.proto",
}

func RegisterMsgServer(s grpc1.Server, srv MsgServer) {
	s.RegisterService(&_Msg_serviceDesc, srv)
}

func _Msg_CreateReferral_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgCreateReferral)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).CreateReferral(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.referral.Msg/CreateReferral",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).CreateReferral(ctx, req.(*MsgCreateReferral))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_ClaimReferralReward_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgClaimReferralReward)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).ClaimReferralReward(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.referral.Msg/ClaimReferralReward",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).ClaimReferralReward(ctx, req.(*MsgClaimReferralReward))
	}
	return interceptor(ctx, in, info, handler)
}

var _Query_serviceDesc = grpc.ServiceDesc{
	ServiceName: "mcchain.referral.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Referral",
			Handler:    _Query_Referral_Handler,
		},
		{
			MethodName: "ReferralsByInviter",
			Handler:    _Query_ReferralsByInviter_Handler,
		},
		{
			MethodName: "PendingRewards",
			Handler:    _Query_PendingRewards_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "mcchain/referral/query.proto",
}

func RegisterQueryServer(s grpc1.Server, srv QueryServer) {
	s.RegisterService(&_Query_serviceDesc, srv)
}

func _Query_Referral_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryReferralRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Referral(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.referral.Query/Referral",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Referral(ctx, req.(*QueryReferralRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_ReferralsByInviter_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryReferralsByInviterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).ReferralsByInviter(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.referral.Query/ReferralsByInviter",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).ReferralsByInviter(ctx, req.(*QueryReferralsByInviterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_PendingRewards_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryPendingRewardsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).PendingRewards(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.referral.Query/PendingRewards",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).PendingRewards(ctx, req.(*QueryPendingRewardsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _ status.Status
var _ codes.Code
