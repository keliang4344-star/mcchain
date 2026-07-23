package types

import (
	"context"

	grpc1 "github.com/cosmos/gogoproto/grpc"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// =============================================================================
// Query request/response types — mirrors proto/mcchain/dex/query.proto
// =============================================================================

type QueryPoolRequest struct {
	PoolId uint64 `json:"pool_id"`
}

func (m *QueryPoolRequest) Reset()         { *m = QueryPoolRequest{} }
func (m *QueryPoolRequest) String() string  { return "QueryPoolRequest" }
func (*QueryPoolRequest) ProtoMessage()     {}

type QueryPoolResponse struct {
	Pool *Pool `json:"pool"`
}

func (m *QueryPoolResponse) Reset()         { *m = QueryPoolResponse{} }
func (m *QueryPoolResponse) String() string  { return "QueryPoolResponse" }
func (*QueryPoolResponse) ProtoMessage()     {}

type QueryPoolsRequest struct {
	Pagination []byte `json:"pagination"`
}

func (m *QueryPoolsRequest) Reset()         { *m = QueryPoolsRequest{} }
func (m *QueryPoolsRequest) String() string  { return "QueryPoolsRequest" }
func (*QueryPoolsRequest) ProtoMessage()     {}

type QueryPoolsResponse struct {
	Pools      []Pool `json:"pools"`
	Pagination []byte `json:"pagination"`
}

func (m *QueryPoolsResponse) Reset()         { *m = QueryPoolsResponse{} }
func (m *QueryPoolsResponse) String() string  { return "QueryPoolsResponse" }
func (*QueryPoolsResponse) ProtoMessage()     {}

type QueryEstimateSwapRequest struct {
	PoolId   uint64 `json:"pool_id"`
	DenomIn  string `json:"denom_in"`
	AmountIn string `json:"amount_in"`
	DenomOut string `json:"denom_out"`
}

func (m *QueryEstimateSwapRequest) Reset()         { *m = QueryEstimateSwapRequest{} }
func (m *QueryEstimateSwapRequest) String() string  { return "QueryEstimateSwapRequest" }
func (*QueryEstimateSwapRequest) ProtoMessage()     {}

type QueryEstimateSwapResponse struct {
	AmountOut      string `json:"amount_out"`
	PriceImpactBps string `json:"price_impact_bps"`
}

func (m *QueryEstimateSwapResponse) Reset()         { *m = QueryEstimateSwapResponse{} }
func (m *QueryEstimateSwapResponse) String() string  { return "QueryEstimateSwapResponse" }
func (*QueryEstimateSwapResponse) ProtoMessage()     {}

type QueryPriceRequest struct {
	PoolId uint64 `json:"pool_id"`
	Denom  string `json:"denom"`
}

func (m *QueryPriceRequest) Reset()         { *m = QueryPriceRequest{} }
func (m *QueryPriceRequest) String() string  { return "QueryPriceRequest" }
func (*QueryPriceRequest) ProtoMessage()     {}

type QueryPriceResponse struct {
	Price      string `json:"price"`
	PriceDenom string `json:"price_denom"`
}

func (m *QueryPriceResponse) Reset()         { *m = QueryPriceResponse{} }
func (m *QueryPriceResponse) String() string  { return "QueryPriceResponse" }
func (*QueryPriceResponse) ProtoMessage()     {}

// QueryLiquidityLockRequest queries the LP lock for a specific address and pool.
type QueryLiquidityLockRequest struct {
	LpAddress string `json:"lp_address"`
	PoolId    uint64 `json:"pool_id"`
}

func (m *QueryLiquidityLockRequest) Reset()         { *m = QueryLiquidityLockRequest{} }
func (m *QueryLiquidityLockRequest) String() string  { return "QueryLiquidityLockRequest" }
func (*QueryLiquidityLockRequest) ProtoMessage()     {}

type QueryLiquidityLockResponse struct {
	Lock *LiquidityLock `json:"lock"`
}

func (m *QueryLiquidityLockResponse) Reset()         { *m = QueryLiquidityLockResponse{} }
func (m *QueryLiquidityLockResponse) String() string  { return "QueryLiquidityLockResponse" }
func (*QueryLiquidityLockResponse) ProtoMessage()     {}

// =============================================================================
// Service interfaces
// =============================================================================

// MsgServer is the server API for the Msg service.
type MsgServer interface {
	CreatePool(context.Context, *MsgCreatePool) (*MsgCreatePoolResponse, error)
	AddLiquidity(context.Context, *MsgAddLiquidity) (*MsgAddLiquidityResponse, error)
	RemoveLiquidity(context.Context, *MsgRemoveLiquidity) (*MsgRemoveLiquidityResponse, error)
	SwapExactIn(context.Context, *MsgSwapExactIn) (*MsgSwapExactInResponse, error)
}

// QueryServer is the server API for the Query service.
type QueryServer interface {
	Pool(context.Context, *QueryPoolRequest) (*QueryPoolResponse, error)
	Pools(context.Context, *QueryPoolsRequest) (*QueryPoolsResponse, error)
	EstimateSwap(context.Context, *QueryEstimateSwapRequest) (*QueryEstimateSwapResponse, error)
	Price(context.Context, *QueryPriceRequest) (*QueryPriceResponse, error)
	LiquidityLock(context.Context, *QueryLiquidityLockRequest) (*QueryLiquidityLockResponse, error)
}

// =============================================================================
// Msg service descriptor and handler registration
// =============================================================================

var _Msg_serviceDesc = grpc.ServiceDesc{
	ServiceName: "mcchain.dex.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreatePool",
			Handler:    _Msg_CreatePool_Handler,
		},
		{
			MethodName: "AddLiquidity",
			Handler:    _Msg_AddLiquidity_Handler,
		},
		{
			MethodName: "RemoveLiquidity",
			Handler:    _Msg_RemoveLiquidity_Handler,
		},
		{
			MethodName: "SwapExactIn",
			Handler:    _Msg_SwapExactIn_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "mcchain/dex/tx.proto",
}

func RegisterMsgServer(s grpc1.Server, srv MsgServer) {
	s.RegisterService(&_Msg_serviceDesc, srv)
}

func _Msg_CreatePool_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgCreatePool)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).CreatePool(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Msg/CreatePool",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).CreatePool(ctx, req.(*MsgCreatePool))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_AddLiquidity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgAddLiquidity)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).AddLiquidity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Msg/AddLiquidity",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).AddLiquidity(ctx, req.(*MsgAddLiquidity))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_RemoveLiquidity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgRemoveLiquidity)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).RemoveLiquidity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Msg/RemoveLiquidity",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).RemoveLiquidity(ctx, req.(*MsgRemoveLiquidity))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_SwapExactIn_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgSwapExactIn)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).SwapExactIn(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Msg/SwapExactIn",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).SwapExactIn(ctx, req.(*MsgSwapExactIn))
	}
	return interceptor(ctx, in, info, handler)
}

// =============================================================================
// Query service descriptor and handler registration
// =============================================================================

var _Query_serviceDesc = grpc.ServiceDesc{
	ServiceName: "mcchain.dex.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Pool",
			Handler:    _Query_Pool_Handler,
		},
		{
			MethodName: "Pools",
			Handler:    _Query_Pools_Handler,
		},
		{
			MethodName: "EstimateSwap",
			Handler:    _Query_EstimateSwap_Handler,
		},
		{
			MethodName: "Price",
			Handler:    _Query_Price_Handler,
		},
		{
			MethodName: "LiquidityLock",
			Handler:    _Query_LiquidityLock_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "mcchain/dex/query.proto",
}

func RegisterQueryServer(s grpc1.Server, srv QueryServer) {
	s.RegisterService(&_Query_serviceDesc, srv)
}

func _Query_Pool_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryPoolRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Pool(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Query/Pool",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Pool(ctx, req.(*QueryPoolRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_Pools_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryPoolsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Pools(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Query/Pools",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Pools(ctx, req.(*QueryPoolsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_EstimateSwap_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryEstimateSwapRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).EstimateSwap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Query/EstimateSwap",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).EstimateSwap(ctx, req.(*QueryEstimateSwapRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_Price_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryPriceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Price(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Query/Price",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Price(ctx, req.(*QueryPriceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_LiquidityLock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryLiquidityLockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).LiquidityLock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/mcchain.dex.Query/LiquidityLock",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).LiquidityLock(ctx, req.(*QueryLiquidityLockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// =============================================================================
// Query client (for grpc-gateway) — minimal stub
// =============================================================================

// QueryClient is the client API for Query service.
type QueryClient interface {
	Pool(ctx context.Context, in *QueryPoolRequest, opts ...grpc.CallOption) (*QueryPoolResponse, error)
	Pools(ctx context.Context, in *QueryPoolsRequest, opts ...grpc.CallOption) (*QueryPoolsResponse, error)
	EstimateSwap(ctx context.Context, in *QueryEstimateSwapRequest, opts ...grpc.CallOption) (*QueryEstimateSwapResponse, error)
	Price(ctx context.Context, in *QueryPriceRequest, opts ...grpc.CallOption) (*QueryPriceResponse, error)
	LiquidityLock(ctx context.Context, in *QueryLiquidityLockRequest, opts ...grpc.CallOption) (*QueryLiquidityLockResponse, error)
}

type queryClient struct {
	cc grpc1.ClientConn
}

func NewQueryClient(cc grpc1.ClientConn) QueryClient {
	return &queryClient{cc: cc}
}

func (c *queryClient) Pool(ctx context.Context, in *QueryPoolRequest, opts ...grpc.CallOption) (*QueryPoolResponse, error) {
	out := new(QueryPoolResponse)
	err := c.cc.Invoke(ctx, "/mcchain.dex.Query/Pool", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) Pools(ctx context.Context, in *QueryPoolsRequest, opts ...grpc.CallOption) (*QueryPoolsResponse, error) {
	out := new(QueryPoolsResponse)
	err := c.cc.Invoke(ctx, "/mcchain.dex.Query/Pools", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) EstimateSwap(ctx context.Context, in *QueryEstimateSwapRequest, opts ...grpc.CallOption) (*QueryEstimateSwapResponse, error) {
	out := new(QueryEstimateSwapResponse)
	err := c.cc.Invoke(ctx, "/mcchain.dex.Query/EstimateSwap", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) Price(ctx context.Context, in *QueryPriceRequest, opts ...grpc.CallOption) (*QueryPriceResponse, error) {
	out := new(QueryPriceResponse)
	err := c.cc.Invoke(ctx, "/mcchain.dex.Query/Price", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) LiquidityLock(ctx context.Context, in *QueryLiquidityLockRequest, opts ...grpc.CallOption) (*QueryLiquidityLockResponse, error) {
	out := new(QueryLiquidityLockResponse)
	err := c.cc.Invoke(ctx, "/mcchain.dex.Query/LiquidityLock", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RegisterQueryHandlerClient registers the http handlers for the Query service to mux.
// This is a minimal stub — full grpc-gateway requires generated code.
func RegisterQueryHandlerClient(ctx context.Context, mux interface{}, client QueryClient) error {
	return nil
}

var _ status.Status
var _ codes.Code
