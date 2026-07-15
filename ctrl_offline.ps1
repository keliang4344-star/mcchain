$ErrorActionPreference = 'Continue'
$NODE = '$HOME/mcchain\build\mcchaind.exe'
$CHAINHOME = '$HOME/mcchain\testnet'
$CHAIN = 'mcchain-testnet-1'
$MINER = 'mc1zpzk062u54sv4j9w4qvlwkyjpxauqpuh72fz9w'
$LOGDIR = '$HOME/mcchain'
$out = "$LOGDIR\ctrl_offline.log"
$GEN = "$LOGDIR\unsigned.json"
$SIG = "$LOGDIR\signed.json"

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
$job = Start-Job -ScriptBlock { & '$HOME/mcchain\build\mcchaind.exe' start --home '$HOME/mcchain\testnet' --log_level error *> '$HOME/mcchain\node_job.log' }
"=== ctrl_offline started $(Get-Date) ===" | Out-File $out -Encoding ascii
$ready = $false
for ($i = 0; $i -lt 60; $i++) { & $NODE 'status' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' > $null 2>&1; if ($LASTEXITCODE -eq 0) { $ready = $true; break }; Start-Sleep -Seconds 2 }
$accJson = $null
for ($k = 0; $k -lt 30; $k++) { $accJson = & $NODE 'q' 'auth' 'account' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json; if ($accJson -and $accJson.account_number -ne $null) { break }; Start-Sleep -Seconds 2 }
$ACC = $accJson.account_number
$SEQ = [int]$accJson.sequence
"=== miner account_number=$ACC sequence=$SEQ ===" | Out-File $out -Append -Encoding ascii

"=== MINER BEFORE ===" | Out-File $out -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $out -Append -Encoding ascii

# OFFLINE sign + broadcast task2 (control)
"=== OFFLINE submit-contribution task2 inference 80 (acc=$ACC seq=$SEQ) ===" | Out-File $out -Append -Encoding ascii
& $NODE 'tx' 'depin' 'submit-contribution' 'task2' 'inference' '80' '--from' 'miner' '--home' $CHAINHOME '--chain-id' $CHAIN '--keyring-backend' 'test' '--account-number' $ACC '--sequence' $SEQ '--gas' '500000' '--gas-prices' '0umc' '--generate-only' '--output' 'json' 2>$null | Out-File $GEN -Encoding ascii
& $NODE 'tx' 'sign' $GEN '--from' 'miner' '--home' $CHAINHOME '--chain-id' $CHAIN '--keyring-backend' 'test' '--account-number' $ACC '--sequence' $SEQ '--offline' '--output' 'json' 2>&1 | Out-File $SIG -Encoding ascii
& $NODE 'tx' 'broadcast' $SIG '--node' 'tcp://localhost:26657' '--broadcast-mode' 'sync' '--output' 'json' 2>&1 | Out-File $out -Append -Encoding ascii
Start-Sleep -Seconds 5

"=== MINER AFTER ===" | Out-File $out -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $out -Append -Encoding ascii

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Stop-Job -Job $job -ErrorAction SilentlyContinue; Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
"=== done ===" | Out-File $out -Append -Encoding ascii
