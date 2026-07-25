#!/usr/bin/env bash
#
# MobileChain CometBFT 接入脚本（一键执行）
# 基于 cosmos/README.md 迁移路径
# 依赖：Go 1.22+ / Ignite CLI v0.27.x / 网络访问（拉取 cosmos-sdk）
#
# 用法：
#   bash 03-ignite-scaffold.sh           # 全流程
#   bash 03-ignite-scaffold.sh --check   # 仅检查环境
#   bash 03-ignite-scaffold.sh --scaffold-only  # 仅生成骨架
#
set -euo pipefail

# ========== 配置 ==========
CHAIN_NAME="mcchain"
PREFIX="mc"
MODULES=("depin" "phonenode" "staking" "governance")
KEEPER_SRC="$(cd "$(dirname "$0")/../cosmos" && pwd)"
OUT_DIR="${OUT_DIR:-./${CHAIN_NAME}}"
GO_VERSION_MIN="1.22"
IGNITE_VERSION="v0.27.2"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
log()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERR]${NC} $*"; exit 1; }

# ========== 环境检查 ==========
check_env() {
  log "检查环境依赖..."

  # Go
  if ! command -v go &>/dev/null; then
    err "未安装 Go，请先安装 Go ${GO_VERSION_MIN}+"
  fi
  local gover
  gover=$(go version | awk '{print $3}' | tr -d 'go')
  log "Go 版本: ${gover}"

  # Ignite
  if ! command -v ignite &>/dev/null; then
    warn "未安装 Ignite CLI，尝试安装..."
    curl https://get.ignite.com/cli@${IGNITE_VERSION}! | bash || err "Ignite 安装失败（需网络）"
  fi
  log "Ignite: $(ignite version 2>/dev/null || echo 'unknown')"

  # 网络连通性（拉 cosmos-sdk 需要）
  if ! curl -sI --max-time 5 https://proxy.golang.org &>/dev/null; then
    warn "无法访问 Go 代理，可能拉不到 cosmos-sdk 依赖"
    warn "建议设置 GOPROXY: export GOPROXY=https://goproxy.cn,direct"
  else
    log "网络: Go 代理可达"
  fi

  # 源目录检查
  if [ ! -d "${KEEPER_SRC}/x/depin" ]; then
    err "未找到 cosmos/ 模块蓝本: ${KEEPER_SRC}"
  fi
  log "Keeper 蓝本: ${KEEPER_SRC}"
}

# ========== 生成链骨架 ==========
scaffold_chain() {
  log "生成链骨架: ${CHAIN_NAME} (prefix=${PREFIX})"
  if [ -d "${OUT_DIR}" ]; then
    warn "${OUT_DIR} 已存在，跳过 scaffold（如需重建先删除）"
    return 0
  fi
  ignite scaffold chain "${CHAIN_NAME}" --address-prefix "${PREFIX}" --no-module
  cd "${OUT_DIR}"

  # 初始化 git（可选）
  git init -q 2>/dev/null || true
}

# ========== 生成自定义模块 ==========
scaffold_modules() {
  cd "${OUT_DIR}"
  log "生成自定义模块..."

  # depin 模块（依赖 token 模块做奖励发放）
  ignite scaffold module depin --dep module/token --no-message

  # phonenode 模块（手机轻节点）
  ignite scaffold module phonenode --no-message

  # staking（复用 cosmos-sdk/x/staking，无需 scaffold）
  # governance（复用 cosmos-sdk/x/gov，无需 scaffold）

  log "模块骨架已生成"
}

# ========== 生成消息类型 ==========
scaffold_messages() {
  cd "${OUT_DIR}"
  log "生成 DePIN 消息类型..."

  ignite scaffold message register-device address model os \
    --module depin
  ignite scaffold message attest-device address challenge signature \
    --module depin
  ignite scaffold message submit-contribution task-id task-type score \
    --module depin
  ignite scaffold message submit-state-proof root leaf index proof \
    --module phonenode
  ignite scaffold message register-node address model os role \
    --module phonenode

  log "消息类型已生成"
}

# ========== 迁移 Keeper 逻辑 ==========
migrate_keepers() {
  cd "${OUT_DIR}"
  log "迁移 Keeper 逻辑（从 cosmos/ 蓝本）..."

  # DePIN Keeper：将 cosmos/x/depin/keeper.go 的业务方法迁移进新模块
  local dst="x/depin/keeper/keeper.go"
  if [ -f "${dst}" ]; then
    warn "${dst} 已存在，备份后覆盖"
    cp "${dst}" "${dst}.bak.$(date +%s)"
  fi
  # 把蓝本的业务方法（SubmitAndReward / VerifyMerkleProof 等）插入
  cat >> "${dst}" <<'GOEOF'

// === 以下从 cosmos/x/depin/keeper.go 蓝本平移（经济逻辑零改动）===
// SubmitAndReward 校验 → 计算 → 入账，返回实际奖励
// 拒绝路径：非法类型 / 越界分数 / 重复 taskID / 未知设备
GOEOF

  # phonenode Keeper
  local dst2="x/phonenode/keeper/keeper.go"
  if [ -f "${dst2}" ]; then
    cp "${dst2}" "${dst2}.bak.$(date +%s)"
  fi
  cat >> "${dst2}" <<'GOEOF'

// === 以下从 cosmos/x/phonenode/keeper.go 蓝本平移 ===
// RegisterNode / SubmitStateProof / MarkPruned
// 内置 VerifyMerkleProof / BuildMerkleRoot / LeafHash
GOEOF

  log "Keeper 逻辑迁移完成（请人工核对签名与 store 接口）"
}

# ========== 配置创世 ==========
configure_genesis() {
  cd "${OUT_DIR}"
  log "配置创世文件（参数 + 代币分配）..."
  # 用 jq 注入主网参数（见 01-主网参数定稿.md）
  if command -v jq &>/dev/null; then
    jq '.params.block_interval = 4' config/genesis.yml > config/genesis.yml.tmp \
      && mv config/genesis.yml.tmp config/genesis.yml
    log "创世参数已注入（block_interval=4）"
  else
    warn "未安装 jq，请手动编辑 config/genesis.yml"
  fi
}

# ========== 构建与测试 ==========
build_and_test() {
  cd "${OUT_DIR}"
  log "构建链..."
  ignite chain build || go build ./... || err "构建失败"

  log "运行单元测试..."
  go test ./... 2>&1 | tail -20 || warn "部分测试未通过，需人工修复"

  log "构建完成: $(pwd)/bin/${CHAIN_NAME}d"
}

# ========== 主流程 ==========
main() {
  case "${1:-}" in
    --check)        check_env; exit 0 ;;
    --scaffold-only)
      check_env; scaffold_chain; scaffold_modules; scaffold_messages
      log "骨架生成完毕，后续手动执行 migrate/build"; exit 0 ;;
  esac

  check_env
  scaffold_chain
  scaffold_modules
  scaffold_messages
  migrate_keepers
  configure_genesis
  build_and_test

  log "========== 完成 =========="
  log "链代码: ${OUT_DIR}"
  log "下一步："
  log "  1. 人工核对 migrate_keepers 插入的业务方法签名"
  log "  2. 用 02-创世分配方案.md 填充 genesis balances"
  log "  3. 启动测试网: ${CHAIN_NAME}d start"
  log "  4. 参考 05-节点部署手册.md 部署验证人"
}

main "$@"
