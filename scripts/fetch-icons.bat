@echo off
rem Double-click this to install the pal/item icons. It just runs the
rem PowerShell script beside it with execution policy bypassed, so nobody
rem has to change any Windows setting by hand.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0fetch-icons.ps1" %*
echo.
pause
