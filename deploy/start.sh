#!/usr/bin/env bash
#
# MobileChain 主网生产启动脚本
#
# 适用场景：脱离 systemd 的手动/容器/单节点启动入口。
# 主网生产推荐走 deploy/mcchaind.service（systemd）；本脚本是
# 等价能力的「无 systemd 入口」，与 systemd unit 共享同一套闸门
# （MC_REQUIRE_REAL_GENESIS_KEYS=1、MC_ORACLE_PUBKEY、MC_FOUNDATION_*_PUBKEY）。
#
# 用法：
#   ./deploy/start.sh                          # 默认 ~/.mcchain，闸门齐备
#   ./deploy/start.sh --home /var/lib/mcchain  # 自定义 home
#   ./deploy/start.sh --foreground             # 不后台化（容器/调试用）
#   ./deploy/start.sh --stop                   # 用 .pid 停
#   ./deploy/start.sh --status                 # 查询进程状态
#   ./deploy/start.sh --help
#
# 环境变量（与 deploy/mainnet.env.example 对齐）：
#   MC_REQUIRE_REAL_GENESIS_KEYS   主网密钥闸门（必须 1）
#   MC_ORACLE_PUBKEY               TeeOracle 公钥（base64 33 字节）
#   MC_FOUNDATION_EARLY_DEV_PUBKEY 基金会早期开发公钥
#   MC_FOUNDATION_OPS_PUBKEY       基金会运营公钥
#   MC_FOUNDATION_VESTING_PUBKEY   基金会锁定公钥
#   HOME_DIR                       节点 home（也可用 --home 覆盖）
#   LOG_DIR                        日志目录（默认 $HOME_DIR/../logs）
#   MCCHAIND                       mcchaind 二进制路径（默认自动解析）
#
# 设计原则：
#   · 启动前 fail-fast 自检：二进制存在、home 合规、闸门齐备；任何一项缺失
#     都拒绝启动并打印修法。
#   · 日志走 logrotate 友好的文件（每次重启归档一次），不写到 stdout。
#   · 进程信号传递：trap 转发 SIGTERM 给 mcchaind，优雅退出。
#   · pid 文件：每次启动前清理过期 pid，pid 由 mcchaind 自身的进程接管。
#
set -euo pipefail

# ── 默认参数 ─────────────────────────────────────────────────────────────
HOME_DIR="${HOME_DIR:-$HOME/.mcchain}"
LOG_DIR="${LOG_DIR:-$(dirname "$HOME_DIR")/logs}"
MCCHAIND="${MCCHAIND:-}"
DAEMONIZE=true

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() { sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0; }
log()  { printf '\033[1;34m[start]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[start]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[start][FATAL]\033[0m %s\n' "$*" >&2; exit 1; }

# ── 参数解析 ─────────────────────────────────────────────────────────────
while (( $# )); do
  case "$1" in
    --home)        HOME_DIR="$2"; shift 2;;
    --foreground)  DAEMONIZE=false; shift;;
    --stop)        shift; ACTION=stop;;
    --status)      shift; ACTION=status;;
    --help|-h)     usage;;
    *)             die "未知参数: $1（用 --help 查看用法）";;
  esac
done

resolve_bin() {
  if [[ -n "$MCCHAIND" ]]; then
    [[ -x "$MCCHAIND" ]] || die "MCCHAIND=$MCCHAIND 不存在或不可执行"
    return
  fi
  # 优先级：环境变量 → 仓库 build 目录 → PATH
  for c in "$REPO_ROOT/build/mcchaind" "$REPO_ROOT/build/mcchaind.exe"; do
    [[ -x "$c" ]] && { MCCHAIND="$c"; return; }
  done
  MCCHAIND="$(command -v mcchaind || true)"
  [[ -n "$MCCHAIND" ]] || die "未找到 mcchaind：编译 'go build -o build/mcchaind ./cmd/mcchaind' 或设置 MCCHAIND"
}

win() {
  # git-bash 下把 /e/foo 转成 E:/foo（原生 PE 程序对 MSYS 路径处理不统一）
  if command -v cygpath >/dev/null 2>&1; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

# ── 自检 ─────────────────────────────────────────────────────────────────
self_check() {
  log "启动前自检（home=$HOME_DIR）"

  # [1] home 必须是已 init 的节点（genesis.json 存在）
  [[ -f "$HOME_DIR/config/genesis.json" ]] || die "$HOME_DIR/config/genesis.json 不存在：先 ./deploy/init.sh 建链，或 ./scripts/devnet.sh init 建本地链"

  # [2] 闸门齐备（仅在 mainnet chain-id 下严格，dev/test 允许缺失）
  local cid; cid="$(win "$HOME_DIR")"
  local chain_id
  chain_id=$(grep -oE '"chain_id"\s*:\s*"[^"]+"' "$HOME_DIR/config/genesis.json" | head -1 | sed 's/.*"chain_id"\s*:\s*"\([^"]*\)".*/\1/')
  if [[ "$chain_id" == *mainnet* ]]; then
    local missing=0
    for v in MC_REQUIRE_REAL_GENESIS_KEYS MC_ORACLE_PUBKEY \
             MC_FOUNDATION_EARLY_DEV_PUBKEY MC_FOUNDATION_OPS_PUBKEY \
             MC_FOUNDATION_VESTING_PUBKEY; do
      if [[ -z "${!v:-}" ]]; then
        warn "主网 chain-id 缺关键 env: $v"
        missing=$((missing + 1))
      fi
    done
    if (( missing > 0 )); then
      die "$missing 道 fail-closed 闸门缺失（详见 deploy/mainnet.env.example）。主网链不会启动。"
    fi
  else
    # devnet / testnet：未设 MC_ORACLE_PUBKEY 时自动加 MC_ORACLE_ALLOW_SOFT=1
    if [[ -z "${MC_ORACLE_PUBKEY:-}" && "${MC_ORACLE_ALLOW_SOFT:-}" != "1" ]]; then
      warn "未检测到 MC_ORACLE_PUBKEY；非主网自动注入 MC_ORACLE_ALLOW_SOFT=1（仅本地/测试）"
      export MC_ORACLE_ALLOW_SOFT=1
    fi
  fi
  log "闸门齐备（chain-id=$chain_id）"
}

# ── stop / status ────────────────────────────────────────────────────────
cmd_stop() {
  local pidfile="$HOME_DIR/.mcchaind.pid"
  if [[ ! -f "$pidfile" ]]; then warn "$pidfile 不存在，进程可能未运行"; return 0; fi
  local pid; pid="$(cat "$pidfile")"
  if kill -0 "$pid" 2>/dev/null; then
    log "停止 mcchaind（pid $pid）"
    kill "$pid" && sleep 2
    kill -0 "$pid" 2>/dev/null && kill -9 "$pid" 2>/dev/null || true
    log "已停止"
  else
    warn "pid $pid 已不存在"
  fi
  rm -f "$pidfile"
}

cmd_status() {
  local pidfile="$HOME_DIR/.mcchaind.pid"
  if [[ -f "$pidfile" ]] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    log "mcchaind 正在运行（pid $(cat "$pidfile")，home=$HOME_DIR）"
    return 0
  fi
  log "mcchaind 未运行（home=$HOME_DIR）"
  return 1
}

# ── start ─────────────────────────────────────────────────────────────────
cmd_start() {
  resolve_bin
  self_check

  # 防重复启动
  local pidfile="$HOME_DIR/.mcchaind.pid"
  if [[ -f "$pidfile" ]] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    die "mcchaind 已在运行（pid $(cat "$pidfile")）；先用 --stop"
  fi
  rm -f "$pidfile"

  mkdir -p "$LOG_DIR"
  local stamp; stamp="$(date +%Y%m%d-%H%M%S)"
  # 当前日志 + 归档（rotate by restart）
  local logfile="$LOG_DIR/mcchaind-$stamp.log"
  ln -sfn "$logfile" "$LOG_DIR/mcchaind-current.log"
  # 清理 7 天前的归档
  find "$LOG_DIR" -maxdepth 1 -name 'mcchaind-*.log' -mtime +7 -delete 2>/dev/null || true

  log "启动 mcchaind → $logfile"
  log "binary: $MCCHAIND"
  log "home:   $HOME_DIR"
  log "command: $MCCHAIND start --home $(win "$HOME_DIR")"

  if $DAEMONIZE; then
    # nohup + disown：父进程退出后不被 SIGHUP；pid 由 $! 捕获
    nohup "$MCCHAIND" start --home "$(win "$HOME_DIR")" >"$logfile" 2>&1 &
    local pid=$!
    echo "$pid" > "$pidfile"
    disown "$pid" 2>/dev/null || true
    sleep 2
    if kill -0 "$pid" 2>/dev/null; then
      log "已后台启动（pid $pid）；日志：$logfile"
      log "运维命令：tail -F $LOG_DIR/mcchaind-current.log  /  ./deploy/start.sh --status  /  ./deploy/start.sh --stop"
    else
      die "进程已退出，请检查日志：$logfile"
    fi
  else
    # 前台：容器或调试；trap 转发 SIGTERM / SIGINT 让 mcchaind 优雅退出
    trap 'kill -TERM "$child" 2>/dev/null; wait "$child" || true; exit' TERM INT
    "$MCCHAIND" start --home "$(win "$HOME_DIR")" &
    local child=$!
    wait "$child"
  fi
}

# ── 分派 ─────────────────────────────────────────────────────────────────
case "${ACTION:-start}" in
  start)  cmd_start;;
  stop)   cmd_stop;;
  status) cmd_status;;
  *)      die "未知动作: ${ACTION:-}";;
esac