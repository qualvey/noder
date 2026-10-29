# Download the user's Xray configuration and start Xray on Windows.
Param(
    [string]$YamlPath = "config.yaml"
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

function Parse-SimpleYaml {
    param ([string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        throw "Configuration file not found: $Path"
    }

    $config = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        $trimmed = $line.Trim()
        if (-not $trimmed -or $trimmed.StartsWith("#")) {
            continue
        }

        if ($trimmed -match '^([a-zA-Z0-9_-]+)\s*:\s*["'']?(.*?)["'']?\s*$') {
            $config[$matches[1]] = $matches[2].Trim()
        }
    }
    return $config
}

function Resolve-LauncherPath {
    param ([string]$Path)

    if ([System.IO.Path]::IsPathRooted($Path)) {
        return $Path
    }
    return Join-Path $ScriptDir $Path
}

Write-Host "[*] Reading configuration from $YamlPath..." -ForegroundColor Cyan
$config = Parse-SimpleYaml -Path (Resolve-LauncherPath $YamlPath)

$upstreamUrl = [string]$config["upstream_url"]
$token = [string]$config["token"]
$configFile = if ($config["config_file"]) { [string]$config["config_file"] } else { "config.json" }
$xrayBin = if ($config["xray_bin"]) { [string]$config["xray_bin"] } else { "./xray.exe" }
$configPath = Resolve-LauncherPath $configFile

# Resolve the executable before stopping anything, so a bad path does not
# terminate an unrelated Xray process.
$xrayPath = Resolve-LauncherPath $xrayBin
if (-not (Test-Path -LiteralPath $xrayPath)) {
    $command = Get-Command $xrayBin -ErrorAction SilentlyContinue
    if ($command) {
        $xrayPath = $command.Source
    }
}
if (-not (Test-Path -LiteralPath $xrayPath)) {
    throw "Xray executable not found: $xrayBin. Put xray.exe next to this script or set xray_bin in $YamlPath."
}

# Stop only the configured Xray executable before replacing its config.
$procName = [System.IO.Path]::GetFileNameWithoutExtension($xrayPath)
$existingProcesses = Get-Process -Name $procName -ErrorAction SilentlyContinue
if ($existingProcesses) {
    Write-Host "[*] Stopping existing '$procName' process(es)..." -ForegroundColor Yellow
    $existingProcesses | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 500
}

$downloadSuccess = $false
if ($upstreamUrl -and $token) {
    $separator = if ($upstreamUrl.Contains("?")) { "&" } else { "?" }
    $encodedToken = [System.Uri]::EscapeDataString($token)
    $fullUrl = "${upstreamUrl}${separator}token=${encodedToken}"
    $tempConfigPath = "${configPath}.download"

    Write-Host "[*] Downloading Xray configuration from upstream..." -ForegroundColor Cyan
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri $fullUrl -OutFile $tempConfigPath -Headers @{ "User-Agent" = "xray-launcher" } -UseBasicParsing

        if (-not (Test-Path -LiteralPath $tempConfigPath) -or (Get-Item -LiteralPath $tempConfigPath).Length -eq 0) {
            throw "The upstream returned an empty configuration."
        }
        Move-Item -LiteralPath $tempConfigPath -Destination $configPath -Force
        Write-Host "[+] Xray configuration updated: $configFile" -ForegroundColor Green
        $downloadSuccess = $true
    } catch {
        Remove-Item -LiteralPath $tempConfigPath -Force -ErrorAction SilentlyContinue
        Write-Warning "Failed to download Xray configuration: $_"
    }
} else {
    Write-Warning "Missing 'upstream_url' or 'token' in $YamlPath."
}

if (-not $downloadSuccess) {
    if (Test-Path -LiteralPath $configPath) {
        Write-Host "[!] Using existing local configuration: $configFile" -ForegroundColor Yellow
    } else {
        throw "Failed to get Xray configuration: remote download failed and local '$configFile' does not exist."
    }
}

Write-Host "[*] Starting Xray..." -ForegroundColor Cyan
Write-Host "    Command: $xrayBin run -c $configFile" -ForegroundColor Yellow
try {
    & $xrayPath run -c $configPath
    $exitCode = $LASTEXITCODE
} catch {
    Write-Error "Xray failed to start: $($_.Exception.Message)"
    $exitCode = 1
}

if ($exitCode -ne 0) {
    Write-Host "[-] Xray exited with code $exitCode. Keeping this window open for error details..." -ForegroundColor Red
    Read-Host "Press Enter to exit" | Out-Null
    exit $exitCode
}
