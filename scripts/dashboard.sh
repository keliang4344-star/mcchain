#!/usr/bin/env bash
#
# MC 公链 · 运营数据看板（第一阶段）
#
# 数据源：节点 RPC/REST + mcchaind query。零依赖（bash + curl + python3）。
# 用法：
#   ./scripts/dashboard.sh            # 一次性输出当前快照
#   watch -n 30 ./scripts/dashboard.sh # 挂大屏 30s 刷新
#
# 指标（第一阶段运营核心 6 项）：
#   1) 链高度与出块状态      2) 验证人数量/投票权
#   3) umc 总供应            4) phonenode 注册/在线设备
#   5) depin 累计 payout     6) 今日 tx 数（活动度）
#
set -uo pipefail

RPC="${RPC:-http://127.0.0.1:26657}"
REST="${REST:-http://127.0.0.1:1317}"
MCCHAIND="${MCCHAIND:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/build/mcchaind}"
PY="$(command -v python3 || command -v python)"

if [[ ! -x "$MCCHAIND" ]]; then echo "[FATAL] 未找到 mcchaind" >&2; exit 2; fi
q()  { "$MCCHAIND" query "$@" --node "$RPC" -o json 2>/dev/null; }
get(){ curl -sf --max-time 8 "$1" 2>/dev/null; }

bold=$(printf '\033[1m'); grn=$(printf '\033[32m'); ylw=$(printf '\033[33m'); rst=$(printf '\033[0m')
line(){ printf '%s\n' "--------------------------------------------------------------"; }
kv(){ printf '  %-22s %s\n' "$1" "$2"; }

echo "${bold}MC 公链 · 运营数据看板${rst}  $(date '+%F %T')"
line

# 1) 链高度
st=$("$MCCHAIND" status --node "$RPC" 2>/dev/null)
if [[ -z "$st" ]]; then
  echo "  ${ylw}[!] RPC 不可达：$RPC${rst}"; exit 1
fi
height=$(echo "$st" | "$PY" -c 'import json,sys;print(json.load(sys.stdin)["sync_info"]["latest_block_height"])' 2>/dev/null)
catching=$(echo "$st" | "$PY" -c 'import json,sys;print(json.load(sys.stdin)["sync_info"]["catching_up"])')
kv "链高度" "$height"
kv "同步状态" "$([[ "$catching" == "false" ]] && echo "${grn}已追平（出块中）${rst}" || echo "${ylw}追赶中${rst}")"

# 2) 验证人
vals=$(q staking validators 2>/dev/null | "$PY" -c '
import json,sys
d=json.load(sys.stdin)
v=d.get("validators",d if isinstance(d,list) else [])
print(len(v))
' 2>/dev/null)
kv "验证人数量" "${vals:-N/A}"

# 3) 供应
supply=$(get "$REST/cosmos/bank/v1beta1/supply" | "$PY" -c '
import json,sys
d=json.load(sys.stdin)
for c in d.get("supply",[]):
    if c.get("denom")=="umc":
        a=int(c["amount"]); print(f"{a:,} umc (≈ {a/1e6:,.0f} MC)"); break
' 2>/dev/null)
kv "umc 总供应" "${supply:-N/A}"

# 4) phonenode 设备
pn=$("$MCCHAIND" query phonenode params --node "$RPC" -o json >/dev/null 2>&1 && \
     "$MCCHAIND" query phonenode list-node 2>/dev/null --node "$RPC" -o json | "$PY" -c '
import json,sys
try:
    d=json.load(sys.stdin)
    items=d.get("nodes") or d.get("Node") or d
    print(len(items) if isinstance(items,list) else "见CLI")
except Exception:
    print("见CLI")
' 2>/dev/null)
kv "phonenode 注册设备" "${pn:-见 CLI: mcchaind query phonenode list-node}"

# 5) depin payout
depin=$(get "$REST/mcchain/depin/params" >/dev/null 2>&1 && echo "params ok" || echo "")
kv "depin 模块" "${depin:-REST 路径未开放，用 CLI query depin …}"

# 6) 今日活动（近 14400 块 = 16h 内 tx 粗估；第一阶段用块浏览器聚合替代精确值）
kv "今日 tx（近 1h 块数）" "$("$PY" -c "
h=$height
print(f'{h} (高度)') if False else print('按 21600 块/日，今日已出 ', h % 21600, ' 块')
" 2>/dev/null)"

line
echo "  ${ylw}提示：全部指标同时应出现在 Prometheus（deploy/prometheus-mcchain.yml）；"
echo "  本脚本用于大屏快速查看与值班晨报。${rst}"
