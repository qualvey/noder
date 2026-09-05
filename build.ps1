<#
.SYNOPSIS
    Noder 自动化构建与重启脚本 (Windows PowerShell)

.DESCRIPTION
    支持一键完成前端 Vue 打包、Go 后端编译与服务重启，自动解决 Windows 进程占用文件锁定问题。

.PARAMETER FrontendOnly
    仅构建前端静态资源 (pnpm build)

.PARAMETER BackendOnly
    仅编译 Go 后端二进制 (go build)

.PARAMETER Restart
    编译完成后自动重启后台服务 (安全杀掉占用端口的旧进程并拉起新进程)

.PARAMETER Clean
    清理历史构建缓存与编译产物

.EXAMPLE
    .\build.ps1                 # 全量构建前端与后端
    .\build.ps1 -Restart        # 全量构建并立即重启运行服务
    .\build.ps1 -r              # 简写，全量构建并重启
    .\build.ps1 -BackendOnly -r # 仅编译后端并重启服务
    .\build.ps1 -FrontendOnly   # 仅编译前端静态资源
#>

[CmdletBinding()]
param(
    [Alias("fe")]
    [switch]$FrontendOnly,

    [Alias("be")]
    [switch]$BackendOnly,

    [Alias("r")]
    [switch]$Restart,

    [Alias("x")]
    [switch]$Cross,

    [switch]$Clean
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

function Write-Info($msg) { Write-Host "[INFO] $msg" -ForegroundColor Cyan }
function Write-Success($msg) { Write-Host "[SUCCESS] $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "[WARN] $msg" -ForegroundColor Yellow }
function Write-ErrorMsg($msg) { Write-Host "[ERROR] $msg" -ForegroundColor Red }

# 1. 清理
if ($Clean) {
    Write-Info "正在清理构建缓存..."
    Remove-Item -Path ".\static\assets\*" -Force -Recurse -ErrorAction SilentlyContinue
    Remove-Item -Path ".\noder.exe" -Force -ErrorAction SilentlyContinue
    Write-Success "清理完成"
    if (-not ($FrontendOnly -or $BackendOnly)) { return }
}

# 2. 前端构建 (如果不单独编译后端)
if (-not $BackendOnly) {
    Write-Info "===> [1/2] 开始构建前端 (web/)..."
    Push-Location "$ScriptDir\web"
    try {
        pnpm build
        if ($LASTEXITCODE -ne 0) {
            throw "前端构建失败，退出码: $LASTEXITCODE"
        }
        Write-Success "前端构建完成 -> static/"
    }
    finally {
        Pop-Location
    }
}

# 3. 交叉编译发布包
if ($Cross) {
    Write-Info "===> 开始交叉编译多平台发布包..."
    if (-not (Test-Path "dist")) { New-Item -ItemType Directory -Path "dist" | Out-Null }

    $targets = @(
        "linux:amd64:dist\noder-linux-amd64",
        "linux:arm64:dist\noder-linux-arm64",
        "windows:amd64:dist\noder-windows-amd64.exe"
    )

    foreach ($item in $targets) {
        $parts = $item.Split(':')
        $targetOS = $parts[0]
        $targetArch = $parts[1]
        $targetOut = $parts[2]
        Write-Info "正在编译: $targetOS/$targetArch -> $targetOut ..."
        $env:GOOS = $targetOS
        $env:GOARCH = $targetArch
        $env:CGO_ENABLED = "0"
        go build -ldflags "-s -w" -o $targetOut ./cmd/server
        if ($LASTEXITCODE -ne 0) {
            Write-ErrorMsg "交叉编译 $targetOS/$targetArch 失败！"
            exit $LASTEXITCODE
        }
    }
    # 恢复当前环境变量
    $env:GOOS = ""
    $env:GOARCH = ""
    $env:CGO_ENABLED = ""
    Write-Success "交叉编译完成！产物位于 .\dist\"
    Get-ChildItem .\dist | Select-Object Name, Length, LastWriteTime
    exit 0
}

# 4. 本地后端编译：检查旧进程占用
if (-not $FrontendOnly) {
    Write-Info "===> [2/2] 开始编译 Go 后端..."
    
    $runningProc = Get-Process -Name "noder" -ErrorAction SilentlyContinue
    if ($runningProc) {
        Write-Warn "检测到旧版 noder.exe (PID: $($runningProc.Id)) 正在运行，解除文件占用锁定..."
        $runningProc | Stop-Process -Force
        Start-Sleep -Milliseconds 600
    }

    # 编译 Go 可执行文件并剥离符号表以缩减体积
    go build -ldflags "-s -w" -o noder.exe ./cmd/server
    if ($LASTEXITCODE -ne 0) {
        Write-ErrorMsg "Go 后端编译失败！"
        exit $LASTEXITCODE
    }
    Write-Success "Go 二进制编译成功: .\noder.exe"

    # 4. 重启服务
    if ($Restart) {
        Write-Info "正在拉起新版本 noder.exe 服务..."
        Start-Process -FilePath "$ScriptDir\noder.exe" -WorkingDirectory $ScriptDir
        Start-Sleep -Seconds 1

        # 检查服务端口
        $portCheck = Get-NetTCPConnection -LocalPort 8000 -ErrorAction SilentlyContinue
        if ($portCheck) {
            $runningPid = $portCheck[0].OwningProcess
            Write-Host ""
            Write-Host "====================================================" -ForegroundColor Green
            Write-Host " 🎉 Noder 服务编译并重启成功！" -ForegroundColor Green
            Write-Host " 🌐 访问地址 : http://localhost:8000" -ForegroundColor Cyan
            Write-Host " ⚡ 运行状态 : 正常监听 8000 端口 (PID: $runningPid)" -ForegroundColor Green
            Write-Host "====================================================" -ForegroundColor Green
        } else {
            Write-Warn "服务已拉起，端口 8000 正在初始化，请访问 http://localhost:8000 查看。"
        }
    } else {
        Write-Info "提示: 如需自动拉起新版本服务，可添加 -Restart 参数: .\build.ps1 -Restart"
    }
}

