@echo off
REM Windows batch script to create/update Crush desktop shortcut
REM This ensures the shortcut works with the latest fixes

echo Creating/Updating Crush Desktop Shortcut...

REM Try to find the Crush binary in common locations
set CRUSH_BINARY=

REM First, try the go install location
for /f "tokens=*" %%i in ('go env GOPATH') do set GOPATH=%%i
if exist "%GOPATH%\bin\crush.exe" (
    set CRUSH_BINARY=%GOPATH%\bin\crush.exe
)

REM Second, try the current directory build
if "%CRUSH_BINARY%"=="" if exist "crush.exe" (
    set CRUSH_BINARY=%CD%\crush.exe
)

REM Third, try the go bin directory in PATH
if "%CRUSH_BINARY%"=="" (
    where crush.exe >nul 2>nul
    if !errorlevel! equ 0 (
        for /f "tokens=*" %%i in ('where crush.exe') do set CRUSH_BINARY=%%i
    )
)

REM Check if the binary exists
if "%CRUSH_BINARY%"=="" (
    echo Error: Crush binary not found. Please run one of the following first:
    echo   - 'go install .' to install system-wide
    echo   - 'go build .' to build in current directory
    echo   - Make sure Crush is installed and in your PATH
    pause
    exit /b 1
)

REM Get the desktop folder
set DESKTOP_FOLDER=%USERPROFILE%\Desktop
set SHORTCUT_NAME=Crush.lnk

REM Create the shortcut using the PowerShell script
powershell -ExecutionPolicy Bypass -File "%~dp0create-desktop-shortcut.ps1" -BinaryPath "%CRUSH_BINARY%"

if %ERRORLEVEL% EQU 0 (
    echo ✅ Desktop shortcut created/updated successfully!
    echo Location: %DESKTOP_FOLDER%\%SHORTCUT_NAME%
    echo Target: %CRUSH_BINARY%
) else (
    echo ❌ Failed to create desktop shortcut
)

echo.
echo Press any key to continue...
pause >nul
