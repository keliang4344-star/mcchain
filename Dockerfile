# MobileChain (MC) 主网节点镜像
# 多阶段构建：builder 阶段在 Linux 编译 mcchaind，runtime 用精简 Debian。
FROM golang:1.22-bullseye AS builder

WORKDIR /src
# 先拉依赖，利用层缓存
COPY go.mod go.sum ./
RUN go mod download
# 拷贝源码并编译静态二进制
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/mcchaind ./cmd/mcchaind

FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl jq vim-tiny \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /root
COPY --from=builder /out/mcchaind /usr/local/bin/mcchaind
# p2p / rpc / api
EXPOSE 26656 26657 1317
ENTRYPOINT ["mcchaind"]
CMD ["start"]
