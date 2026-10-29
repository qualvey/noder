@echo off
cd /d "%~dp0"

echo Starting Xray launcher...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0launch.ps1"
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo Script ended with error code %ERRORLEVEL%.
    pause
)
