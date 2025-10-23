@echo off
REM Direct launcher that bypasses all the complexity
echo Starting Crush Directly...

REM Change to project root
cd /d "%~dp0.."

REM Find crush.exe and launch it directly
if exist "crush.exe" (
    echo Found crush.exe in: %CD%
    echo Starting Crush...
    echo.
    
    REM Launch crush.exe directly
    crush.exe
    
    if %ERRORLEVEL% NEQ 0 (
        echo.
        echo Crush exited with error code %ERRORLEVEL%
        echo Press any key to continue...
        pause >nul
    )
) else (
    echo ERROR: crush.exe not found in %CD%
    echo Please build it first with: go build .
    echo.
    echo Current directory contents:
    dir /b *.exe
    echo.
    pause
)
