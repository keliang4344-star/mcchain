#!/usr/bin/env bash
# ============================================================================
# MC 公链 · 发布就绪门禁（mainnet readiness gate）
#
# 设计原则（修复「全绿误导」）：
#   1. 关键项失败 = 非零退出；关键项 SKIP 在 --mainnet 模式下同样视为失败。
#   2. 绝不用 `|| true` 吞掉关键检查的退出码；只有显式标注为 info 的检查允许失败。
#   3. 输出机器可读 JSON 报告（commit、工具版本、测试结果、二进制/创世哈希、
#      时间戳），供审计与发布存档；只有全部必需项通过才打印 `MAINNET GO`。
#
# 用法：
#   bash scripts/mainnet_readiness.sh check-local            # 本地快速门禁
#   bash scripts/mainnet_readiness.sh check-ci               # CI 全量门禁
#   bash scripts/mainnet_readiness.sh check-genesis          # 创世校验
#   bash scripts/mainnet_readiness.sh check-artifact         # 制品哈希/SBOM/签名
#   bash scripts/mainnet_readiness.sh all --mainnet          # 发布前 Go/No-Go
#
# 退出码：0 = 全部必需项通过；1 = 存在失败或关键 SKIP；2 = 用法/环境错误
# ============================================================================
set -uo pipefail
cd "$(dirname "$0")/.."

MODE="${1:-check-local}"
MAINNET=0
for arg in "$@"; do
  case "$arg" in
    --mainnet) MAINNET=1 ;;
  esac
done
case "$MODE" in
  check-local|check-ci|check-genesis|check-artifact|all) ;;
  *) echo "usage: $0 {check-local|check-ci|check-genesis|check-artifact|all} [--mainnet]" >&2; exit 2 ;;
esac

REPORT_DIR="${REPORT_DIR:-build/readiness}"
mkdir -p "$REPORT_DIR"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS="$REPORT_DIR/.results-$STAMP.tsv"
: > "$RESULTS"

# 工具链前置检查：按模式校验所需工具，缺失时以环境错误退出（退出码 2），
# 而不是让每个子检查各自失败、最后报告一堆无法归因的 FAIL。
need_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[FATAL] required tool '$1' not found in PATH (mode=$MODE)" >&2
    exit 2
  fi
}
need_tool git
case "$MODE" in
  check-local|check-ci|check-artifact|all) need_tool go ;;
esac
case "$MODE" in
  check-ci|check-genesis|all) need_tool python3 ;;
esac

FAILURES=0
REQUIRED_SKIPS=0

# record <name> <pass|fail|skip|info> <required:0|1> <detail>
record() {
  local name="$1" status="$2" required="$3" detail="$4"
  printf '%s\t%s\t%s\t%s\n' "$name" "$status" "$required" "${detail//$'\n'/ }" >> "$RESULTS"
  case "$status" in
    fail) FAILURES=$((FAILURES + 1)); echo "  [FAIL] $name — $detail" ;;
    skip)
      if [[ "$required" == "1" && "$MAINNET" == "1" ]]; then
        REQUIRED_SKIPS=$((REQUIRED_SKIPS + 1)); echo "  [SKIP→FAIL] $name — $detail"
      else
        echo "  [SKIP] $name — $detail"
      fi
      ;;
    info) echo "  [INFO] $name — $detail" ;;
    *)    echo "  [PASS] $name" ;;
  esac
}

# run_check <name> <required> <command...>
run_check() {
  local name="$1" required="$2"; shift 2
  local out rc
  out="$("$@" 2>&1)"; rc=$?
  if [[ $rc -eq 0 ]]; then
    record "$name" pass "$required" "ok"
  else
    record "$name" fail "$required" "$(printf '%s' "$out" | tail -n 5)"
  fi
  return 0
}

echo "== MC mainnet readiness: mode=$MODE mainnet=$MAINNET =="

# ---------------------------------------------------------------------------
# 1) 构建与测试
# ---------------------------------------------------------------------------
if [[ "$MODE" == "check-local" || "$MODE" == "check-ci" || "$MODE" == "all" ]]; then
  echo "-- build --"
  run_check "go-build" 1 go build ./...

  echo "-- tests (full) --"
  run_check "go-test-all" 1 go test ./...

  echo "-- race detector --"
  if [[ "$MODE" == "check-local" ]]; then
    record "go-test-race" skip 0 "check-local 跳过 race（CI 的 check-ci 模式必须执行）"
  else
    run_check "go-test-race" 1 go test -race -count=1 ./x/... ./app/... ./internal/...
  fi

  echo "-- fuzz (short budget) --"
  FUZZ_TIME="${FUZZ_TIME:-5s}"
  fuzz_failed=0
  fuzz_ran=0
  while IFS= read -r target; do
    pkg="./${target%%:*}"
    fn="${target##*:}"
    fuzz_ran=$((fuzz_ran + 1))
    if ! go test "$pkg" -run '^$' -fuzz "^${fn}$" -fuzztime "$FUZZ_TIME" >/dev/null 2>&1; then
      fuzz_failed=$((fuzz_failed + 1))
    fi
  done < <(grep -rHn --include='*_test.go' '^func Fuzz' x/ internal/ 2>/dev/null | sed -E 's#/[^/]+_test\.go:.*func (Fuzz[A-Za-z0-9_]+).*#:\1#' | sed 's#^\./##')
  if [[ $fuzz_ran -eq 0 ]]; then
    record "fuzz" skip 1 "未发现 Fuzz 目标"
  elif [[ $fuzz_failed -eq 0 ]]; then
    record "fuzz" pass 1 "ran $fuzz_ran target(s) for $FUZZ_TIME each"
  else
    record "fuzz" fail 1 "$fuzz_failed/$fuzz_ran fuzz target(s) failed"
  fi

  echo "-- docs consistency --"
  if command -v python3 >/dev/null 2>&1; then
    run_check "docs-consistency" 1 python3 scripts/check_docs_consistency.py
  else
    record "docs-consistency" skip 1 "python3 not found"
  fi

  echo "-- worktree clean (tracked + untracked) --"
  dirty="$(git status --porcelain 2>/dev/null)"
  if [[ -z "$dirty" ]]; then
    record "worktree-clean" pass 1 "clean"
  else
    record "worktree-clean" fail 1 "$(printf '%s' "$dirty" | head -n 5 | tr '\n' ' ')"
  fi

  echo "-- TODO scan (informational) --"
  todo_hits="$(grep -rn 'TODO\|FIXME\|XXX' --include='*.go' x/ app/ cmd/ internal/ 2>/dev/null | grep -v '_test.go' | wc -l | tr -d ' ')"
  record "todo-scan" info 0 "$todo_hits hit(s) in non-test code"
fi

# ---------------------------------------------------------------------------
# 2) 供应链与安全（CI 模式）
# ---------------------------------------------------------------------------
if [[ "$MODE" == "check-ci" || "$MODE" == "all" ]]; then
  echo "-- govulncheck --"
  if command -v govulncheck >/dev/null 2>&1; then
    run_check "govulncheck" 1 govulncheck ./...
  else
    record "govulncheck" skip 1 "govulncheck not installed (go install golang.org/x/vuln/cmd/govulncheck@latest)"
  fi

  echo "-- secret scan --"
  if command -v gitleaks >/dev/null 2>&1; then
    run_check "secret-scan" 1 gitleaks detect --source . --no-banner --redact
  elif command -v python3 >/dev/null 2>&1; then
    # 仓库内置高信号扫描（与 CI secret-scan job 使用同一实现，避免门禁分叉）。
    run_check "secret-scan" 1 python3 scripts/secret_scan.py --root .
  else
    record "secret-scan" skip 1 "no gitleaks and no python3"
  fi

  echo "-- E2E / deterministic state --"
  run_check "e2e-app-determinism" 1 go test ./app/ -run 'TestAppStateDeterminism|TestAppImportExport' -count=1
fi

# ---------------------------------------------------------------------------
# 3) 创世校验
# ---------------------------------------------------------------------------
if [[ "$MODE" == "check-genesis" || "$MODE" == "all" ]]; then
  echo "-- genesis params regression --"
  if command -v python3 >/dev/null 2>&1; then
    run_check "genesis-params" 1 python3 scripts/test_genesis_params.py
  else
    record "genesis-params" skip 1 "python3 not found"
  fi

  GENESIS_HOME="${GENESIS_HOME:-build/.mcchain}"
  GENESIS_FILE="$GENESIS_HOME/config/genesis.json"
  if [[ -f "$GENESIS_FILE" ]]; then
    if command -v mcchaind >/dev/null 2>&1; then
      run_check "genesis-validate" 1 mcchaind validate-genesis --home "$GENESIS_HOME"
    elif [[ -x build/mcchaind ]]; then
      run_check "genesis-validate" 1 build/mcchaind validate-genesis --home "$GENESIS_HOME"
    else
      record "genesis-validate" skip 1 "mcchaind binary not available"
    fi
    record "genesis-sha256" info 0 "$(sha256sum "$GENESIS_FILE" | awk '{print $1}')"

    # 档位一致性：创世 chain-id 必须被 chainprofile 精确识别，
    # 且与运维声明的 MC_PROFILE 一致（与应用/CLI/启动脚本共用同一判定）。
    GENESIS_CHAIN_ID="$(python3 - "$GENESIS_FILE" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    print(json.load(f).get("chain_id", ""))
PY
)"
    BIN_FOR_PROFILE=""
    if command -v mcchaind >/dev/null 2>&1; then BIN_FOR_PROFILE="mcchaind"; elif [[ -x build/mcchaind ]]; then BIN_FOR_PROFILE="build/mcchaind"; fi
    if [[ -z "$BIN_FOR_PROFILE" ]]; then
      record "genesis-profile" skip 1 "mcchaind binary not available"
    elif GENESIS_PROFILE="$("$BIN_FOR_PROFILE" profile "$GENESIS_CHAIN_ID" 2>/dev/null)"; then
      if [[ -n "${MC_PROFILE:-}" && "${MC_PROFILE}" != "$GENESIS_PROFILE" ]]; then
        record "genesis-profile" fail 1 "genesis chain-id $GENESIS_CHAIN_ID → $GENESIS_PROFILE, but MC_PROFILE=$MC_PROFILE"
      else
        record "genesis-profile" pass 1 "chain-id=$GENESIS_CHAIN_ID profile=$GENESIS_PROFILE"
      fi
    else
      record "genesis-profile" fail 1 "chain-id '$GENESIS_CHAIN_ID' is not registered in internal/chainprofile"
    fi
  else
    # 发布门禁不允许「创世不存在就跳过」——这是原脚本产生误导性全绿的主要来源。
    record "genesis-validate" skip 1 "$GENESIS_FILE 不存在（发布前必须提供已复核的创世）"
  fi
fi

# ---------------------------------------------------------------------------
# 4) 制品（二进制哈希 / SBOM / 签名）
# ---------------------------------------------------------------------------
BIN_SHA=""
if [[ "$MODE" == "check-artifact" || "$MODE" == "all" ]]; then
  echo "-- artifact build --"
  mkdir -p build
  if go build -trimpath -buildvcs=true -o build/mcchaind ./cmd/mcchaind 2>/dev/null; then
    BIN_SHA="$(sha256sum build/mcchaind | awk '{print $1}')"
    record "artifact-build" pass 1 "build/mcchaind sha256=$BIN_SHA"
  else
    record "artifact-build" fail 1 "go build ./cmd/mcchaind failed"
  fi

  echo "-- SBOM (go version -m) --"
  if [[ -n "$BIN_SHA" ]]; then
    go version -m build/mcchaind > "$REPORT_DIR/sbom-$STAMP.txt" 2>/dev/null
    if [[ -s "$REPORT_DIR/sbom-$STAMP.txt" ]]; then
      record "sbom" pass 1 "$REPORT_DIR/sbom-$STAMP.txt"
    else
      record "sbom" fail 1 "empty SBOM"
    fi
  else
    record "sbom" skip 1 "no binary"
  fi

  echo "-- artifact signature --"
  if command -v cosign >/dev/null 2>&1; then
    record "artifact-signature" skip 1 "cosign available; keyless signing requires CI OIDC (run in release workflow)"
  elif command -v minisign >/dev/null 2>&1; then
    record "artifact-signature" skip 1 "minisign available; provide signing key in release workflow"
  else
    record "artifact-signature" skip 1 "no signing tool (cosign/minisign) installed"
  fi
fi

# ---------------------------------------------------------------------------
# 5) 机器可读报告
# ---------------------------------------------------------------------------
COMMIT="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
GO_VERSION="$(go version 2>/dev/null || echo unknown)"
GENESIS_SHA="$( [[ -f "${GENESIS_HOME:-build/.mcchain}/config/genesis.json" ]] && sha256sum "${GENESIS_HOME:-build/.mcchain}/config/genesis.json" | awk '{print $1}' || echo "" )"
JSON="$REPORT_DIR/readiness-$STAMP.json"

python3 - "$RESULTS" "$JSON" "$MODE" "$MAINNET" "$COMMIT" "$GO_VERSION" "$BIN_SHA" "$GENESIS_SHA" "$STAMP" <<'PY'
import json, sys
results_path, out_path, mode, mainnet, commit, go_version, bin_sha, genesis_sha, stamp = sys.argv[1:10]
checks = []
with open(results_path, encoding="utf-8") as f:
    for line in f:
        parts = line.rstrip("\n").split("\t")
        if len(parts) >= 4:
            checks.append({
                "name": parts[0],
                "status": parts[1],
                "required": parts[2] == "1",
                "detail": parts[3],
            })
report = {
    "tool": "mcchain-mainnet-readiness",
    "mode": mode,
    "mainnet": mainnet == "1",
    "commit": commit,
    "go_version": go_version,
    "binary_sha256": bin_sha,
    "genesis_sha256": genesis_sha,
    "timestamp_utc": stamp,
    "checks": checks,
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(report, f, indent=2, ensure_ascii=False)
print(out_path)
PY

echo "== summary =="
awk -F'\t' '{c[$2]++} END {for (k in c) printf "  %-5s %d\n", k, c[k]}' "$RESULTS"
echo "  report: $JSON"

if [[ $FAILURES -gt 0 || $REQUIRED_SKIPS -gt 0 ]]; then
  echo "MAINNET NO-GO: $FAILURES failure(s), $REQUIRED_SKIPS required skip(s)"
  exit 1
fi

if [[ "$MAINNET" == "1" ]]; then
  echo "MAINNET GO"
else
  echo "READINESS OK (non-mainnet mode; re-run with --mainnet for Go/No-Go)"
fi
