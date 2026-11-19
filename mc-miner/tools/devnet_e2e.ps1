# devnet E2E：新建一次性测试账户 → 注资 → 运行依赖链上状态的集成测试。
#
# 为什么每次要新账户：phonenode 节点注册后 100 块（≈400s）不心跳会被离线 slash，
# attestation 吊销 + 43200 块冷却期内禁止重认证，旧账户短期内无法再跑
# 贡献 / EdgeAI 用例。心跳本身由 App MinerService 维持，测试进程退出即停跳。
#
# 用法（需 devnet 已在跑：bash scripts/devnet.sh start）：
#   powershell -File mc-miner\tools\devnet_e2e.ps1
$ErrorActionPreference = 'Stop'
$RepoRoot = if ($PSScriptRoot) { Split-Path (Split-Path $PSScriptRoot -Parent) -Parent } else { 'E:\mc-build\mcchain' }
$GitBash = 'C:\Program Files\Git\bin\bash.exe'
if (-not (Test-Path $GitBash)) { throw "未找到 git-bash：$GitBash" }

$name = "e2e-$(Get-Date -Format 'HHmmss')"
Write-Host "== 创建测试账户 $name" -ForegroundColor Cyan
$keys = & $GitBash -lc "cd /e/mc-build/mcchain 2>/dev/null || cd '$($RepoRoot -replace '\\','/')'; ./build/mcchaind.exe keys add $name --keyring-backend test --home .devnet/node0 2>/dev/null | grep -oE 'mc1[a-z0-9]+' | head -1"
$addr = ($keys | Select-String 'mc1').ToString().Trim()
if (-not $addr) { throw "创建账户失败（devnet 是否在跑？）" }

$priv = & $GitBash -lc "cd '$($RepoRoot -replace '\\','/')' && echo y | ./build/mcchaind.exe keys export $name --unsafe --unarmored-hex --keyring-backend test --home .devnet/node0 2>/dev/null"
$priv = ($priv | Select-String '^[0-9a-f]{64}$').ToString().Trim()
if (-not $priv) { throw "导出私钥失败" }

Write-Host "== 注资 1,000,000 MC → $addr" -ForegroundColor Cyan
& $GitBash -lc "cd '$($RepoRoot -replace '\\','/')' && bash scripts/devnet.sh fund $addr 1000000000000 >/dev/null 2>&1"
Start-Sleep -Seconds 6

$env:MC_DEVNET_REST = 'http://127.0.0.1:1317'
$env:MC_DEVNET_PRIVKEY = $priv
$env:MC_DEVNET_ADDRESS = $addr
Write-Host "== 运行 devnet 集成测试（EdgeAI 用例含 ~8 分钟结算等待）" -ForegroundColor Cyan
& "$env:USERPROFILE\.gradle\wrapper\dists\gradle-8.7-bin\bhs2wmbdwecv87pi65oeuq5iu\gradle-8.7\bin\gradle.bat" -p "$RepoRoot\mc-miner" :shared:jvmTest --no-daemon --console=plain
$code = $LASTEXITCODE

Get-ChildItem "$RepoRoot\mc-miner\shared\build\test-results\jvmTest\*.xml" | ForEach-Object {
    # 用正则而非 [xml] 解析：失败用例的 raw_log 可能含非法 XML 字符，cast 会抛错。
    $head = Get-Content $_.FullName -TotalCount 2 | Out-String
    if ($head -match 'tests="(\d+)" skipped="(\d+)" failures="(\d+)" errors="(\d+)"') {
        "{0}: tests={1} skipped={2} failures={3} errors={4}" -f $_.Name, $Matches[1], $Matches[2], $Matches[3], $Matches[4]
    } else {
        "{0}: (无法解析统计头)" -f $_.Name
    }
}
exit $code
