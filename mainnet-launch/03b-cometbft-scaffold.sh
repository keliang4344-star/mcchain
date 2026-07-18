#!/usr/bin/env bash
#
# MobileChain CometBFT 接入脚本（修正版 03b）
# ============================================================
# 修复原 03-ignite-scaffold.sh 的三处硬伤：
#   1) scaffold module depin 的 --dep module/token 在 cosmos-sdk 中不存在
#      -> 改为 --dep bank（DePIN 奖励发放依赖 bank 模块做代币转移）
#   2) keeper 迁移不能只追加注释 -> 把蓝本逻辑「逐字」归档到
#      <chain>/legacy-blueprint/（不参与编译），保证后续迁 store 零丢失
#   3) genesis 不能只改 block_interval -> 注入 chainID / bech32 前缀等
#      主网身份参数（见 01-主网参数定稿.md）
#
# 依赖：Go 1.22+ / Ignite CLI v0.27.2 / 网络（拉 cosmos-sdk）
# 用法：
#   bash 03b-cometbft-scaffold.sh            # 全流程
#   bash 03b-cometbft-scaffold.sh --check    # 仅检查环境
#   bash 03b-cometbft-scaffold.sh --scaffold # 仅生成骨架+配置（不 build）
#
set -euo pipefail

# ========== 路径 ==========
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CHAIN_NAME="mcchain"
PREFIX="mc"
OUT_DIR="${OUT_DIR:-$SCRIPT_DIR/../${CHAIN_NAME}}"   # -> $HOME/mcchain
KEEPER_SRC="$SCRIPT_DIR/../cosmos"                   # -> $HOME/cosmos
IGNITE_BIN="$HOME/go/bin/ignite"

# ========== 主网参数（来自 01-主网参数定稿.md） ==========
CHAIN_ID="mcchain-1"
DENOM="umc"              # 主网小数位 6，符号 umc（迭代阶段再接 bond denom）
GO_VERSION_MIN="1.22"
IGNITE_VERSION="v0.27.2"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[03b]${NC} $*"; }
warn() { echo -e "${YELLOW}[03b WARN]${NC} $*"; }
err()  { echo -e "${RED}[03b ERR]${NC} $*" >&2; exit 1; }

# ========== 环境检查 ==========
check_env() {
  log "检查环境依赖..."
  if ! command -v go &>/dev/null; then
    err "未找到 Go，请先装 Go ${GO_VERSION_MIN}+（应在 /mnt/d/go）"
  fi
  log "Go: $(go version | awk '{print $3}')"

  if [ ! -x "$IGNITE_BIN" ]; then
    err "未找到 ignite：期望 $IGNITE_BIN。请先装 ignite v0.27.2 发布版到 D 盘 Go bin"
  fi
  log "Ignite: $("$IGNITE_BIN" version 2>/dev/null | head -1 || echo unknown)"

  if [ ! -d "$KEEPER_SRC/x/depin" ]; then
    err "未找到 cosmos 蓝本： $KEEPER_SRC"
  fi
  log "蓝本: $KEEPER_SRC"
  log "输出链: $OUT_DIR"
}

# ========== 生成链骨架 ==========
scaffold_chain() {
  log "scaffold chain: $CHAIN_NAME (prefix=$PREFIX)"
  if [ -d "$OUT_DIR" ]; then
    warn "$OUT_DIR 已存在，跳过 scaffold（如需重建先删该目录）"
    return 0
  fi
  ( cd "$SCRIPT_DIR" && "$IGNITE_BIN" scaffold chain "$CHAIN_NAME" --address-prefix "$PREFIX" )
  # scaffold 会在当前目录生成 $CHAIN_NAME/，挪到 OUT_DIR
  if [ "$OUT_DIR" != "$SCRIPT_DIR/$CHAIN_NAME" ] && [ -d "$SCRIPT_DIR/$CHAIN_NAME" ]; then
    mv "$SCRIPT_DIR/$CHAIN_NAME" "$OUT_DIR"
  fi
  ( cd "$OUT_DIR" && git init -q 2>/dev/null || true )
  log "链骨架已生成: $OUT_DIR"
}

# ========== 生成自定义模块（修复 --dep） ==========
scaffold_modules() {
  cd "$OUT_DIR"
  log "scaffold 模块（--dep bank 修复）..."
  echo y | "$IGNITE_BIN" scaffold module depin --dep bank
  echo y | "$IGNITE_BIN" scaffold module phonenode
  log "模块骨架已生成"
}

# ========== 生成消息类型 ==========
scaffold_messages() {
  cd "$OUT_DIR"
  log "scaffold 消息类型..."
  echo y | "$IGNITE_BIN" scaffold message register-device address model os --module depin
  echo y | "$IGNITE_BIN" scaffold message attest-device address challenge signature --module depin
  echo y | "$IGNITE_BIN" scaffold message submit-contribution task-id task-type score --module depin
  echo y | "$IGNITE_BIN" scaffold message submit-state-proof root leaf index proof --module phonenode
  echo y | "$IGNITE_BIN" scaffold message register-node address model os role --module phonenode
  log "消息类型已生成"
}

# ========== 归档蓝本（零丢失，不参与编译） ==========
preserve_blueprint() {
  cd "$OUT_DIR"
  log "归档 cosmos 蓝本到 legacy-blueprint/（供后续迁 store 逐字参考）..."
  rm -rf legacy-blueprint
  mkdir -p legacy-blueprint
  cp -r "$KEEPER_SRC/." legacy-blueprint/
  # 写一份说明，明确内存 keeper -> store keeper 是迭代任务
  cat > legacy-blueprint/README.md <<'MDEOF'
# 蓝本归档（legacy-blueprint）

本目录逐字保存迁移前的 cosmos 内存实现蓝本，**不参与编译**。
业务 keeper 当前为 `sync.RWMutex + map` 内存实现，接口已对齐 Cosmos SDK Keeper。

迁 store 任务清单（迭代，由开发团队执行）：
1. x/depin/keeper/keeper.go  — 把 legacy-blueprint/x/depin/keeper.go 的业务方法
   （RegisterDevice / SubmitAndReward / DeviceReward / AllContributions 等）
   改为基于 collections.Store 的持久化实现。
2. x/phonenode/keeper/keeper.go — 同上，并保留 VerifyMerkleProof / BuildMerkleRoot
   等纯函数（与链上状态无关，可直接复用）。
3. 奖励引擎依赖 mcchain-staging/depin（ComputeReward / IsValidTaskType），
   迁 store 时一并纳入 go.mod 或直接内联。
4. denom 接 umc：在 app params 把 staking bond denom 改为 umc（小数位 6）。
MDEOF
  log "蓝本已归档: $OUT_DIR/legacy-blueprint"
}

# ========== 注入主网身份参数 ==========
configure_genesis() {
  cd "$OUT_DIR"
  # bech32 前缀已由 scaffold --address-prefix mc 写入 app.go (AccountAddressPrefix="mc")
  # config.yml 在 v0.27.2 无 bech32Prefix 字段，故只注入 chainID。
  log "注入主网 chainID=$CHAIN_ID 到 config.yml ..."
  if [ -f config.yml ]; then
    if grep -q '^chainID:' config.yml; then
      sed -i "s/^chainID:.*/chainID: $CHAIN_ID/" config.yml
    else
      sed -i "1i chainID: $CHAIN_ID" config.yml
    fi
    log "config.yml 已设 chainID=$CHAIN_ID（bech32 前缀 mc 由 app.go 决定，无需改 config.yml）"
  else
    warn "未找到 config.yml，请手动设 chainID=$CHAIN_ID"
  fi
}

# ========== 构建 ==========
build_chain() {
  cd "$OUT_DIR"
  log "构建链（首次会拉取 cosmos-sdk 依赖，约几分钟）..."
  "$IGNITE_BIN" chain build || go build ./... || err "构建失败"
  log "构建完成: $(pwd)/bin/${CHAIN_NAME}d"
}

# ========== 主流程 ==========
main() {
  case "${1:-}" in
    --check)     check_env; exit 0 ;;
    --scaffold)
      check_env; scaffold_chain; scaffold_modules; scaffold_messages
      preserve_blueprint; configure_genesis
      log "骨架+配置完成，下一步手动执行 build"; exit 0 ;;
  esac
  check_env
  scaffold_chain
  scaffold_modules
  scaffold_messages
  preserve_blueprint
  configure_genesis
  build_chain
  log "========== 03b 完成 =========="
  log "链代码: $OUT_DIR"
  log "二进制: $OUT_DIR/bin/${CHAIN_NAME}d"
  log "验证 init : $OUT_DIR/bin/${CHAIN_NAME}d init mynode --chain-id $CHAIN_ID"
  log "验证 start: $OUT_DIR/bin/${CHAIN_NAME}d start"
  log "下一步（迭代）：legacy-blueprint/README.md 中的迁 store 任务"
}

main "$@"
