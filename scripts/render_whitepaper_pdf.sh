#!/usr/bin/env bash
#
# MC 公链 · 白皮书印刷版渲染（中文典藏版 + 英文典藏版）
#
# 流水线（每一版都一样）：
#   <正典>.md  ──(render_whitepaper_html.py)──>  docs/<html>
#              ──(Chrome headless, A4 @media print)──>  docs/<pdf>
#              ──(add_pdf_outline.py)──>  写入目录书签
#
# 中文正典 WHITEPAPER_CN.md 是唯一内容来源；英文正典 WHITEPAPER.md 是它的
# 逐章对应翻译。两版共用同一套深金视觉壳，只是 UI 文案与语种不同。
# 这样 PDF 永远与正典 Markdown 同步，不会出现「尾章/附录缺失」这类漂移。
#
# 用法：
#   ./scripts/render_whitepaper_pdf.sh             # 刷新两版的 HTML 与 PDF
#   ./scripts/render_whitepaper_pdf.sh --no-html   # 只出 PDF（沿用现有 HTML）
#   ./scripts/render_whitepaper_pdf.sh --zh        # 只出中文版
#   ./scripts/render_whitepaper_pdf.sh --en        # 只出英文版
#
# 依赖：python3（书签步骤需 pypdf：pip install pypdf）、Google Chrome 或 Microsoft Edge
#
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

REFRESH_HTML=1
WANT_ZH=1
WANT_EN=1
for arg in "$@"; do
  case "$arg" in
    --no-html) REFRESH_HTML=0 ;;
    --zh) WANT_EN=0 ;;
    --en) WANT_ZH=0 ;;
    *) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

# ---- 路径转换 ----------------------------------------------------------------
# Windows 原生程序（python.exe / chrome.exe）不认 git-bash 的 /e/... 形式，会被
# 当成当前盘符下的相对路径。统一转成 E:/a/b/c 这种混合风格。
winpath() {
  local p
  p="$(cygpath -w "$1" 2>/dev/null || printf '%s' "$1")"
  printf '%s' "$p" | sed 's#\\#/#g'
}

# ---- 定位解释器（书签步骤需要 pypdf）----------------------------------------
find_python() {
  local c
  for c in \
    "${PYTHON:-}" \
    "C:/Users/Administrator/.workbuddy/binaries/python/envs/default/Scripts/python.exe" \
    "$(command -v python3 2>/dev/null)" \
    "$(command -v python 2>/dev/null)" ; do
    [ -n "$c" ] && [ -x "$c" ] || continue
    if "$c" -c "import pypdf" >/dev/null 2>&1; then printf '%s' "$c"; return 0; fi
  done
  return 1
}

# ---- 定位浏览器 --------------------------------------------------------------
find_browser() {
  local c
  for c in \
    "${CHROME_PATH:-}" \
    "/c/Program Files/Google/Chrome/Application/chrome.exe" \
    "/c/Program Files (x86)/Google/Chrome/Application/chrome.exe" \
    "/c/Program Files/Microsoft/Edge/Application/msedge.exe" \
    "/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" \
    "$(command -v google-chrome 2>/dev/null)" \
    "$(command -v chromium 2>/dev/null)" \
    "$(command -v chromium-browser 2>/dev/null)" ; do
    [ -n "$c" ] && [ -x "$c" ] && { printf '%s' "$c"; return 0; }
  done
  return 1
}

BROWSER="$(find_browser)" || {
  echo "ERROR: 未找到 Chrome / Edge，可用 CHROME_PATH 指定" >&2
  exit 2
}

PY="$(find_python || true)"

# ---- 单版渲染 ----------------------------------------------------------------
# render_one <标签> <正典md> <html> <pdf> <语种> <壳来源>
render_one() {
  local label="$1" MD="$2" HTML="$3" PDF="$4" LANG_CODE="$5" SHELL_FROM="$6"

  echo ""
  echo "== $label =="

  if [ "$REFRESH_HTML" = "1" ]; then
    echo "[1/3] 渲染 HTML  ← $(basename "$MD")"
    [ -z "$PY" ] && { echo "ERROR: 需要一个可用的 python3" >&2; return 1; }
    "$PY" "$(winpath "$ROOT/scripts/render_whitepaper_html.py")" \
          --src "$(winpath "$MD")" --dst "$(winpath "$HTML")" \
          --shell-from "$(winpath "$SHELL_FROM")" --lang "$LANG_CODE" || return 1
  else
    echo "[1/3] 跳过 HTML 渲染（--no-html）"
  fi

  echo "[2/3] 打印 PDF  ← $(basename "$HTML")"
  local URL OUT_WIN
  URL="file:///$(winpath "$HTML")"
  OUT_WIN="$(winpath "$PDF")"
  rm -f "$PDF"

  "$BROWSER" \
    --headless=new \
    --disable-gpu \
    --no-sandbox \
    --no-pdf-header-footer \
    --print-to-pdf-no-header \
    --run-all-compositor-stages-before-draw \
    --virtual-time-budget=60000 \
    --print-to-pdf="$OUT_WIN" \
    "$URL" >/dev/null 2>&1

  if [ ! -s "$PDF" ]; then
    echo "ERROR: PDF 未生成" >&2
    return 1
  fi

  echo "[3/3] 写入目录书签"
  if [ -n "$PY" ]; then
    "$PY" "$(winpath "$ROOT/scripts/add_pdf_outline.py")" \
          "$(winpath "$PDF")" "$(winpath "$MD")" || \
      echo "      警告：书签写入失败（PDF 正文不受影响）"
  else
    echo "      跳过：未找到带 pypdf 的解释器（pip install pypdf）"
  fi

  local SIZE
  SIZE="$(stat -c%s "$PDF" 2>/dev/null || echo '?')"
  echo "完成: $PDF  (${SIZE} 字节)"
}

ZH_MD="$ROOT/WHITEPAPER_CN.md"
ZH_HTML="$ROOT/docs/whitepaper.html"
ZH_PDF="$ROOT/docs/MobileChain白皮书_完整典藏版.pdf"

EN_MD="$ROOT/WHITEPAPER.md"
EN_HTML="$ROOT/docs/whitepaper_en.html"
EN_PDF="$ROOT/docs/MobileChain-Whitepaper-Collector-Edition.pdf"

STATUS=0

if [ "$WANT_ZH" = "1" ]; then
  render_one "中文典藏版" "$ZH_MD" "$ZH_HTML" "$ZH_PDF" zh "$ZH_HTML" || STATUS=1
fi

if [ "$WANT_EN" = "1" ]; then
  # 英文版首次生成时自身还没有 HTML，用中文版作视觉壳（UI 文案会被本地化）。
  EN_SHELL="$EN_HTML"
  [ -f "$EN_HTML" ] || EN_SHELL="$ZH_HTML"
  render_one "英文典藏版" "$EN_MD" "$EN_HTML" "$EN_PDF" en "$EN_SHELL" || STATUS=1
fi

exit "$STATUS"
