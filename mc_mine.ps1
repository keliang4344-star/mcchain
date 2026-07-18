$ErrorActionPreference = 'Continue'
$NODE = '$HOME/mcchain\build\mcchaind.exe'
$PY = 'python3'
$CHAINHOME = '$HOME/mcchain\testnet'
$CHAIN = 'mcchain-testnet-1'
$MINER = 'mc1zpzk062u54sv4j9w4qvlwkyjpxauqpuh72fz9w'
$DEPIN = 'mc1lpan9nughhevhdfdu0eywkzllgz39cu8a49jlp'
$LOGDIR = '$HOME/mcchain'
$resultLog = "$LOGDIR\mc_mine.log"
$GEN = "$LOGDIR\unsigned.json"
$SIG = "$LOGDIR\signed.json"
$TASKS = 3   # number of contributions to mine (task1..taskN)

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

"========== MC REPEAT-MINE started $(Get-Date) ==========" | Out-File $resultLog -Encoding ascii

# ---- helper: read umc balance ----
function GetUmc($addr) {
    $j = & $NODE 'q' 'bank' 'balances' $addr '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json
    $b = $j.balances | Where-Object { $_.denom -eq 'umc' }
    if ($b) { return [long]$b.amount } else { return [long]0 }
}

# ---- RESET (keep keyring-test) ----
Remove-Item "$CHAINHOME\config" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "$CHAINHOME\data" -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item "$CHAINHOME\genesis.json" -Force -ErrorAction SilentlyContinue
& $NODE 'init' 'mcnode' '--chain-id' $CHAIN '--home' $CHAINHOME 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $PY "$LOGDIR\fix_genesis.py" 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'add-genesis-account' 'miner' '1000000000umc' '--home' $CHAINHOME '--keyring-backend' 'test' 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'add-genesis-account' 'validator' '100000000000umc' '--home' $CHAINHOME '--keyring-backend' 'test' 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'gentx' 'validator' '100000000000umc' '--min-self-delegation' '100000000000' '--home' $CHAINHOME '--keyring-backend' 'test' '--chain-id' $CHAIN 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'collect-gentxs' '--home' $CHAINHOME 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $NODE 'validate-genesis' '--home' $CHAINHOME 2>&1 | Out-File $resultLog -Append -Encoding ascii
& $PY "$LOGDIR\edit_configs.py" 2>&1 | Out-File $resultLog -Append -Encoding ascii

# ---- START NODE ----
$job = Start-Job -ScriptBlock { & '$HOME/mcchain\build\mcchaind.exe' start --home '$HOME/mcchain\testnet' --log_level error *> '$HOME/mcchain\node_job.log' }
"=== node job $($job.Id) ===" | Out-File $resultLog -Append -Encoding ascii
$ready = $false
for ($i = 0; $i -lt 60; $i++) { & $NODE 'status' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' > $null 2>&1; if ($LASTEXITCODE -eq 0) { $ready = $true; "=== ready ~$($i*2)s ===" | Out-File $resultLog -Append -Encoding ascii; break }; Start-Sleep -Seconds 2 }
if (-not $ready) { "=== NODE FAIL ===" | Out-File $resultLog -Append -Encoding ascii; Get-Content "$LOGDIR\node_job.log" -Tail 30 | Out-File $resultLog -Append -Encoding ascii; Stop-Job -Job $job -ErrorAction SilentlyContinue; Remove-Job -Job $job -Force -ErrorAction SilentlyContinue; Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue; exit }

# fetch account number + sequence (wait until first block committed)
$accJson = $null
for ($k = 0; $k -lt 30; $k++) {
    $accJson = & $NODE 'q' 'auth' 'account' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json
    if ($accJson -and ($accJson.account_number -ne $null) -and ($accJson.account_number -ne '')) { break }
    Start-Sleep -Seconds 2
}
$ACC = $accJson.account_number
$SEQ = [int]$accJson.sequence
"=== miner account_number=$ACC sequence=$SEQ ===" | Out-File $resultLog -Append -Encoding ascii

"=== MINER BEFORE = $(GetUmc $MINER) ===" | Out-File $resultLog -Append -Encoding ascii
"=== DEPIN POOL BEFORE = $(GetUmc $DEPIN) ===" | Out-File $resultLog -Append -Encoding ascii

# ---- offline sign + broadcast (reliable path) ----
function BuildSignBroadcast($label, $cmdArgs) {
    "=== $label (acc=$ACC seq=$SEQ) ===" | Out-File $resultLog -Append -Encoding ascii
    & $NODE 'tx' @cmdArgs '--from' 'miner' '--home' $CHAINHOME '--chain-id' $CHAIN '--keyring-backend' 'test' '--account-number' $ACC '--sequence' $SEQ '--gas' '500000' '--gas-prices' '0umc' '--generate-only' '--output' 'json' 2>$null | Out-File $GEN -Encoding ascii
    & $NODE 'tx' 'sign' $GEN '--from' 'miner' '--home' $CHAINHOME '--chain-id' $CHAIN '--keyring-backend' 'test' '--account-number' $ACC '--sequence' $SEQ '--offline' '--output' 'json' 2>&1 | Out-File $SIG -Encoding ascii
    $bc = & $NODE 'tx' 'broadcast' $SIG '--node' 'tcp://localhost:26657' '--broadcast-mode' 'sync' '--output' 'json' 2>&1
    $bc | Out-File $resultLog -Append -Encoding ascii
    try { $bcJson = $bc | ConvertFrom-Json; "    -> broadcast code=$($bcJson.code) txhash=$($bcJson.txhash)" | Out-File $resultLog -Append -Encoding ascii } catch { "    -> broadcast parse failed" | Out-File $resultLog -Append -Encoding ascii }
    $script:SEQ = $script:SEQ + 1
    Start-Sleep -Seconds 2
}

# ---- 5-step registration/attestation (once) ----
BuildSignBroadcast 'STEP1 register-device' @('depin','register-device',$MINER,'pixel8','android')
BuildSignBroadcast 'STEP2 attest-device' @('depin','attest-device',$MINER,'ch123','sigabc')
BuildSignBroadcast 'STEP3 register-node' @('phonenode','register-node',$MINER,'pixel8','android','contributor')
BuildSignBroadcast 'STEP4 submit-attestation' @('phonenode','submit-attestation','roothash01','nonce01','devicehash01')

# ---- mine N contributions, verifying each payout by polling balance ----
"=== MINING LOOP (expect +400 umc per task) ===" | Out-File $resultLog -Append -Encoding ascii
$okCount = 0
for ($t = 1; $t -le $TASKS; $t++) {
    $taskId = "task$t"
    $before = GetUmc $MINER
    BuildSignBroadcast "STEP5 submit-contribution $taskId inference 80" @('depin','submit-contribution',$taskId,'inference','80')
    # wait until balance increases (up to ~12s / 3 blocks)
    $after = $before
    for ($w = 0; $w -lt 6; $w++) {
        Start-Sleep -Seconds 2
        $after = GetUmc $MINER
        if ($after -gt $before) { break }
    }
    $delta = $after - $before
    $poolAfter = GetUmc $DEPIN
    $status = if ($delta -eq 400) { 'OK' } else { 'FAIL' }
    if ($delta -eq 400) { $okCount++ }
    "    -> $taskId : before=$before after=$after delta=$delta pool=$poolAfter [$status]" | Out-File $resultLog -Append -Encoding ascii
}

# ---- final summary ----
$mFinal = GetUmc $MINER
$dFinal = GetUmc $DEPIN
"=== FINAL: miner umc=$mFinal ; depin pool umc=$dFinal ; paid tasks=$okCount/$TASKS ===" | Out-File $resultLog -Append -Encoding ascii
if ($okCount -eq $TASKS) { "=== REPEAT MINING OK: every contribution paid 400 umc ===" | Out-File $resultLog -Append -Encoding ascii }
else { "=== REPEAT MINING PARTIAL/FAIL: only $okCount/$TASKS paid ===" | Out-File $resultLog -Append -Encoding ascii }
"=== DONE $(Get-Date) ===" | Out-File $resultLog -Append -Encoding ascii

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Stop-Job -Job $job -ErrorAction SilentlyContinue; Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
"=== node stopped ===" | Out-File $resultLog -Append -Encoding ascii
