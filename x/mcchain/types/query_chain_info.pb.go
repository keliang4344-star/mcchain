package types

import (
	fmt "fmt"
	io "io"
)

// QueryChainInfoRequest is request type for the Query/ChainInfo RPC method.
type QueryChainInfoRequest struct{}

func (m *QueryChainInfoRequest) Reset()         { *m = QueryChainInfoRequest{} }
func (m *QueryChainInfoRequest) String() string { return fmt.Sprintf("%+v", *m) }
func (m *QueryChainInfoRequest) ProtoMessage()  {}
func (m *QueryChainInfoRequest) Marshal() ([]byte, error) {
	return m.MarshalToSizedBuffer(make([]byte, m.Size()))
}
func (m *QueryChainInfoRequest) MarshalTo(dAtA []byte) (int, error) {
	return m.MarshalToSizedBuffer(dAtA[:m.Size()])
}
func (m *QueryChainInfoRequest) MarshalToSizedBuffer(dAtA []byte) ([]byte, error) {
	return dAtA, nil
}
func (m *QueryChainInfoRequest) Size() int {
	if m == nil {
		return 0
	}
	return 0
}
func (m *QueryChainInfoRequest) Unmarshal(dAtA []byte) error {
	return nil
}

// QueryChainInfoResponse is response type for the Query/ChainInfo RPC method.
type QueryChainInfoResponse struct {
	ChainName     string `protobuf:"bytes,1,opt,name=chain_name,json=chainName,proto3" json:"chain_name,omitempty"`
	ChainVersion  string `protobuf:"bytes,2,opt,name=chain_version,json=chainVersion,proto3" json:"chain_version,omitempty"`
	GenesisTime   int64  `protobuf:"varint,3,opt,name=genesis_time,json=genesisTime,proto3" json:"genesis_time,omitempty"`
	BlockHeight   int64  `protobuf:"varint,4,opt,name=block_height,json=blockHeight,proto3" json:"block_height,omitempty"`
	LastHeartbeat int64  `protobuf:"varint,5,opt,name=last_heartbeat,json=lastHeartbeat,proto3" json:"last_heartbeat,omitempty"`
}

func (m *QueryChainInfoResponse) Reset()         { *m = QueryChainInfoResponse{} }
func (m *QueryChainInfoResponse) String() string { return fmt.Sprintf("%+v", *m) }
func (m *QueryChainInfoResponse) ProtoMessage()  {}

func (m *QueryChainInfoResponse) Marshal() ([]byte, error) {
	size := m.Size()
	dAtA := make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *QueryChainInfoResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *QueryChainInfoResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	if m.LastHeartbeat != 0 {
		i = encodeVarintQuery(dAtA, i, uint64(m.LastHeartbeat))
		i--
		dAtA[i] = 0x28
	}
	if m.BlockHeight != 0 {
		i = encodeVarintQuery(dAtA, i, uint64(m.BlockHeight))
		i--
		dAtA[i] = 0x20
	}
	if m.GenesisTime != 0 {
		i = encodeVarintQuery(dAtA, i, uint64(m.GenesisTime))
		i--
		dAtA[i] = 0x18
	}
	if len(m.ChainVersion) > 0 {
		i -= len(m.ChainVersion)
		copy(dAtA[i:], m.ChainVersion)
		i = encodeVarintQuery(dAtA, i, uint64(len(m.ChainVersion)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.ChainName) > 0 {
		i -= len(m.ChainName)
		copy(dAtA[i:], m.ChainName)
		i = encodeVarintQuery(dAtA, i, uint64(len(m.ChainName)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *QueryChainInfoResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.ChainName)
	if l > 0 {
		n += 1 + l + sovQuery(uint64(l))
	}
	l = len(m.ChainVersion)
	if l > 0 {
		n += 1 + l + sovQuery(uint64(l))
	}
	if m.GenesisTime != 0 {
		n += 1 + sovQuery(uint64(m.GenesisTime))
	}
	if m.BlockHeight != 0 {
		n += 1 + sovQuery(uint64(m.BlockHeight))
	}
	if m.LastHeartbeat != 0 {
		n += 1 + sovQuery(uint64(m.LastHeartbeat))
	}
	return n
}

func (m *QueryChainInfoResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowQuery
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: QueryChainInfoResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: QueryChainInfoResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ChainName", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowQuery
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthQuery
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthQuery
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ChainName = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ChainVersion", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowQuery
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthQuery
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthQuery
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ChainVersion = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field GenesisTime", wireType)
			}
			m.GenesisTime = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowQuery
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.GenesisTime |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field BlockHeight", wireType)
			}
			m.BlockHeight = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowQuery
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.BlockHeight |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field LastHeartbeat", wireType)
			}
			m.LastHeartbeat = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowQuery
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.LastHeartbeat |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipQuery(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthQuery
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}
	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
