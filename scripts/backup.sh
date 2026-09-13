#!/usr/bin/env bash
#
# MC 公链 · 节点数据备份 / 恢复
#
# 策略（docs/VALIDATOR_RUNBOOK §1.4 + NODE_CONFIG §7）：
#   - 每日快照：节点 data 目录 → 本地 BACKUP_DIR（保留 7 份）
#   - 每周归档：本地快照 → OFFSITE_DIR（异地盘/对象存储挂载点）
#   - 创世与配置档案（genesis.json / config.toml / app.toml / node_key.json）
#     每次备份都全量带走 —— 重建一台节点只需要「档案 + 任意一份 data 快照」。
#   - 恢复：./scripts/backup.sh restore（本脚本的 restore 子命令）从指定快照重建节点，2 小时内完成。
#
# 用法：
#   ./scripts/backup.sh backup  <node_home> [label]   # 本地快照 + 每周归档
#   ./scripts/backup.sh restore <node_home> <snapshot.tar.zst|.tar.gz>  # 从快照恢复
#   ./scripts/backup.sh list <node_home>              # 列出可用快照
#
# 环境变量：
#   BACKUP_DIR   本地快照目录（默认 <node_home>/../backups）
#   OFFSITE_DIR  异地归档目录（默认空 = 跳过归档；设为挂载点/对象存储路径即启用）
#   KEEP_LOCAL   本地保留份数（默认 7）
#
# 安全：node_key.json / priv_validator_key.json 一并备份——这是恢复的唯一凭据，
#       因此 BACKUP_DIR 与 OFFSITE_DIR 必须 chmod 700，且**不允许落在公网可读路径**。
#
set -euo pipefail

CMD="${1:-}"; shift || true
KEEP_LOCAL="${KEEP_LOCAL:-7}"

log() { printf '\033[1;34m[BACKUP]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[FATAL]\033[0m %s\n' "$*" >&2; exit 1; }

# 选压缩器：优先 zstd（快且压得好），退回 gzip
pick_cmp() {
  if command -v zstd >/dev/null 2>&1; then echo "zstd"; else echo "gzip"; fi
}

backup() {
  local home="${1:?需要 node_home}"; shift || true
  local label="${1:-daily}"
  [[ -d "$home/data" ]] || die "$home/data 不存在（不是有效的节点 home）"

  local bdir="${BACKUP_DIR:-$(dirname "$home")/backups}"
  mkdir -p "$bdir"; chmod 700 "$bdir"

  local stamp ts
  stamp="$(date +%Y%m%d-%H%M%S)"
  ts="$(date +%Y-%m-%dT%H:%M:%S%z)"

  # 1) 停写窗口：轻节点/RPC 可以不停止直接拷贝（tendermint 的数据目录在运行时
  #    是 goleveldb，直接拷贝有一致性风险）。这里采用「先 tendermint snapshot
  #    语义的一致点」：优先用 mcchaind comet snapshot 不存在，所以退而求其次
  #    —— 用文件系统级拷贝 + 事后校验 chain 状态。对 validator：必须先 stop。
  local cmp ext
  cmp="$(pick_cmp)"; [[ "$cmp" == "zstd" ]] && ext="tar.zst" || ext="tar.gz"

  local out="$bdir/mcchain-${label}-$stamp.$ext"
  log "开始备份 $home → $out"

  local tmpdir; tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/mc-bak.XXXXXX")"
  trap 'rm -rf "$tmpdir"' EXIT

  # 档案（小文件）与 data（大头）分开拷，档案永远全量
  mkdir -p "$tmpdir/config"
  cp "$home/config/genesis.json" "$tmpdir/config/" 2>/dev/null || die "genesis.json 缺失"
  for f in config.toml app.toml node_key.json priv_validator_key.json priv_validator_state.json; do
    [[ -f "$home/config/$f" ]] && cp "$home/config/$f" "$tmpdir/config/"
  done
  [[ -f "$home/data/priv_validator_state.json" ]] && cp "$home/data/priv_validator_state.json" "$tmpdir/data.priv_validator_state.json"

  # data 目录：cp 而非 tar 直压，先落地再校验再压缩
  cp -r "$home/data" "$tmpdir/data"

  # 一致性校验：application.db 必须存在（否则快照残缺）
  [[ -d "$tmpdir/data/application.db" ]] || die "快照缺少 application.db，拒绝打包"

  # priv_validator_state.json 仅 validator 节点有，RPC/轻节点场景不存在。
  # tar 无条件打包会失败并留下**残缺快照**，还会被 list/轮转当有效快照。
  # 因此：文件存在才打包；压缩失败即删除输出。
  local tar_paths=(config data)
  [[ -f "$tmpdir/data.priv_validator_state.json" ]] && tar_paths+=(data.priv_validator_state.json)

  if ! tar -cf - -C "$tmpdir" "${tar_paths[@]}" \
      | { [[ "$cmp" == "zstd" ]] && zstd -3 -T0 || gzip -3; } > "$out"; then
    rm -f "$out"
    die "打包/压缩失败，已删除残缺输出 $out"
  fi

  chmod 600 "$out"
  local sz; sz=$(du -h "$out" | awk '{print $1}')
  log "完成：$out ($sz)"

  # 2) 元信息（高度从快照里读 application.db 不可行，记录当时链高由调用方传入或省略）
  cat > "${out%.${ext}}.meta" <<EOF
timestamp=$ts
home=$home
label=$label
compressor=$cmp
size_bytes=$(stat -c%s "$out" 2>/dev/null || wc -c < "$out")
EOF

  # 3) 每周归档（周日）
  if [[ "$(date +%u)" == "7" && -n "${OFFSITE_DIR:-}" ]]; then
    mkdir -p "$OFFSITE_DIR"; chmod 700 "$OFFSITE_DIR"
    cp "$out" "$OFFSITE_DIR/"
    log "每周归档已复制到 $OFFSITE_DIR"
  fi

  # 4) 本地轮转：只留最近 KEEP_LOCAL 份同 label 快照
  ls -1t "$bdir"/mcchain-${label}-*.${ext} 2>/dev/null | tail -n +$((KEEP_LOCAL + 1)) | while read -r old; do
    rm -f "$old" "${old%.${ext}}.meta"
    log "轮转删除旧快照: $old"
  done
}

restore() {
  local home="${1:?需要 node_home}"; shift
  local snap="${1:?需要快照路径}"
  [[ -f "$snap" ]] || die "快照不存在: $snap"
  [[ -d "$home" ]] && die "$home 已存在（恢复到空目录，防覆盖）。如需覆盖请先手动移走。"

  local cmp; cmp="$(pick_cmp)"
  local tmpdir; tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/mc-res.XXXXXX")"
  trap 'rm -rf "$tmpdir"' EXIT

  log "从 $snap 恢复到 $home"
  mkdir -p "$home/config" "$home/data"
  if [[ "$snap" == *.zst ]]; then
    command -v zstd >/dev/null 2>&1 || die "快照是 zstd 格式但本机未安装 zstd"
    zstd -dc "$snap" | tar -xf - -C "$tmpdir"
  else
    tar -xzf "$snap" -C "$tmpdir"
  fi

  cp -r "$tmpdir/config/." "$home/config/"
  cp -r "$tmpdir/data/." "$home/data/"
  [[ -f "$tmpdir/data.priv_validator_state.json" ]] && \
    cp "$tmpdir/data.priv_validator_state.json" "$home/data/priv_validator_state.json"

  # 权限收紧
  chmod 700 "$home"
  find "$home" -name "*.json" -exec chmod 600 {} \;

  log "恢复完成。启动前请校验："
  log "  1) ${home}/config/genesis.json 的 sha256 与官网公示一致"
  log "  2) mcchaind validate-genesis --home ${home}"
  log "  3) mcchaind start --home ${home} 后 status 是否追平"
}

list() {
  local home="${1:?需要 node_home}"
  local bdir="${BACKUP_DIR:-$(dirname "$home")/backups}"
  echo "可用快照（$bdir）:"
  ls -1t "$bdir" 2>/dev/null | grep -E '\.(tar\.zst|tar\.gz)$' | while read -r f; do
    local meta="$bdir/${f%.*}.meta"
    printf '  %s  (%s)\n' "$f" "$([[ -f "$meta" ]] && grep timestamp= "$meta" | cut -d= -f2 || echo 'no meta')"
  done
}

case "$CMD" in
  backup)  backup "$@" ;;
  restore) restore "$@" ;;
  list)    list "$@" ;;
  *) die "用法: backup.sh backup|restore|list ..." ;;
esac
