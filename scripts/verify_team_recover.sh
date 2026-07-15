#!/usr/bin/env bash
# 真实验证：用真实 mcchaind 从 team_keys_gen.json 的助记词恢复 5 把私钥，
# 用标准 `keys add --multisig`（默认按地址排序）重建 3-of-5 多签，
# 比较其地址是否与链编译的 TeamAddress 一致。这是资金可控性的最终判据。
set +e
export MSYS_NO_PATHCONV=1
BIN="$HOME/mcchain/mcchaind.exe"
HD="$HOME/mcchain/verify_home"
JSON="$HOME/mcchain/team_keys_gen.json"
EXPECTED="mc1uq85t4erj44lf3x23xnrr97lt4wlyfz5kkf96f"

cmd //c "if exist \"$HD\" rmdir /s /q \"$HD\"" 2>/dev/null
mkdir -p "$HD"

# 用 python 提取 5 个助记词
MN=$($HOME/.workbuddy/binaries/python/versions/3.13.12/python.exe -c "import json;d=json.load(open(r'$JSON'));print('\n'.join(x['mnemonic'] for x in d))")
i=1
while IFS= read -r line; do
  printf '%s\n' "$line" | "$BIN" keys add "team$i" --recover --keyring-backend test --home "$HD" >/dev/null 2>&1
  echo "recover team$i exit=$?"
  i=$((i+1))
done <<< "$MN"

"$BIN" keys add teammultisig --multisig=team1,team2,team3,team4,team5 --multisig-threshold=3 --keyring-backend test --home "$HD" >/dev/null 2>&1
GOT=$("$BIN" keys show teammultisig -a --keyring-backend test --home "$HD")
echo "reconstructed multisig addr = $GOT"
echo "expected (compiled) addr    = $EXPECTED"
if [ "$GOT" = "$EXPECTED" ]; then
  echo "✅ 通过：真实 mcchaind 恢复的助记词重建出的多签 = 链编译 TeamAddress，团队资金可控。"
else
  echo "❌ 失败：重建多签与链编译 TeamAddress 不一致，团队资金可能永久锁定！"
fi
cmd //c "if exist \"$HD\" rmdir /s /q \"$HD\"" 2>/dev/null
