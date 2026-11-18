#!/bin/bash
# MC protoc 生成脚本（唯一入口）。
#
# 用法：bash scripts/generate_proto.sh <module>     e.g. phonenode / edgeai / tokenomics
# 依赖：
#   - protoc 在 PATH（或设置 PROTOC）
#   - protoc-gen-gocosmos / protoc-gen-grpc-gateway 在 PATH（go install 生成）
#
# 版本不再硬编码：依赖路径从 go.mod 实际锁定版本解析，避免脚本与 go.mod 漂移。
set -euo pipefail

PROTO_ROOT=proto
GEN_DIR=_gen
M_ANY=Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types

PROTOC="${PROTOC:-protoc}"
command -v "$PROTOC" >/dev/null 2>&1 || {
  echo "ERROR: protoc not found in PATH (install protoc or set PROTOC=/path/to/protoc)" >&2
  exit 1
}
command -v protoc-gen-gocosmos >/dev/null 2>&1 || {
  echo "ERROR: protoc-gen-gocosmos not found (go install github.com/cosmos/gogoproto/protoc-gen-gocosmos)" >&2
  exit 1
}
command -v protoc-gen-grpc-gateway >/dev/null 2>&1 || {
  echo "ERROR: protoc-gen-grpc-gateway not found (go install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway)" >&2
  exit 1
}

GOMODCACHE="$(go env GOMODCACHE)"
GOGO_VER="$(go list -m -f '{{.Version}}' github.com/cosmos/gogoproto)"
SDK_VER="$(go list -m -f '{{.Version}}' github.com/cosmos/cosmos-sdk)"
GRPC_GW_VER="$(go list -m -f '{{.Version}}' github.com/grpc-ecosystem/grpc-gateway)"

GOGO_ROOT="${GOMODCACHE}/github.com/cosmos/gogoproto@${GOGO_VER}"
SDK_PROTO="${GOMODCACHE}/github.com/cosmos/cosmos-sdk@${SDK_VER}/proto"
GRPC_GATEWAY_APIS="${GOMODCACHE}/github.com/grpc-ecosystem/grpc-gateway@${GRPC_GW_VER}/third_party/googleapis"
PROTOC_INCLUDE="${PROTO_ROOT}/include"

for p in "$GOGO_ROOT" "$SDK_PROTO" "$GRPC_GATEWAY_APIS"; do
  [[ -d "$p" ]] || {
    echo "ERROR: proto include path not found: $p (run 'go mod download' first)" >&2
    exit 1
  }
done

MODULE="${1:-}"
if [[ -z "$MODULE" ]]; then
    echo "Usage: $0 <module-name>  (e.g., phonenode, edgeai)" >&2
    exit 1
fi

PROTO_FILES="$PROTO_ROOT/mcchain/$MODULE"/*.proto
if ! compgen -G "$PROTO_FILES" >/dev/null; then
    echo "ERROR: no proto files found at $PROTO_ROOT/mcchain/$MODULE/*.proto" >&2
    exit 1
fi

rm -rf "$GEN_DIR" && mkdir -p "$GEN_DIR"

echo "Generating pb.go for $MODULE (sdk=$SDK_VER gogoproto=$GOGO_VER)..."
"$PROTOC" \
  --proto_path="$PROTO_ROOT" \
  --proto_path="$GOGO_ROOT" \
  --proto_path="$SDK_PROTO" \
  --proto_path="$GRPC_GATEWAY_APIS" \
  --proto_path="$PROTOC_INCLUDE" \
  --gocosmos_out="plugins=interfacetype+grpc,$M_ANY:$GEN_DIR" \
  --gocosmos_opt=paths=source_relative \
  --grpc-gateway_out="$M_ANY:$GEN_DIR" \
  --grpc-gateway_opt=paths=source_relative \
  $PROTO_FILES

echo "Moving generated files to x/$MODULE/types/"
cp "$GEN_DIR"/mcchain/"$MODULE"/*.pb.go "x/$MODULE/types/"
if compgen -G "$GEN_DIR/mcchain/$MODULE/*.pb.gw.go" >/dev/null; then
  cp "$GEN_DIR"/mcchain/"$MODULE"/*.pb.gw.go "x/$MODULE/types/"
fi
rm -rf "$GEN_DIR"

echo "Done. Generated files in x/$MODULE/types/"
