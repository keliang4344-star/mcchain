package keeper

import (
	"context"
	"encoding/json"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"mcchain/x/mcchain/types"
)

// ChainInfo 返回链系统锚点信息（白皮书行 590）。
func (k Keeper) ChainInfo(ctx sdk.Context) types.ChainInfo {
	params := k.GetParams(ctx)
	lastBlock := k.GetLastBlockHeight(ctx)
	return types.ChainInfo{
		ChainName:     params.ChainName,
		ChainVersion:  params.ChainVersion,
		GenesisTime:   params.GenesisTime,
		BlockHeight:   ctx.BlockHeight(),
		LastHeartbeat: lastBlock,
	}
}

// RecordHeartbeat 在 BeginBlock 记录链心跳 — 存储最新区块高度。
func (k Keeper) RecordHeartbeat(ctx sdk.Context) {
	store := ctx.KVStore(k.storeKey)
	bz := sdk.Uint64ToBigEndian(uint64(ctx.BlockHeight()))
	store.Set(types.HeartbeatKey, bz)
}

// GetLastBlockHeight 返回上一次心跳记录的区块高度。
func (k Keeper) GetLastBlockHeight(ctx sdk.Context) int64 {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.HeartbeatKey)
	if bz == nil {
		return 0
	}
	return int64(sdk.BigEndianToUint64(bz))
}

// chainInfoServer implements a gRPC handler for QueryChainInfo.
type chainInfoServer struct {
	keeper *Keeper
}

// ChainInfo gRPC handler.
func (s *chainInfoServer) ChainInfo(ctx context.Context, _ *chainInfoRequest) (*chainInfoResponse, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	info := s.keeper.ChainInfo(sdkCtx)
	bz, err := json.Marshal(info)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal: %v", err)
	}
	return &chainInfoResponse{Data: bz}, nil
}

// chainInfoRequest is a stub proto.Message.
type chainInfoRequest struct{}

func (m *chainInfoRequest) Reset()         {}
func (m *chainInfoRequest) String() string { return "" }
func (m *chainInfoRequest) ProtoMessage()  {}

// chainInfoResponse is a stub proto.Message wrapping JSON bytes.
type chainInfoResponse struct {
	Data []byte
}

func (m *chainInfoResponse) Reset()         {}
func (m *chainInfoResponse) String() string { return string(m.Data) }
func (m *chainInfoResponse) ProtoMessage()  {}

// RegisterChainInfoService registers the ChainInfo query on the gRPC server.
func RegisterChainInfoService(server *grpc.Server, k *Keeper) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "mcchain.mcchain.Query",
		HandlerType: (*chainInfoServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "ChainInfo",
				Handler: func(srv interface{}, ctx context.Context, dec func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
					in := &chainInfoRequest{}
					if err := dec(in); err != nil {
						return nil, err
					}
					return srv.(*chainInfoServer).ChainInfo(ctx, in)
				},
			},
		},
	}, &chainInfoServer{keeper: k})
}
