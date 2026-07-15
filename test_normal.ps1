$ErrorActionPreference = 'Continue'
$NODE = '$HOME/mcchain\build\mcchaind.exe'
$CHAINHOME = '$HOME/mcchain\testnet'
$CHAIN = 'mcchain-testnet-1'
$MINER = 'mc1zpzk062u54sv4j9w4qvlwkyjpxauqpuh72fz9w'
$out = '$HOME/mcchain\test_normal.log'

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
$job = Start-Job -ScriptBlock { & '$HOME/mcchain\build\mcchaind.exe' start --home '$HOME/mcchain\testnet' --log_level error *> '$HOME/mcchain\node_job.log' }
"=== test_normal started $(Get-Date) ===" | Out-File $out -Encoding ascii
$ready = $false
for ($i = 0; $i -lt 60; $i++) { & $NODE 'status' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' > $null 2>&1; if ($LASTEXITCODE -eq 0) { $ready = $true; break }; Start-Sleep -Seconds 2 }
# wait for first block
$accJson = $null
for ($k = 0; $k -lt 30; $k++) { $accJson = & $NODE 'q' 'auth' 'account' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>$null | ConvertFrom-Json; if ($accJson -and $accJson.account_number -ne $null) { break }; Start-Sleep -Seconds 2 }

"=== MINER BEFORE (normal path) ===" | Out-File $out -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $out -Append -Encoding ascii
"=== DEPIN POOL BEFORE ===" | Out-File $out -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' 'mc1lpan9nughhevhdfdu0eywkzllgz39cu8a49jlp' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $out -Append -Encoding ascii

# NORMAL tx (no explicit account-number/sequence) -- does the standard CLI work?
"=== NORMAL tx submit-contribution task2 inference 80 ===" | Out-File $out -Append -Encoding ascii
& $NODE 'tx' 'depin' 'submit-contribution' 'task2' 'inference' '80' '--from' 'miner' '--home' $CHAINHOME '--chain-id' $CHAIN '--keyring-backend' 'test' '--gas' '500000' '--gas-prices' '0umc' '-y' '--broadcast-mode' 'sync' '--output' 'json' '--node' 'tcp://localhost:26657' 2>&1 | Out-File $out -Append -Encoding ascii
Start-Sleep -Seconds 8

"=== MINER AFTER (normal path) ===" | Out-File $out -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' $MINER '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $out -Append -Encoding ascii
"=== DEPIN POOL AFTER ===" | Out-File $out -Append -Encoding ascii
& $NODE 'q' 'bank' 'balances' 'mc1lpan9nughhevhdfdu0eywkzllgz39cu8a49jlp' '--home' $CHAINHOME '--node' 'tcp://localhost:26657' '--output' 'json' 2>&1 | Out-File $out -Append -Encoding ascii

Get-Process -Name mcchaind -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Stop-Job -Job $job -ErrorAction SilentlyContinue; Remove-Job -Job $job -Force -ErrorAction SilentlyContinue
"=== done ===" | Out-File $out -Append -Encoding ascii
