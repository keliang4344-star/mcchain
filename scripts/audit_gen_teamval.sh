#!/usr/bin/env bash
# MC 公链 - 团队多签作创世验证人的生产 genesis 生成器（审计/部署共用）
set +e
BIN=$HOME/mcchain/build/mcchaind
PY=python3
HD=$HOME/mcchain/audit_home
KEYS=$HOME/mcchain/team_keys_gen.json
CFG=$HOME/mcchain/scripts/genesis-config.example.json
CID=mcchain-mainnet-1

echo "STEP1 clear home"
cmd //c "if exist $HOME\\mcchain\\audit_home rmdir /s /q $HOME\\mcchain\\audit_home"
echo "STEP2 init"
"$BIN" init auditor --chain-id "$CID" --home "$HD" 2>&1 | head -3
echo "init_exit=$?"

echo "STEP3 recover keys"
mapfile -t ENTRIES < <("$PY" -c "import json;[print(e['name']+'|'+e['mnemonic']) for e in json.load(open('$KEYS'))]")
echo "entries_count=${#ENTRIES[@]}"
for e in "${ENTRIES[@]}"; do
  name="${e%%|*}"; mnem="${e#*|}"
  printf '%s\n' "$mnem" | "$BIN" keys add "$name" --recover --keyring-backend test --home "$HD" >/dev/null 2>&1
  echo "recovered $name rc=$?"
done

echo "STEP4 build multisig"
"$BIN" keys add teammultisig --multisig=team1,team2,team3,team4,team5 --multisig-threshold=3 --keyring-backend test --home "$HD" 2>&1 | tail -2
echo "=== teammultisig address (expect mc105qnk0v3gn96naljmazvqjmnza08u5yn0vwpxz) ==="
"$BIN" keys show teammultisig -a --keyring-backend test --home "$HD"
echo "multisig_rc=$?"

echo "STEP5 gentx generate-only"
"$BIN" gentx teammultisig 100000000000umc --chain-id "$CID" --min-self-delegation 30000000000 \
  --keyring-backend test --home "$HD" --generate-only > /tmp/gx.json 2>/tmp/gx.err
echo "gentx_rc=$?"; cat /tmp/gx.err

echo "STEP6 sign with 3 keys"
for k in team1 team2 team3; do
  "$BIN" tx sign /tmp/gx.json --from "$k" --chain-id "$CID" --keyring-backend test --home "$HD" \
    --offline --account-number 0 --sequence 0 --signature-only > "/tmp/sig_$k.json" 2>/tmp/s.err
  echo "sign $k rc=$?"; cat /tmp/s.err
done

echo "STEP7 multisign"
"$BIN" tx multisign /tmp/gx.json teammultisig /tmp/sig_team1.json /tmp/sig_team2.json /tmp/sig_team3.json \
  --chain-id "$CID" --keyring-backend test --home "$HD" > /tmp/gx_signed.json 2>/tmp/ms.err
echo "multisign_rc=$?"; cat /tmp/ms.err

mkdir -p "$HD/config/gentx"
cp /tmp/gx_signed.json "$HD/config/gentx/"
echo "STEP8 collect + normalize + validate"
"$BIN" collect-gentxs --home "$HD" 2>&1 | tail -2
"$PY" $HOME/mcchain/scripts/make_genesis.py --genesis "$HD/config/genesis.json" --out "$HD/config/genesis.json" --config "$CFG" 2>&1 | tail -12
"$BIN" validate-genesis "$HD/config/genesis.json" --home "$HD" 2>&1 | tail -2
echo "GEN_DONE"
