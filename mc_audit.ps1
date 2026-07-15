$ErrorActionPreference = "Continue"
$CHAIN = "$HOME/mcchain"
$NODE  = "$CHAIN\build\mcchaind.exe"
$PY    = "$HOME\.workbuddy\binaries\python\versions\3.13.12\python.exe"
$TMP   = "$HOME/mcchain\audit_home"
$CID   = "mcchain-audit-1"
$RPC   = "tcp://localhost:26677"
$P2P   = "tcp://localhost:26678"

if (Test-Path $TMP) { Remove-Item $TMP -Recurse -Force }
Write-Host "=== init ==="
& $NODE 'init' 'auditor' '--chain-id' $CID '--home' $TMP 2>&1 | Out-String -Width 200
Write-Host "=== keys add validator ==="
& $NODE 'keys' 'add' 'validator' '--keyring-backend' 'test' '--home' $TMP 2>&1 | Out-String -Width 200
Write-Host "=== add-genesis-account ==="
& $NODE 'add-genesis-account' 'validator' '100000000000umc' '--home' $TMP '--keyring-backend' 'test' 2>&1 | Out-String -Width 200
Write-Host "=== gentx ==="
& $NODE 'gentx' 'validator' '100000000000umc' '--min-self-delegation' '100000000000' '--home' $TMP '--keyring-backend' 'test' '--chain-id' $CID 2>&1 | Out-String -Width 200
Write-Host "=== collect-gentxs ==="
& $NODE 'collect-gentxs' '--home' $TMP 2>&1 | Out-String -Width 200
Write-Host "=== make_genesis.py (prod) ==="
& $PY "$CHAIN\scripts\make_genesis.py" '--genesis' "$TMP\config\genesis.json" '--out' "$TMP\config\genesis.json" '--config' "$CHAIN\scripts\genesis-config.example.json" 2>&1 | Out-String -Width 200
Write-Host "=== validate-genesis ==="
& $NODE 'validate-genesis' "$TMP\config\genesis.json" '--home' $TMP 2>&1 | Out-String -Width 200
Write-Host "GENVALID_EXIT=$LASTEXITCODE"

Write-Host "=== start node (background) ==="
$proc = Start-Process -FilePath $NODE -ArgumentList @(
    'start','--home',$TMP,'--chain-id',$CID,
    '--rpc.laddr',$RPC,'--p2p.laddr',$P2P,
    '--consensus.timeout_commit','4s','--minimum-gas-prices','0umc'
) -PassThru -RedirectStandardOutput "$TMP\node.out" -RedirectStandardError "$TMP\node.err"
Write-Host "node pid = $($proc.Id)"
Start-Sleep -Seconds 30

Write-Host "=== mint params (must show inflation_rate_max=0 / inflation=0) ==="
& $NODE 'q' 'mint' 'params' '--home' $TMP '--node' $RPC 2>&1 | Out-String -Width 200

Write-Host "=== bank total supply (sample 1) ==="
$t1 = & $NODE 'q' 'bank' 'total' '--home' $TMP '--node' $RPC 2>&1 | Out-String -Width 200
$t1
Start-Sleep -Seconds 12
Write-Host "=== bank total supply (sample 2, must equal sample 1) ==="
$t2 = & $NODE 'q' 'bank' 'total' '--home' $TMP '--node' $RPC 2>&1 | Out-String -Width 200
$t2

Write-Host "=== tokenomics supply cap query ==="
& $NODE 'q' 'tokenomics' 'show-genesis' '--home' $TMP '--node' $RPC 2>&1 | Out-String -Width 200

$proc | Stop-Process -Force
Write-Host "AUDIT_DONE"