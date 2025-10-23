@echo off
echo Testing launcher script...
echo Current directory: %CD%
echo.

REM Test if launcher script exists
if not exist "%~dp0create-crush-launcher.bat" (
    echo ERROR: Launcher script not found at: %~dp0create-crush-launcher.bat
    pause
    exit /b 1
)

echo Found launcher script: %~dp0create-crush-launcher.bat
echo.

REM Try to run the launcher script with timeout
timeout /t 5 /nobreak >nul
echo Running launcher script...
call "%~dp0create-crush-launcher.bat"

echo.
echo Launcher script completed with exit code: %ERRORLEVEL%
pause
