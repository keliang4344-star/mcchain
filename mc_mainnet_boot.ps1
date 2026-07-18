$ErrorActionPreference = 'Continue'
$NODE = '$HOME/mcchain\build\mcchaind.exe'
$PY = 'python3'
$CHAINHOME = '$HOME/mcchain\mainnetboot'
$CHAIN = 'mcchain-mainnet-1'
$MINER = 'mc1zpzk062u54sv4j9w4qvlwkyjpxauqpuh72fz9w'
$DEPIN = 'mc1lpan9nughhevhdfdu0eywkzllgz39cu8a49jlp'
$LOGDIR = '$HOME/mcchain'
$resultLog = "$LOGDIR\mainnet_boot.log"
$GEN = "$LOGDIR\mainnet_unsigned.json"
$SIG = "$LOGDIR\mainnet_signed.json"

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

"========== MAINNET BOOT started $(Get-Date) (chain=$CHAIN) ==========" | Out-File $resultLog -Encoding ascii

# ---- FRESH HOME ----
Remove-Item $CHAINHOME -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $CHAINHOME -Force > $null

# 复制测试网 keyring-test（含 miner/validator 演示密钥，地址与 mainnet-genesis-config.json 一致）
# 生产环境应改为新建真实密钥，切勿复用演示密钥
Copy-Item "$LOGDIR\testnet\keyring-test" "$CHAINHOME\keyring-test" -Recurse -Force -ErrorAction SilentlyContinue

# ---- INIT (mainnet chain-id) ----
& $NODE 'init' 'mcnode' '--chain-id' $CHAIN '--home' $CHAINHOME 2>&1 | Out-File $resultLog -Append -Encoding ascii

# ---- ACCOUNTS + GENTX ----
& $NODE 'add-genesis-account' 'miner' '1000000000umc' '--home' $CHAINHOME '--keyring-backend' 'test' 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'add-genesis-account' 'validator' '100000000000umc' '--home' $CHAINHOME '--keyring-backend' 'test' 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'gentx' 'validator' '100000000000umc' '--min-self-delegation' '100000000000' '--home' $CHAINHOME '--keyring-backend' 'test' '--chain-id' $CHAIN 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'collect-gentxs' '--home' $CHAINHOME 2>&1 | Out-File $resultLog -Append -Encoding ascii

# ---- NORMALIZE PRODUCTION GENESIS (denom=umc / DePIN pool / cap / chain_id) ----
& $PY "$LOGDIR\scripts\make_genesis.py" --genesis "$CHAINHOME\config\genesis.json" --out "$CHAINHOME\config\genesis.json" --config "$LOGDIR\scripts\mainnet-genesis-config.json" 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'validate-genesis' '--home' $CHAINHOME 2>&1 | Out-File $resultLog -Append -Encoding ascii

# ---- 4s blocks, 0 gas price ----
& $PY "$LOGDIR\edit_configs.py" 2>&1 | Out-File $resultLog -Append -Encoding ascii

# ---- START NODE ----
$job = Start-Job -ScriptBlock { & '$HOME/mcchain\build\mcchaind.exe' start --home '$HOME/mcchain\mainnetboot' --log_level error *> '$HOME/mcchain\mainnet_node.log' }
"=== node job $($job.Id) ===" | Out-File $resultLog -Append -Encoding ascii
$ready = $false
for ($i = 0; $i -lt 60; $i++) { & $NODE 'status' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' > $null 2>&1; if ($LASTEXITCODE -eq 0) { $ready = $true; "=== ready ~$($i*2)s ===" | Out-File $resultLog -Append -Encoding ascii; break }; Start-Sleep -Seconds 2 }
if (-not $ready) { "=== NODE FAIL ===" | Out-File $resultLog -Append -Encoding ascii; Get-Content "$LOGDIR\mainnet_node.log" -Tail 30 | Out-File $resultLog -Append -Encoding ascii; Stop-Job -Job $job -ErrorAction SilentlyContinue; Remove-Job -Job $job -Force -ErrorAction SilentlyContinue; Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue; exit }

# fetch account number + sequence (wait for first committed block)
$accJson = $null
for ($k = 0; $k -lt 30; $k++) {
    $accJson = & $NODE 'q' 'auth' 'account' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json
    if ($accJson -and ($accJson.account_number -ne $null) -and ($accJson.account_number -ne '')) { break }
    Start-Sleep -Seconds 2
}
$ACC = $accJson.account_number
$SEQ = [int]$accJson.sequence
"=== miner account_number=$ACC sequence=$SEQ ===" | Out-File $resultLog -Append -Encoding ascii

"=== chain_id / heights ===" | Out-File $resultLog -Append -Encoding ascii
& $NODE 'status' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' 2>&1 | Out-File $resultLog -Append -Encoding ascii

"=== MINER BEFORE ===" | Out-File $resultLog -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding ascii
"=== DEPIN POOL BEFORE ===" | Out-File $resultLog -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' $DEPIN '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding ascii
"=== tokenomics supply ===" | Out-File $resultLog -Append -Encoding ascii
& $NODE 'q' 'tokenomics' 'supply' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding ascii

function BuildSignBroadcast($label, $cmdArgs) {
    "=== $label (acc=$ACC seq=$SEQ) ===" | Out-File $resultLog -Append -Encoding ascii
    & $NODE 'tx' @cmdArgs '--from' 'miner' '--home' $CHAINHOME '--chain-id' $CHAIN '--keyring-backend' 'test' '--account-number' $ACC '--sequence' $SEQ '--gas' '500000' '--gas-prices' '0umc' '--generate-only' '--output' 'json' 2>$null | Out-File $GEN -Encoding ascii
    & $NODE 'tx' 'sign' $GEN '--from' 'miner' '--home' $CHAINHOME '--chain-id' $CHAIN '--keyring-backend' 'test' '--account-number' $ACC '--sequence' $SEQ '--offline' '--output' 'json' 2>&1 | Out-File $SIG -Encoding ascii
    & $NODE 'tx' 'broadcast' $SIG '--node' 'tcp://localhost:26657' '--broadcast-mode' 'sync' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding ascii
    $script:SEQ = $script:SEQ + 1
    Start-Sleep -Seconds 2
}

BuildSignBroadcast 'STEP1 register-device' @('depin','register-device',$MINER,'pixel8','android')
BuildSignBroadcast 'STEP2 attest-device' @('depin','attest-device',$MINER,'ch123','sigabc')
BuildSignBroadcast 'STEP3 register-node' @('phonenode','register-node',$MINER,'pixel8','android','contributor')
BuildSignBroadcast 'STEP4 submit-attestation' @('phonenode','submit-attestation','roothash01','nonce01','devicehash01')
BuildSignBroadcast 'STEP5 submit-contribution task1 inference 80' @('depin','submit-contribution','task1','inference','80')

Start-Sleep -Seconds 3
"=== MINER AFTER ===" | Out-File $resultLog -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding ascii
"=== DEPIN POOL AFTER ===" | Out-File $resultLog -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' $DEPIN '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding ascii

$mjson = & $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json
$djson = & $NODE 'q' 'bank' 'balances' $DEPIN '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json
$mAmt = ($mjson.balances | Where-Object { $_.denom -eq 'umc' }).amount
$dAmt = ($djson.balances | Where-Object { $_.denom -eq 'umc' }).amount
"=== SUMMARY: miner umc = $mAmt ; depin pool umc = $dAmt ===" | Out-File $resultLog -Append -Encoding ascii
if ($mAmt -eq '1000000400') { "=== MAINNET MINING OK: +400 umc mined on $CHAIN ===" | Out-File $resultLog -Append -Encoding ascii } else { "=== MAINNET MINING MISMATCH ===" | Out-File $resultLog -Append -Encoding ascii }
"=== DONE $(Get-Date) ===" | Out-File $resultLog -Append -Encoding ascii

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Stop-Job -Job $job -ErrorAction SilentlyContinue; Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
"=== node stopped ===" | Out-File $resultLog -Append -Encoding ascii
