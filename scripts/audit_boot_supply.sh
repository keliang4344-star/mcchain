#!/usr/bin/env bash
# 深度审计：启动链 + 验证供给量 = 1e15 + mint inflation = 0
# 前提：audit_home 已有正确 genesis（gen_genesis_teamval.sh 生成）
set +e
export MSYS_NO_PATHCONV=1
BIN="$HOME/mcchain/mcchaind"
HD="$HOME/mcchain/audit_home"
LOG="$HOME/mcchain/audit_boot.log"

echo "=== 启动链 ==="
"$BIN" start --home "$HD" --minimum-gas-prices 0umc > "$LOG" 2>&1 &
CPID=$!
echo "PID=$CPID"
sleep 8  # 等 2 个区块 (4s/block)

echo "=== 查询银行总供给 ==="
SUPPLY=$("$BIN" q bank total --home "$HD" --node tcp://localhost:26657 -o json 2>/dev/null | grep -o '"amount":"[0-9]*"' | head -1 | grep -o '[0-9]*')
echo "bank total supply = $SUPPLY umc"

echo "=== 查询 mint params ==="
MINT_INFL=$("$BIN" q mint params --home "$HD" --node tcp://localhost:26657 -o json 2>/dev/null | grep '"inflation_rate_max"' | grep -o '"[0-9.]*"')
echo "mint inflation_rate_max = $MINT_INFL"

echo "=== 查询 tokenomics ==="
TOK_CAP=$("$BIN" q tokenomics params --home "$HD" --node tcp://localhost:26657 -o json 2>/dev/null | grep '"total_supply_cap"' | grep -o '[0-9]*')
echo "tokenomics total_supply_cap = $TOK_CAP"

echo "=== 查询 depin ==="
DEPIN=$("$BIN" q depin params --home "$HD" --node tcp://localhost:26657 -o json 2>/dev/null | grep '"initial_pool"' | grep -o '[0-9]*')
echo "depin initial_pool = $DEPIN"

echo "=== 验证 ==="
if [ "$SUPPLY" = "1000000000000000" ]; then echo "✅ 总供给 = 1e15 umc"; else echo "❌ 供给不符: $SUPPLY"; fi
if echo "$MINT_INFL" | grep -q "0.000000"; then echo "✅ mint inflation = 0"; else echo "❌ mint inflation != 0: $MINT_INFL"; fi
if [ "$TOK_CAP" = "1000000000000000" ]; then echo "✅ tokenomics cap = 1e15"; else echo "❌ tokenomics cap: $TOK_CAP"; fi
if [ "$DEPIN" = "550000000000000" ]; then echo "✅ depin pool = 5.5e14 (设备激励 55%)"; else echo "❌ depin pool: $DEPIN"; fi

echo "=== 停止链 ==="
kill "$CPID" 2>/dev/null
echo "DONE"
