#!/usr/bin/env bash
#
# MC 公链 · 白皮书印刷版（典藏版 PDF）渲染
#
# 流水线：
#   WHITEPAPER_CN.md  ──(render_whitepaper_html.py)──>  docs/whitepaper.html
#                     ──(Chrome headless, A4 @media print)──>  docs/MobileChain白皮书_完整典藏版.pdf
#                     ──(add_pdf_outline.py)──>  写入 58 条目录书签
#
# 这样 PDF 永远与正典 Markdown 同步，不会出现「尾章/附录缺失」这类漂移。
#
# 用法：
#   ./scripts/render_whitepaper_pdf.sh             # 刷新 HTML 并出 PDF
#   ./scripts/render_whitepaper_pdf.sh --no-html   # 只出 PDF（沿用现有 HTML）
#
# 依赖：python3（书签步骤需 pypdf：pip install pypdf）、Google Chrome 或 Microsoft Edge
#
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HTML="$ROOT/docs/whitepaper.html"
OUT="$ROOT/docs/MobileChain白皮书_完整典藏版.pdf"

REFRESH_HTML=1
[ "${1:-}" = "--no-html" ] && REFRESH_HTML=0

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

# ---- 1/3 从正典 Markdown 刷新 HTML ------------------------------------------
if [ "$REFRESH_HTML" = "1" ]; then
  echo "[1/3] 渲染 HTML  ← WHITEPAPER_CN.md"
  if [ -z "$PY" ]; then
    echo "ERROR: 需要一个可用的 python3" >&2; exit 2
  fi
  "$PY" "$(winpath "$ROOT/scripts/render_whitepaper_html.py")" || exit 1
else
  echo "[1/3] 跳过 HTML 渲染（--no-html）"
fi

# ---- 2/3 打印为 A4 PDF -------------------------------------------------------
echo "[2/3] 打印 PDF  ← $(basename "$HTML")"
echo "      浏览器: $BROWSER"

URL="file:///$(winpath "$HTML")"
OUT_WIN="$(winpath "$OUT")"

rm -f "$OUT"

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

if [ ! -s "$OUT" ]; then
  echo "ERROR: PDF 未生成" >&2
  exit 1
fi

# ---- 3/3 写入目录书签 --------------------------------------------------------
echo "[3/3] 写入目录书签"
if [ -n "$PY" ]; then
  "$PY" "$(winpath "$ROOT/scripts/add_pdf_outline.py")" \
        "$(winpath "$OUT")" "$(winpath "$ROOT/WHITEPAPER_CN.md")" || \
    echo "      警告：书签写入失败（PDF 正文不受影响）"
else
  echo "      跳过：未找到带 pypdf 的解释器（pip install pypdf）"
fi

SIZE="$(stat -c%s "$OUT" 2>/dev/null || echo '?')"
echo "完成: $OUT  (${SIZE} 字节)"
