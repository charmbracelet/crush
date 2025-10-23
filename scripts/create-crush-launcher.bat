@echo off
setlocal enabledelayedexpansion
REM Windows batch script to launch Crush with full environment
REM This ensures all fixes are applied when launching from shortcut

echo Starting Crush Enhanced...

REM Set working directory to the script location
cd /d "%~dp0"

echo Current directory: %CD%
echo Script directory: %~dp0
echo Checking for crush.exe...

REM List files in parent directory to debug
echo Files in parent directory:
dir "..\" | find ".exe"
echo.

REM Find Crush binary
set CRUSH_BINARY=

REM First, check current directory
if exist "%~dp0crush.exe" (
    set CRUSH_BINARY=%~dp0crush.exe
    echo Found Crush binary in current directory: %CRUSH_BINARY%
    goto :found

REM Second, check parent directory (project root)
if exist "%~dp0..\crush.exe" (
    set CRUSH_BINARY=%~dp0..\crush.exe
    echo Found Crush binary in parent directory: %CRUSH_BINARY%
    goto :found

REM Third, check go install location
for /f "tokens=*" %%i in ('go env GOPATH 2^>nul') do set GOPATH=%%i
if exist "%GOPATH%\bin\crush.exe" (
    set CRUSH_BINARY=%GOPATH%\bin\crush.exe
    echo Found Crush binary in Go install location: %CRUSH_BINARY%
    goto :found

REM Fourth, check PATH
for /f "tokens=*" %%i in ('where crush.exe 2^>nul') do (
    set CRUSH_BINARY=%%i
    echo Found Crush binary in PATH: %CRUSH_BINARY%
    goto :found
)

:found
if "%CRUSH_BINARY%"=="" (
    echo Error: Crush binary not found.
    echo.
    echo Please ensure crush.exe is in one of these locations:
    echo   - Current directory: %~dp0crush.exe
    echo   - Parent directory: %~dp0..\crush.exe  
    echo   - Go install location: %%GOPATH%%\bin\crush.exe
    echo   - In your system PATH
    echo.
    echo You can build it with: go build .
    pause
    exit /b 1
)

echo Launching Crush: %CRUSH_BINARY%
echo.

REM Change to parent directory for proper Crush operation
cd /d "%~dp0.."

echo Changed working directory to: %CD%
echo Starting Crush with full environment...
echo.

REM Launch Crush with project root as working directory
%CRUSH_BINARY%

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo Crush exited with error code %ERRORLEVEL%
    pause
)
