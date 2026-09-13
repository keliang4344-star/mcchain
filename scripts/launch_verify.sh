#!/usr/bin/env bash
#
# MC 公链 · 主网 T0 启动验收自检（launch verify）
#
# 定位：deploy/init.sh 负责「建链」，本脚本负责「建好之后的验收」。
# 把 docs/NODE_CONFIG.md §6 的人工五项参数核对 + docs/VALIDATOR_RUNBOOK 的
# 稳定性检查 + T0 验收标准自动化，输出 PASS/FAIL 报告。
#
# 用法（在任意能访问节点 RPC 的机器上执行）：
#   ./scripts/launch_verify.sh [RPC_URL] [连续观察秒数]
#   默认 RPC=http://127.0.0.1:26657，观察 900s（15 分钟出块稳定性）
#
# 退出码：0 = 全部 PASS；1 = 存在 FAIL（不可上线）；2 = 环境错误
#
set -uo pipefail

RPC="${1:-http://127.0.0.1:26657}"
OBSERVE="${2:-900}"
MCCHAIND="${MCCHAIND:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/build/mcchaind}"
PY="$(command -v python3 || command -v python)"

if [[ ! -x "$MCCHAIND" ]]; then
  echo "[FATAL] 未找到 mcchaind: $MCCHAIND" >&2; exit 2
fi
[[ -n "$PY" ]] || { echo "[FATAL] 需要 python3" >&2; exit 2; }

PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); printf '  \033[32mPASS\033[0m %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  \033[31mFAIL\033[0m %s\n' "$1"; }
warn() { printf '  \033[33mWARN\033[0m %s\n' "$1"; }
hdr()  { printf '\n\033[1;36m== %s ==\033[0m\n' "$1"; }

# rpc_get <path>  —— REST(LCD) GET
rpc_get() { curl -sf --max-time 10 "${RPC%%:*}//${RPC#*://}" >/dev/null 2>&1; }  # 占位，实际用 query
q() { "$MCCHAIND" query "$@" --node "$RPC" -o json 2>/dev/null; }

jget() { "$PY" -c "
import json,sys
d=json.load(sys.stdin)
cur=d
for k in sys.argv[1:]:
    cur=cur[int(k)] if isinstance(cur,list) else cur.get(k)
print(cur)
" "$@" 2>/dev/null; }

hdr "0. 连通性"
if "$MCCHAIND" status --node "$RPC" >/dev/null 2>&1; then ok "RPC 可达 $RPC"; else bad "RPC 不可达 $RPC"; echo "[FATAL] 后续检查中止" >&2; exit 1; fi

hdr "1. 出块稳定性（观察 ${OBSERVE}s）"
h1=$("$MCCHAIND" status --node "$RPC" 2>/dev/null | "$PY" -c 'import json,sys;print(json.load(sys.stdin)["SyncInfo"]["latest_block_height"])')
t1=$(date +%s)
sleep "$OBSERVE"
h2=$("$MCCHAIND" status --node "$RPC" 2>/dev/null | "$PY" -c 'import json,sys;print(json.load(sys.stdin)["SyncInfo"]["latest_block_height"])')
t2=$(date +%s)
dt=$((t2-t1)); dh=$((h2-h1))
if (( dh * 4 >= dt * 8 / 10 )); then
  ok "出块速率正常：${dh} 块 / ${dt}s（≈$(awk "BEGIN{printf \"%.2f\", $dh*4.0/$dt}") 倍实时）"
else
  bad "出块过慢：${dh} 块 / ${dt}s（预期 ≥ 实时 0.8 倍）"
fi
catching=$("$MCCHAIND" status --node "$RPC" 2>/dev/null | "$PY" -c 'import json,sys;print(str(json.load(sys.stdin)["SyncInfo"]["catching_up"]).lower())')
[[ "$catching" == "false" ]] && ok "catching_up=false（已追平）" || bad "catching_up=$catching"

hdr "2. 共识参数五项核对（与 PROTOCOL_PARAMS 对齐）"
cp_json=$(curl -sf --max-time 10 "$RPC/consensus_params" 2>/dev/null)
max_gas=$(echo "$cp_json" | "$PY" -c 'import json,sys;print(json.load(sys.stdin)["result"]["consensus_params"]["block"]["max_gas"])' 2>/dev/null)
[[ "$max_gas" == "100000000" ]] && ok "block.max_gas=100000000" || bad "block.max_gas=$max_gas（期望 100000000）"
blk_max_bytes=$(echo "$cp_json" | "$PY" -c 'import json,sys;print(json.load(sys.stdin)["result"]["consensus_params"]["block"]["max_bytes"])' 2>/dev/null)
[[ -n "$blk_max_bytes" && "$blk_max_bytes" != "None" ]] && ok "block.max_bytes=$blk_max_bytes" || bad "block.max_bytes 缺失"

sp=$(q slashing params)
sbw=$(echo "$sp" | jget signed_blocks_window)
jdur=$(echo "$sp" | jget downtime_jail_duration)
[[ "$sbw" == "21600" ]] && ok "slashing.signed_blocks_window=21600（24h）" || bad "slashing.signed_blocks_window=$sbw"
[[ "$jdur" == "600s" ]] && ok "slashing.downtime_jail_duration=600s" || bad "downtime_jail_duration=$jdur"

stp=$(q staking params)
ub=$(echo "$stp" | jget unbonding_time)
mv=$(echo "$stp" | jget max_validators)
bd=$(echo "$stp" | jget bond_denom)
mcr=$(echo "$stp" | jget min_commission_rate)
[[ "$ub" == "1814400s" ]] && ok "staking.unbonding_time=1814400s（21 天）" || bad "unbonding_time=$ub"
[[ "$mv" == "100" ]] && ok "staking.max_validators=100" || bad "max_validators=$mv"
[[ "$bd" == "umc" ]] && ok "staking.bond_denom=umc" || bad "bond_denom=$bd"
# 反零佣金抢单面：min_commission_rate 必须 ≥ 5%（mainnet-genesis-config.json 已固化 0.05）。
if [[ -n "$mcr" && "$mcr" != "None" ]]; then
  if [[ "${mcr%.*}" == "0" ]] || (( $(echo "$mcr < 0.05" | awk '{print ($1+0<0.05)?1:0}' 2>/dev/null || echo 0) )); then
    bad "staking.min_commission_rate=$mcr（应 ≥ 0.05 / 5%；零佣金会打开抢单面）"
  else
    ok "staking.min_commission_rate=$mcr（≥5%）"
  fi
else
  warn "staking.min_commission_rate 读不到（CLI 输出格式问题）"
fi

mp=$(q mint params)
inf_max=$(echo "$mp" | jget inflation_max)
[[ "$inf_max" == "0.000000000000000000" ]] && ok "mint 零通胀（inflation_max=0）" || bad "inflation_max=$inf_max（必须 0）"

# 治理参数：必须与 PROTOCOL_PARAMS.md 对齐（voting/deposit 48h、quorum 0.334、
# threshold 0.5、veto 0.334）。SDK 默认是 172800s/172800s/0.334/0.5/0.334，恰好相同，
# 但仍必须核对——一旦未来 SDK 默认改了、或 deploy/init.sh 漏写，会静默失效。
gp=$(q gov params)
gvp=$(echo "$gp" | jget voting_period)
gmdp=$(echo "$gp" | jget max_deposit_period)
gq=$(echo "$gp" | jget quorum)
gt=$(echo "$gp" | jget threshold)
gv=$(echo "$gp" | jget veto_threshold)
[[ "$gvp" == "172800s" ]] && ok "gov.voting_period=172800s（48h）" || bad "gov.voting_period=$gvp"
[[ "$gmdp" == "172800s" ]] && ok "gov.max_deposit_period=172800s" || bad "gov.max_deposit_period=$gmdp"
[[ "$gq" == "0.334000000000000000" ]] && ok "gov.quorum=0.334" || bad "gov.quorum=$gq"
[[ "$gt" == "0.500000000000000000" ]] && ok "gov.threshold=0.5" || bad "gov.threshold=$gt"
[[ "$gv" == "0.334000000000000000" ]] && ok "gov.veto_threshold=0.334" || bad "gov.veto_threshold=$gv"

hdr "3. 验证人在位"
vals=$(q staking validators 2>/dev/null | "$PY" -c 'import json,sys;d=json.load(sys.stdin);print(len(d.get("validators",d if isinstance(d,list) else [])))' 2>/dev/null)
MIN_VALS="${MIN_VALIDATORS:-3}"
if [[ "${vals:-0}" -ge "$MIN_VALS" ]]; then ok "验证人数 = $vals（≥$MIN_VALS）"; else bad "验证人数 = ${vals:-0}（<$MIN_VALS，主网不成立）"; fi

hdr "4. 供应与五池（tokenomics 总账）"
sup=$(curl -sf --max-time 10 "${RPC/26657/1317}/cosmos/bank/v1beta1/supply" 2>/dev/null | "$PY" -c '
import json,sys
d=json.load(sys.stdin)
for c in d.get("supply",[]):
    if c.get("denom")=="umc": print(c["amount"]); break
' 2>/dev/null)
# REST 端口没开不该被读成「无此项」——回退到 CLI 直查账本，保证这项永远有结论。
if [[ -z "$sup" || "$sup" == "None" ]]; then
  sup=$("$MCCHAIND" query bank total --node "$RPC" -o json 2>/dev/null | "$PY" -c '
import json,sys
d=json.load(sys.stdin)
for c in d.get("supply",[]):
    if c.get("denom")=="umc": print(c["amount"]); break
' 2>/dev/null)
fi
if [[ -n "$sup" && "$sup" != "None" ]]; then
  cap=1000000000000000
  if (( sup <= cap )); then ok "umc 总供应 $sup ≤ 硬顶 $cap"; else bad "umc 总供应 $sup 超过硬顶 $cap（严重！创世账户有五池之外的注资）"; fi
else
  warn "无法读取供应量（REST 与 CLI 均失败）——人工核对"
fi

hdr "5. 端口与外部服务"
for p in 26657 1317 9090; do
  port="${RPC##*:}"
  [[ "$p" == "$port" ]] && continue
  if timeout 3 bash -c "echo >/dev/tcp/127.0.0.1/$p" 2>/dev/null; then ok "本机端口 $p 在听"; else warn "本机端口 $p 未监听（若为对外节点需确认）"; fi
done

# 可选：若本机存在 ~/.mcchain/config/app.toml，校验 minimum-gas-prices 已显式配置。
# launch_verify 不假定 home 位置，但运维跑本脚本最常见的姿势就是在验证人主机上。
app_toml="${HOME:-/root}/.mcchain/config/app.toml"
if [[ -r "$app_toml" ]]; then
  hdr "5.5. app.toml minimum-gas-prices（运维层，未设时 mempool 收 0-fee 交易）"
  mgp=$(grep -E '^minimum-gas-prices\s*=' "$app_toml" 2>/dev/null | head -1 | sed 's/.*=\s*//;s/"//g')
  if [[ -n "$mgp" ]]; then
    if [[ "$mgp" =~ 0[a-zA-Z]*$ ]] || [[ "$mgp" == "0" ]]; then
      bad "app.toml minimum-gas-prices=$mgp：mempool 收 0-fee 交易，垃圾流量没有经济门槛"
    else
      ok "app.toml minimum-gas-prices=$mgp"
    fi
  else
    warn "未读取到 minimum-gas-prices（配置项缺失或行格式不同）"
  fi
fi

hdr "6. 预言机模式（生产必须 TeeOracle）"
if [[ -n "${MC_ORACLE_PUBKEY:-}" ]]; then
  ok "MC_ORACLE_PUBKEY 已设置（TeeOracle 模式）"
else
  if [[ "${MC_ORACLE_ALLOW_SOFT:-}" == "1" ]]; then
    bad "当前为 SoftOracle 模式——生产环境禁止！必须配置 MC_ORACLE_PUBKEY"
  else
    warn "未检测到预言机环境变量（若非 validator 进程所在 shell 可忽略）"
  fi
fi

hdr "7. 主网创世密钥闸门"
# 创世密钥闸门不能靠 chain-id 命名判定（改名即可绕过），必须显式声明。
# 这里做静态可达性检查：systemd unit 里配了，或当前 shell 环境里有，二者其一。
unit_file=""
for f in /etc/systemd/system/mcchaind.service /lib/systemd/system/mcchaind.service; do
  [[ -r "$f" ]] && unit_file="$f" && break
done
if [[ -n "$unit_file" ]] && grep -q '^Environment=MC_REQUIRE_REAL_GENESIS_KEYS=1' "$unit_file"; then
  ok "systemd unit 已声明主网创世密钥闸门：$unit_file"
elif [[ "${MC_REQUIRE_REAL_GENESIS_KEYS:-}" == "1" ]]; then
  ok "当前环境已声明主网创世密钥闸门"
else
  warn "未发现 MC_REQUIRE_REAL_GENESIS_KEYS=1 —— 创世时会退回 chain-id 嗅探判定。"\
       "若 chain-id 不含 mainnet 且团队多签/基金会地址仍是占位密钥，"\
       "1.75 亿 MC 将落到源码可推导的私钥上（上线前必须补齐）"
fi

# ── 基金会/早期开发三把公钥（主网创世闸门）──
#
# 为什么必须单独检：tokenomics.InitGenesis 在 MC_REQUIRE_REAL_GENESIS_KEYS=1（或
# chain-id 含 mainnet）时要求 FoundationOverridesConfigured() 为真，否则**返回错误、
# 链起不来**。而 validate-genesis 不检、节点起不来之前也没有任何链上指标可查，
# 运维只能看到一句 panic。主网部署前必须确认这三把公钥已注入：部署文档、
# systemd unit 与验收脚本三处都要声明，缺一处就会出现「照文档做也起不来」。
#
# 判据（与 x/tokenomics/types/keys.go 的 FoundationOverridesConfigured 同口径）：
#   三个环境变量都非空，或三个真实公钥已写进源码 gen 文件（此时源码内非空）。
env_missing=0
for v in MC_FOUNDATION_EARLY_DEV_PUBKEY MC_FOUNDATION_OPS_PUBKEY MC_FOUNDATION_VESTING_PUBKEY; do
  [[ -n "${!v:-}" ]] || env_missing=$((env_missing + 1))
done
gen_overrides=0
for f in ./x/tokenomics/types/foundation_addrs_gen.go; do
  if [[ -r "$f" ]] && grep -qE '^(earlyDevPubKeyOverride|foundationOpsPubKeyOverride|foundationVestingPubKeyOverride)\s*=\s*"[^"]+"' "$f"; then
    gen_overrides=$((gen_overrides + 1))
  fi
done
if (( env_missing == 0 )); then
  ok "三把基金会/早期开发公钥已由环境变量注入（CI 无法校验其真实性，须人工确认是冷钱包/多签）"
elif (( gen_overrides == 3 )); then
  ok "三把基金会/早期开发公钥已写入源码 gen 文件"
else
  bad "基金会/早期开发拨付公钥未配置齐（env 缺 $env_missing 个，源码 gen 文件配 $gen_overrides 个）"\
      "—— 主网 chain-id（或 MC_REQUIRE_REAL_GENESIS_KEYS=1）下 tokenomics.InitGenesis 会直接报错，链起不来；"\
      "且 18% 总量（早期开发 5% + 基金会 13%）会落在源码可推导的占位私钥上。"\
      "修法：在 /etc/mcchain/mainnet.env 里设 MC_FOUNDATION_EARLY_DEV_PUBKEY / MC_FOUNDATION_OPS_PUBKEY / MC_FOUNDATION_VESTING_PUBKEY"\
      "（模板 deploy/mainnet.env.example），或把真实公钥写进 x/tokenomics/types/foundation_addrs_gen.go。详见 docs/DEPLOYMENT_GUIDE.md"
fi

hdr "结果汇总"
echo "----------------------------------------"
echo "  PASS: $PASS    FAIL: $FAIL"
echo "----------------------------------------"
if (( FAIL > 0 )); then
  echo "\033[31m结论：存在 FAIL 项，不可按 T0 上线。\033[0m" | sed 's/\\033\[31m//g;s/\\033\[0m//g'
  echo "结论：存在 FAIL 项，不可按 T0 上线。"
  exit 1
fi
echo "结论：全部 PASS，具备 T0 上线条件（仍需完成 72h 稳定性观察）。"
exit 0
