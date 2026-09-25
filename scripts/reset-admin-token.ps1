# ...existing code...
<#
.SYNOPSIS
    重置 Noder 管理员 API Token
.DESCRIPTION
    默认先尝试使用 %ProgramData%\noder\noder.env；如果不存在，则回退到仓库中的示例配置文件。
.EXAMPLE
    .\scripts\reset-admin-token.ps1
.EXAMPLE
    .\scripts\reset-admin-token.ps1 -Token "abc123" -EnvFile "C:\etc\noder\noder.env"
.EXAMPLE
    .\scripts\reset-admin-token.ps1 -NoRestart
#>

[CmdletBinding()]
param(
    [string]$Token,
    [string]$EnvFile,
    [switch]$NoRestart,
    [switch]$Help
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Show-Usage {
    @"
用法：.\scripts\reset-admin-token.ps1 [选项]

选项：
  -Token <值>        使用指定 Token；省略时自动生成随机 Token
  -EnvFile <路径>     环境配置文件
  -NoRestart         只修改配置，不重启 noder 服务
  -Help              显示帮助
"@
}

function New-RandomToken {
    $bytes = New-Object byte[] 24
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    $rng.GetBytes($bytes)

    $hex = ($bytes | ForEach-Object { '{0:x2}' -f $_ }) -join ''
    return $hex
}

function Resolve-EnvFile {
    param(
        [string]$ExplicitEnvFile
    )

    if ($ExplicitEnvFile) {
        return $ExplicitEnvFile
    }

    $defaultEnv = Join-Path $env:ProgramData 'noder\noder.env'
    if (Test-Path -LiteralPath $defaultEnv -PathType Leaf) {
        return $defaultEnv
    }

    $repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
    $sampleEnv = Join-Path $repoRoot 'package\etc\noder\noder.env'

    if (Test-Path -LiteralPath $sampleEnv -PathType Leaf) {
        $dir = Split-Path -Parent $defaultEnv
        if (-not (Test-Path -LiteralPath $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }

        Copy-Item -LiteralPath $sampleEnv -Destination $defaultEnv -Force
        return $defaultEnv
    }

    return $defaultEnv
}

if ($Help) {
    Show-Usage
    exit 0
}

$EnvFile = Resolve-EnvFile -ExplicitEnvFile $EnvFile

if (-not $Token) {
    $Token = New-RandomToken
}

if ($Token -notmatch '^[A-Za-z0-9._~+/=-]+$') {
    throw "错误：Token 只能包含字母、数字及 . _ ~ + / = -"
}

if (-not (Test-Path -LiteralPath $EnvFile -PathType Leaf)) {
    throw "错误：配置文件不存在：$EnvFile`n可先复制 package/etc/noder/noder.env 到该路径。"
}

$content = Get-Content -LiteralPath $EnvFile -Raw
$pattern = '(?m)^(?<prefix>\s*)ADMIN_SECRET_TOKEN=.*$'

if ($content -match $pattern) {
    $updated = [regex]::Replace($content, $pattern, "ADMIN_SECRET_TOKEN=$Token", 1)
} else {
    if ($content -notmatch '(?:\r?\n)\s*$') {
        $content += "`r`n"
    }
    $updated = $content + "ADMIN_SECRET_TOKEN=$Token`r`n"
}

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($EnvFile, $updated, $utf8NoBom)

if (-not $NoRestart) {
    $service = Get-Service -Name 'noder' -ErrorAction SilentlyContinue
    if (-not $service) {
        $service = Get-CimInstance -ClassName Win32_Service -Filter "Name='noder.service'" -ErrorAction SilentlyContinue
    }

    if ($service) {
        Restart-Service -Name $service.Name -Force
        Write-Host "管理员 Token 已重置，noder.service 已重启。"
    } else {
        Write-Host "管理员 Token 已写入：$EnvFile"
        Write-Host "请重启 Noder 服务使配置生效。"
    }
} else {
    Write-Host "管理员 Token 已写入：$EnvFile"
    Write-Host "未重启服务（-NoRestart）。"
}

Write-Host "新的管理员 Token：$Token"
# ...existing code...