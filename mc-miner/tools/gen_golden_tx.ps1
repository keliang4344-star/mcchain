#requires -Version 5.1
<#
  生成 shared 的 golden tx 对照向量（Phase 0/1 验收用）。

  必须使用链仓库源码构建出的 mcchaind（与链上 Go 代码同版本），流程：
    1. 临时 home 中导入固定测试私钥
    2. --generate-only 生成未签名交易
    3. tx sign --sign-mode direct --offline 离线签名
    4. tx encode 输出 TxRaw base64
    5. 解析 TxRaw，重组 SignDoc 并计算摘要，写出 JSON

  用法（在 mc-miner 目录）：
    powershell -ExecutionPolicy Bypass -File tools/gen_golden_tx.ps1 -Kind register_device
    powershell -ExecutionPolicy Bypass -File tools/gen_golden_tx.ps1 -Kind bank_send

  注意：临时 keyring 目录保留在系统临时目录（脚本不删除目录，避免误删风险），
  如不再需要可手工删除输出中打印的 tempHome。
#>
param(
    [string]$Mcchaind = "",
    [ValidateSet("register_device", "bank_send")]
    [string]$Kind = "register_device"
)

$ErrorActionPreference = "Stop"

$minerRoot = Split-Path -Parent $PSScriptRoot
$chainRoot = Split-Path -Parent $minerRoot

if (-not $Mcchaind) {
    foreach ($candidate in @(
            (Join-Path $chainRoot "build\mcchaind.exe"),
            (Join-Path $chainRoot "build\mcchaind"))) {
        if (Test-Path $candidate) { $Mcchaind = $candidate; break }
    }
}
if (-not $Mcchaind -or -not (Test-Path $Mcchaind)) {
    throw "找不到 mcchaind，请用 -Mcchaind 指定链仓库构建产物路径"
}
$Mcchaind = (Resolve-Path $Mcchaind).Path

# 固定测试私钥 = SHA-256("mcchain-golden-tx-v1-do-not-use")。
# 仅用于 golden vector，禁止向对应地址转入任何资产。
$privateKeyHex = "e7a3158f08d4f8474b59db0769151e5d64eee6216c482afc30895dfdf6e3e6e3"

$chainId       = "mcchain-dev-1"
$accountNumber = 0
$sequence      = 0
$feeDenom      = "umc"
$feeAmount     = "5000"
$gasLimit      = 200000
$model         = "Pixel 8 Pro"
$deviceOs      = "Android 15"
# 固定收款测试私钥 = SHA-256("mcchain-golden-tx-recipient-v1-do-not-use")，仅用于 golden vector。
$recipientPrivateKeyHex = "503679372450b104edd9175521db0436fde715490c8bcaa4dd932c879a0d0beb"

function Invoke-Cli {
    param([string[]]$CliArgs)
    $output = & $Mcchaind @CliArgs 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "mcchaind $($CliArgs -join ' ') 失败（exit $LASTEXITCODE）：`n$($output -join "`n")"
    }
    return ($output -join "`n")
}

function Write-Utf8NoBom {
    param([string]$Path, [string]$Text)
    $dir = Split-Path -Parent $Path
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }
    [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Read-Varint {
    param([byte[]]$Bytes, [hashtable]$State)
    $shift = 0
    $value = [long]0
    while ($true) {
        $b = $Bytes[$State.i]
        $State.i++
        $value = $value -bor ([long]($b -band 0x7F) -shl $shift)
        if (($b -band 0x80) -eq 0) { return $value }
        $shift += 7
        if ($shift -gt 63) { throw "varint 过长" }
    }
}

function Add-Varint {
    param([System.Collections.Generic.List[byte]]$List, [long]$Value)
    $v = $Value
    while ($true) {
        if (($v -band [long]0x7FFFFFFFFFFFFF80) -eq 0) {
            $List.Add([byte]$v)
            return
        }
        $List.Add([byte](($v -band 0x7F) -bor 0x80))
        $v = $v -shr 7
    }
}

function Add-BytesField {
    param([System.Collections.Generic.List[byte]]$List, [int]$Field, [byte[]]$Value)
    Add-Varint $List ([long](($Field -shl 3) -bor 2))
    Add-Varint $List ([long]$Value.Length)
    $List.AddRange($Value)
}

function ConvertTo-Hex {
    param([byte[]]$Bytes)
    return (($Bytes | ForEach-Object { $_.ToString("x2") }) -join "")
}

$tempHome = Join-Path ([System.IO.Path]::GetTempPath()) ("mc-golden-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempHome | Out-Null

Invoke-Cli -CliArgs @(
    "keys", "import-hex", "golden", $privateKeyHex,
    "--keyring-backend", "test", "--home", $tempHome
) | Out-Null
$address = (Invoke-Cli -CliArgs @(
    "keys", "show", "golden", "-a", "--keyring-backend", "test", "--home", $tempHome
)).Trim()

Invoke-Cli -CliArgs @(
    "keys", "import-hex", "recipient", $recipientPrivateKeyHex,
    "--keyring-backend", "test", "--home", $tempHome
) | Out-Null
$recipient = (Invoke-Cli -CliArgs @(
    "keys", "show", "recipient", "-a", "--keyring-backend", "test", "--home", $tempHome
)).Trim()

switch ($Kind) {
    "register_device" {
        $memo = "golden-register-device-v1"
        $outFile = Join-Path $minerRoot "shared\src\jvmTest\resources\golden\register_device.json"
        $msgArgs = @("tx", "depin", "register-device", $address, $model, $deviceOs)
        $msgJson = [ordered]@{
            typeUrl = "/mcchain.depin.MsgRegisterDevice"
            creator = $address
            address = $address
            model   = $model
            os      = $deviceOs
        }
    }
    "bank_send" {
        $memo = "golden-bank-send-v1"
        $outFile = Join-Path $minerRoot "shared\src\jvmTest\resources\golden\bank_send.json"
        $msgArgs = @("tx", "bank", "send", $address, $recipient, "1234umc")
        $msgJson = [ordered]@{
            typeUrl   = "/cosmos.bank.v1beta1.MsgSend"
            fromAddress = $address
            toAddress = $recipient
            amount    = [ordered]@{ denom = $feeDenom; amount = "1234" }
        }
    }
}

$unsignedPath = Join-Path $tempHome "unsigned.json"
$unsigned = Invoke-Cli -CliArgs ($msgArgs + @(
        "--from", "golden", "--keyring-backend", "test", "--home", $tempHome,
        "--chain-id", $chainId, "--account-number", "$accountNumber", "--sequence", "$sequence",
        "--generate-only", "--fees", "$feeAmount$feeDenom", "--gas", "$gasLimit",
        "--note", $memo, "--output", "json"
    ))
Write-Utf8NoBom $unsignedPath $unsigned

$signedPath = Join-Path $tempHome "signed.json"
$signedJson = Invoke-Cli -CliArgs @(
    "tx", "sign", $unsignedPath,
    "--from", "golden", "--keyring-backend", "test", "--home", $tempHome,
    "--chain-id", $chainId, "--account-number", "$accountNumber", "--sequence", "$sequence",
    "--sign-mode", "direct", "--offline", "--output", "json"
)
Write-Utf8NoBom $signedPath $signedJson
$signed = $signedJson | ConvertFrom-Json

$txRawBase64 = (Invoke-Cli -CliArgs @("tx", "encode", $signedPath)).Trim()
$txRaw = [Convert]::FromBase64String($txRawBase64)

$state = @{ i = 0 }
$fields = @{}
while ($state.i -lt $txRaw.Length) {
    $tag = Read-Varint -Bytes $txRaw -State $state
    $field = [int]($tag -shr 3)
    $wire = [int]($tag -band 7)
    switch ($wire) {
        2 {
            $len = [int](Read-Varint -Bytes $txRaw -State $state)
            $value = New-Object byte[] $len
            [Array]::Copy($txRaw, $state.i, $value, 0, $len)
            $state.i += $len
        }
        0 {
            $value = Read-Varint -Bytes $txRaw -State $state
        }
        default {
            throw "不支持的 wire type：$wire"
        }
    }
    $fields[$field] = $value
}

$body = [byte[]]$fields[1]
$authInfo = [byte[]]$fields[2]
$signature = [byte[]]$fields[3]

$signatureHex = ConvertTo-Hex -Bytes $signature
$expectedSignatureHex = ConvertTo-Hex -Bytes ([Convert]::FromBase64String($signed.signatures[0]))
if ($signatureHex -ne $expectedSignatureHex) {
    throw "TxRaw 中的签名与 sign 输出不一致"
}

$publicKeyHex = ConvertTo-Hex -Bytes ([Convert]::FromBase64String($signed.auth_info.signer_infos[0].public_key.key))

$signDoc = New-Object System.Collections.Generic.List[byte]
Add-BytesField -List $signDoc -Field 1 -Value $body
Add-BytesField -List $signDoc -Field 2 -Value $authInfo
Add-BytesField -List $signDoc -Field 3 -Value ([System.Text.Encoding]::UTF8.GetBytes($chainId))
if ($accountNumber -ne 0) {
    $signDoc.Add([byte]0x20)
    Add-Varint $signDoc ([long]$accountNumber)
}
$signDocBytes = $signDoc.ToArray()

$sha = [System.Security.Cryptography.SHA256]::Create()
$signDocSha256 = ConvertTo-Hex -Bytes ($sha.ComputeHash($signDocBytes))

$vector = [ordered]@{
    schema           = "mc-golden-tx/v1"
    kind             = $Kind
    comment          = "TEST ONLY：由 mcchaind 离线签名生成，用于校验 shared 的 TxEncoder/TxSigner 字节一致；私钥为公开测试值，禁止转入资产。"
    chainId          = $chainId
    accountNumber    = $accountNumber
    sequence         = $sequence
    memo             = $memo
    timeoutHeight    = 0
    fee              = [ordered]@{ denom = $feeDenom; amount = $feeAmount; gasLimit = $gasLimit }
    msg              = $msgJson
    privateKeyHex    = $privateKeyHex
    publicKeyHex     = $publicKeyHex
    address          = $address
    bodyBytesHex     = (ConvertTo-Hex -Bytes $body)
    authInfoBytesHex = (ConvertTo-Hex -Bytes $authInfo)
    signDocHex       = (ConvertTo-Hex -Bytes $signDocBytes)
    signDocSha256Hex = $signDocSha256
    signatureHex     = $signatureHex
    txRawBase64      = $txRawBase64
    txRawHex         = (ConvertTo-Hex -Bytes $txRaw)
    txHashHex        = (ConvertTo-Hex -Bytes ($sha.ComputeHash($txRaw))).ToUpperInvariant()
}

Write-Utf8NoBom $outFile ($vector | ConvertTo-Json -Depth 6)

Write-Host "已生成（$Kind）：$outFile"
Write-Host "  address  = $address"
Write-Host "  txHash   = $($vector.txHashHex)"
Write-Host "  tempHome = $tempHome"
