$ErrorActionPreference = 'Continue'
$NODE = '$HOME/mcchain\build\mcchaind.exe'
$CHAINHOME = '$HOME/mcchain\testnet'
$CHAIN = 'mcchain-testnet-1'
$MINER = 'mc1zpzk062u54sv4j9w4qvlwkyjpxauqpuh72fz9w'
$DEPIN = 'mc1lpan9nughhevhdfdu0eywkzllgz39cu8a49jlp'
$LOGDIR = '$HOME/mcchain'
$nodeLog = "$LOGDIR\node_job.log"
$resultLog = "$LOGDIR\mining_result.log"

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

"=== Mining flow v3 started at $(Get-Date) ===" | Out-File $resultLog -Encoding utf8

$job = Start-Job -ScriptBlock {
    & '$HOME/mcchain\build\mcchaind.exe' start --home '$HOME/mcchain\testnet' --log_level error *> '$HOME/mcchain\node_job.log'
}
"=== Node job started id=$($job.Id) ===" | Out-File $resultLog -Append -Encoding utf8

$ready = $false
for ($i = 0; $i -lt 60; $i++) {
    & $NODE 'status' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' > $null 2>&1
    if ($LASTEXITCODE -eq 0) { $ready = $true; "=== ready ~$($i*2)s ===" | Out-File $resultLog -Append -Encoding utf8; break }
    Start-Sleep -Seconds 2
}
if (-not $ready) {
    "=== NODE FAIL ===" | Out-File $resultLog -Append -Encoding utf8
    Get-Content $nodeLog -Tail 40 | Out-File $resultLog -Append -Encoding utf8
    Stop-Job -Job $job -ErrorAction SilentlyContinue; Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
    Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    exit
}

# Fetch miner account (account_number + sequence) fresh
$accJson = & $NODE 'q' 'auth' 'account' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json
$ACC = $accJson.account_number
$SEQ = [int]$accJson.sequence
"=== miner account_number=$ACC sequence=$SEQ ===" | Out-File $resultLog -Append -Encoding utf8

# Balances before
"=== MINER BEFORE ===" | Out-File $resultLog -Append -Encoding utf8
& $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding utf8
"=== DEPIN POOL BEFORE ===" | Out-File $resultLog -Append -Encoding utf8
& $NODE 'q' 'bank' 'balances' $DEPIN '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding utf8

function Run-Tx($argsArr) {
    & $NODE 'tx' @argsArr '--from' 'miner' '--home' $CHAINHOME '--chain-id' $CHAIN '--keyring-backend' 'test' '--account-number' $ACC '--sequence' $SEQ '--gas' '500000' '--gas-prices' '0umc' '-y' '--broadcast-mode' 'sync' '--output' 'json' '--node' 'tcp://localhost:26657' 2>&1 | Out-File $resultLog -Append -Encoding utf8
    $global:SEQ = $global:SEQ + 1
    Start-Sleep -Seconds 1
}

"=== STEP1 register-device ===" | Out-File $resultLog -Append -Encoding utf8
Run-Tx @('depin','register-device',$MINER,'pixel8','android')

"=== STEP2 attest-device ===" | Out-File $resultLog -Append -Encoding utf8
Run-Tx @('depin','attest-device',$MINER,'ch123','sigabc')

"=== STEP3 register-node ===" | Out-File $resultLog -Append -Encoding utf8
Run-Tx @('phonenode','register-node',$MINER,'pixel8','android','contributor')

"=== STEP4 submit-attestation ===" | Out-File $resultLog -Append -Encoding utf8
Run-Tx @('phonenode','submit-attestation','roothash01','nonce01','devicehash01')

"=== STEP5 submit-contribution task1 inference 80 ===" | Out-File $resultLog -Append -Encoding utf8
Run-Tx @('depin','submit-contribution','task1','inference','80')

Start-Sleep -Seconds 3

"=== MINER AFTER ===" | Out-File $resultLog -Append -Encoding utf8
& $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding utf8
"=== DEPIN POOL AFTER ===" | Out-File $resultLog -Append -Encoding utf8
& $NODE 'q' 'bank' 'balances' $DEPIN '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $resultLog -Append -Encoding utf8

$mjson = & $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json
$djson = & $NODE 'q' 'bank' 'balances' $DEPIN '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json
$mAmt = ($mjson.balances | Where-Object { $_.denom -eq 'umc' }).amount
$dAmt = ($djson.balances | Where-Object { $_.denom -eq 'umc' }).amount
"=== SUMMARY: miner umc = $mAmt ; depin pool umc = $dAmt ===" | Out-File $resultLog -Append -Encoding utf8
"=== DONE at $(Get-Date) ===" | Out-File $resultLog -Append -Encoding utf8

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Stop-Job -Job $job -ErrorAction SilentlyContinue; Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
"=== Node stopped ===" | Out-File $resultLog -Append -Encoding utf8
