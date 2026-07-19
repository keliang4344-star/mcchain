# 开发环境搭建（MC 公链）

本文档记录本仓库的工具链约定与踩坑点，避免重复踩雷。

---

## 1. 工具链位置（铁律）

| 组件 | 路径 | 说明 |
|------|------|------|
| Go | `go` | Go 1.22.5，`GOROOT=$GOROOT` |
| GOPATH | `$GOPATH` | 已整体迁移，含 `pkg/mod` / `bin` |
| GOMODCACHE | `$GOMODCACHE` | 模块缓存 |
| protoc | `protoc` | 手动协议生成 |
| 插件 | `$GOPATH/bin/protoc-gen-gocosmos`、`protoc-gen-grpc-gateway` | protoc 调用 |

每次打开新终端，先把以下加入 PATH：

```bash
export GOROOT=/usr/local/go
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
```

---

## 2. 编译与测试

```bash
# 全量编译
go build ./...

# 运行测试（建议逐模块，避免一次性全量耗时）
go test ./x/edgeai/... ./x/depin/... ./x/tokenomics/...

# 覆盖率
go test ./x/edgeai/... -cover
```

CI 门禁见 `.github/workflows/ci.yml`（`go test ./...` + `-cover`，关键模块目标 ≥ 70%）。

---

## 3. 协议代码生成（**必须手动 protoc**）

**`ignite generate proto-go` 不可用**：Ignite 会下载一个临时 protoc 执行，报 `executable file not found in %PATH%`。

改用 **手动 protoc**（已验证可用）。示例（在 `mcchain` 仓库根目录执行）：

```bash
export GOROOT=/usr/local/go
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH

# 依赖 proto 路径（版本按 go.mod 中实际锁定版本对齐）
cosmosSdkVer="v0.47.3"
deps=(
  "-I" "proto"
  "-I" "${GOMODCACHE}/github.com/cosmos/cosmos-sdk@${cosmosSdkVer}/proto"
  "-I" "${GOMODCACHE}/github.com/cosmos/gogoproto@v1.4.10"
  "-I" "${GOMODCACHE}/github.com/cosmos/cosmos-proto@v1.0.0-beta.2/proto"
  "-I" "${GOMODCACHE}/github.com/cosmos/gogo/googleapis@v1.4.1"
  "-I" "${GOMODCACHE}/github.com/ignite-hq/cli/ignite/pkg/protoc/data/include"
)

protoc "${deps[@]}" \
  --gocosmos_out="plugins=interfacetype+grpc,Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types,Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations:.",\
  --grpc-gateway_out="." \
  proto/mcchain/<module>/<file>.proto
```

生成文件落在 `./mcchain/x/<module>/`（需按需要 move 到 `x/<module>/` 对应包目录）。

> 若新增模块级 proto（如 edgeai 状态改 protobuf），同样走此流程；grpc-gateway 用 `--grpc-gateway_out`。

### 3.1 protoc-gen-gocosmos 缺失时的重建

`$GOPATH/bin/protoc-gen-gocosmos` 在某些会话中会被清掉，导致 `protoc ... --gocosmos_out` 报
`protoc-gen-gocosmos 不是内部或外部命令`。直接从模块缓存源码重建即可（gogoproto 已在 go.mod 锁定）：

```bash
# 在 mcchain 仓库根目录
go build -o protoc-gen-gocosmos github.com/cosmos/gogoproto/protoc-gen-gocosmos
# 然后 protoc 用 --plugin 显式指向刚生成的 exe
protoc -I proto -I "$GOMODCACHE/github.com/cosmos/gogoproto@v1.4.10" \
  --plugin=protoc-gen-gocosmos=$PWD/protoc-gen-gocosmos \
  --gocosmos_out="...:." proto/mcchain/<module>/<file>.proto
# 生成后把 ./mcchain/x/<module>/<file>.pb.go 移到 x/<module>/，再删掉临时 ./mcchain 目录
```

注意：proto 中 `option (gogoproto.goproto_stringer) = false;` 会让新版 gocosmos 生成的
`xxx_messageInfo.Marshal` 因缺 `String()` 而不满足 `proto.Message` 接口 → 编译失败。
**状态 message 不要加 `goproto_stringer=false`**，保留默认生成的 `String()` 即可。

---

## 4. 常用命令

```bash
# 本地单节点
mcchaind init mynode --chain-id mcchain-1
mcchaind keys add alice
mcchaind start

# 链下预言机签名服务（默认读 MC_ORACLE_PRIVKEY / MC_ORACLE_PUBKEY 环境变量）
mcchaind oracle

# 事件订阅
event-subscriber
```

---

## 5. 已知限制

- `ignite chain serve` / `ignite generate proto-go` 不可用 → 用 `go build` + 手动 protoc。
- 前端 `web/` 依赖 `cosmjs-bundle.js`，构建需 Node 环境（见 web 目录 README）。
- 团队多签公钥/地址在 `x/tokenomics/types` 中由 `scripts/gen_team_keys` 生成，主网前须替换为真实公钥。
