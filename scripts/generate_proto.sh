#!/bin/bash
# MC protoc 生成脚本（已验证可用于 phonenode + edgeai）
# 用法：cd $HOME/mcchain && bash scripts/generate_proto.sh
# 依赖：$HOME/protoc/bin/protoc, $HOME/gopath/bin/protoc-gen-gocosmos, $HOME/gopath/bin/protoc-gen-grpc-gateway

set -e
export PATH="$HOME/gopath/bin:$HOME/protoc/bin:$PATH"

PROTO_ROOT=proto
GEN_DIR=_gen
GOGO_ROOT=$GOPATH/pkg/mod/github.com/cosmos/gogoproto@v1.4.10
SDK_PROTO=$GOPATH/pkg/mod/github.com/cosmos/cosmos-sdk@v0.47.3/proto
GRPC_GATEWAY_APIS=$GOPATH/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.16.0/third_party/googleapis
PROTOC_INCLUDE=$PROTOC_ROOT/include
M_ANY=Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types

MODULE=$1
if [ -z "$MODULE" ]; then
    echo "Usage: $0 <module-name>  (e.g., phonenode, edgeai)"
    exit 1
fi

PROTO_FILES="$PROTO_ROOT/mcchain/$MODULE/*.proto"
if ! ls $PROTO_FILES >/dev/null 2>&1; then
    echo "No proto files found at $PROTO_FILES"
    exit 1
fi

rm -rf $GEN_DIR && mkdir -p $GEN_DIR

echo "Generating pb.go for $MODULE..."
protoc \
  --proto_path=$PROTO_ROOT \
  --proto_path=$GOGO_ROOT \
  --proto_path=$SDK_PROTO \
  --proto_path=$GRPC_GATEWAY_APIS \
  --proto_path=$PROTOC_INCLUDE \
  --gocosmos_out=plugins=interfacetype+grpc,$M_ANY:$GEN_DIR \
  --gocosmos_opt=paths=source_relative \
  --grpc-gateway_out=$M_ANY:$GEN_DIR \
  --grpc-gateway_opt=paths=source_relative \
  $PROTO_FILES

echo "Moving to x/$MODULE/types/"
cp $GEN_DIR/mcchain/$MODULE/*.pb.go x/$MODULE/types/ 2>/dev/null
cp $GEN_DIR/mcchain/$MODULE/*.pb.gw.go x/$MODULE/types/ 2>/dev/null || true
rm -rf $GEN_DIR

echo "Done. Generated files in x/$MODULE/types/"
