$ErrorActionPreference = "Continue"
$CHAIN = "$HOME/mcchain"
$NODE  = "$CHAIN\build\mcchaind.exe"
$PY    = "python3"
$TMP   = "$HOME/mcchain\audit_home"
$CID   = "mcchain-mainnet-1"   # 最终 chain-id（gentx 必须用它签名，与 make_genesis 覆盖后一致）

if (Test-Path $TMP) { Remove-Item $TMP -Recurse -Force }
Write-Host "=== init ==="
& $NODE 'init' 'auditor' '--chain-id' $CID '--home' $TMP 2>&1 | Out-Null
Write-Host "=== keys add validator ==="
& $NODE 'keys' 'add' 'validator' '--keyring-backend' 'test' '--home' $TMP 2>&1 | Out-Null
Write-Host "=== add-genesis-account ==="
& $NODE 'add-genesis-account' 'validator' '100000000000umc' '--home' $TMP '--keyring-backend' 'test' 2>&1 | Out-Null
Write-Host "=== gentx (signed with FINAL chain-id $CID) ==="
& $NODE 'gentx' 'validator' '100000000000umc' '--min-self-delegation' '100000000000' '--home' $TMP '--keyring-backend' 'test' '--chain-id' $CID 2>&1 | Out-String -Width 200
Write-Host "=== collect-gentxs ==="
& $NODE 'collect-gentxs' '--home' $TMP 2>&1 | Out-Null
Write-Host "=== make_genesis.py (prod) ==="
& $PY "$CHAIN\scripts\make_genesis.py" '--genesis' "$TMP\config\genesis.json" '--out' "$TMP\config\genesis.json" '--config' "$CHAIN\scripts\genesis-config.example.json" 2>&1 | Out-String -Width 200
Write-Host "=== validate-genesis ==="
& $NODE 'validate-genesis' "$TMP\config\genesis.json" '--home' $TMP 2>&1 | Out-String -Width 200
Write-Host "GENVALID_EXIT=$LASTEXITCODE"
Write-Host "AUDIT_GEN_DONE"