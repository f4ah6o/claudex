@echo off
setlocal
set "POWERSHELL=powershell.exe"
where pwsh.exe >nul 2>&1
if not errorlevel 1 set "POWERSHELL=pwsh.exe"
"%POWERSHELL%" -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0claudex.ps1" %*
exit /b %ERRORLEVEL%
