<#
.SYNOPSIS
    修改 Noder 数据库中的管理员 Token。

.DESCRIPTION
    强制覆盖 SQLite appsetting 表中的管理员 Token，不要求旧 Token。
    仅允许通过本机 localhost 接口执行。

.EXAMPLE
    .\scripts\change-admin-password.ps1

.EXAMPLE
    .\scripts\change-admin-password.ps1 -BaseUrl 'http://127.0.0.1:8000'
#>

[CmdletBinding()]
param(
    [string]$BaseUrl = 'http://127.0.0.1:8000'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Read-SecretText([string]$Prompt) {
    $secure = Read-Host $Prompt -AsSecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
    } finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
    }
}

$newToken = Read-SecretText '请输入新的管理员 Token'
$confirmation = Read-SecretText '请再次输入新的管理员 Token'

if ([string]::IsNullOrWhiteSpace($newToken)) {
    throw '新的管理员 Token 不能为空。'
}
if ($newToken -ne $confirmation) {
    throw '两次输入的管理员 Token 不一致。'
}
if ($newToken -notmatch '^[A-Za-z0-9._~+/=-]+$') {
    throw 'Token 只能包含字母、数字及 . _ ~ + / = -'
}

$body = @{ token = $newToken } | ConvertTo-Json
$uri = "$($BaseUrl.TrimEnd('/'))/api/auth/force-change"

try {
    Invoke-RestMethod -Uri $uri -Method Post -ContentType 'application/json' -Body $body | Out-Null
} catch {
    throw "管理员 Token 修改失败：$($_.Exception.Message)"
}

Write-Host '管理员 Token 已写入数据库，立即生效。'
Write-Host '无需修改环境变量，也无需重启服务。'
