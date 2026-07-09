@echo off
setlocal EnableExtensions

set "QUIET=0"
set "INSTALL_CLIP=0"

:parse_args
if "%~1"=="" goto after_args
if /I "%~1"=="/quiet" set "QUIET=1"
if /I "%~1"=="/clip" set "INSTALL_CLIP=1"
if /I "%~1"=="/no-clip" set "INSTALL_CLIP=0"
shift
goto parse_args

:after_args
set "SOURCE_EXE=%~dp0re_code.exe"
if not exist "%SOURCE_EXE%" set "SOURCE_EXE=%~dp0..\..\dist\re_code.exe"

if not exist "%SOURCE_EXE%" (
  echo re_code.exe was not found.
  echo Put install_crs.bat next to re_code.exe, or run it from this repo after building dist\re_code.exe.
  goto :done
)

set "INSTALL_DIR=%LOCALAPPDATA%\re.code"
set "ALIAS_DIR=%LOCALAPPDATA%\Microsoft\WindowsApps"
set "INSTALL_EXE=%INSTALL_DIR%\re_code.exe"
set "ALIAS_CMD=%ALIAS_DIR%\crs.cmd"

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if errorlevel 1 (
  echo Failed to create "%INSTALL_DIR%".
  goto :done
)

if not exist "%ALIAS_DIR%" mkdir "%ALIAS_DIR%"
if errorlevel 1 (
  echo Failed to create "%ALIAS_DIR%".
  goto :done
)

copy /Y "%SOURCE_EXE%" "%INSTALL_EXE%" >nul
if errorlevel 1 (
  echo Failed to copy re_code.exe to "%INSTALL_EXE%".
  goto :done
)

> "%ALIAS_CMD%" echo @echo off
>>"%ALIAS_CMD%" echo "%INSTALL_EXE%" %%*
if errorlevel 1 (
  echo Failed to create "%ALIAS_CMD%".
  goto :done
)

powershell -NoProfile -ExecutionPolicy Bypass -Command "$dir = [Environment]::ExpandEnvironmentVariables('%ALIAS_DIR%'); $userPath = [Environment]::GetEnvironmentVariable('Path','User'); if ($null -eq $userPath) { $userPath = '' }; $expanded = $userPath.Split(';', [System.StringSplitOptions]::RemoveEmptyEntries) ^| ForEach-Object { [Environment]::ExpandEnvironmentVariables($_).TrimEnd('\') }; if ($expanded -notcontains $dir.TrimEnd('\')) { $newPath = ($userPath.TrimEnd(';') + ';' + $dir).TrimStart(';'); [Environment]::SetEnvironmentVariable('Path', $newPath, 'User') }"
if errorlevel 1 (
  echo Installed crs.cmd, but could not update the user PATH automatically.
  echo Add "%ALIAS_DIR%" to your user PATH manually if crs is not found.
  goto :done
)

echo Installed re.code to "%INSTALL_EXE%".
echo Installed global command: crs
echo Open a new CMD or PowerShell window, then run: crs

call :maybe_install_clip
goto :done

:maybe_install_clip
set "CLIP_SOURCE_DIR=%~dp0clip"
if not exist "%CLIP_SOURCE_DIR%\install.ps1" set "CLIP_SOURCE_DIR=%~dp0..\..\clip"

if "%INSTALL_CLIP%"=="0" goto :eof

if not exist "%CLIP_SOURCE_DIR%\install.ps1" (
  echo Clipboard bridge installer was not found.
  echo Put the clip folder next to install_crs.bat, or run this from the repo.
  goto :eof
)

set "CLIP_VPS_HOST=157.173.127.84"
set "CLIP_VPS_USER=root"
set "CLIP_SSH_PORT=22"

if "%QUIET%"=="0" (
  set "CLIP_INPUT="
  set /P "CLIP_INPUT=VPS host for clipboard bridge [%CLIP_VPS_HOST%]: "
  if not "%CLIP_INPUT%"=="" set "CLIP_VPS_HOST=%CLIP_INPUT%"
  set "CLIP_INPUT="
  set /P "CLIP_INPUT=VPS user for clipboard bridge [%CLIP_VPS_USER%]: "
  if not "%CLIP_INPUT%"=="" set "CLIP_VPS_USER=%CLIP_INPUT%"
  set "CLIP_INPUT="
  set /P "CLIP_INPUT=SSH port for clipboard bridge [%CLIP_SSH_PORT%]: "
  if not "%CLIP_INPUT%"=="" set "CLIP_SSH_PORT=%CLIP_INPUT%"
)

echo Installing clipboard bridge for %CLIP_VPS_USER%@%CLIP_VPS_HOST%:%CLIP_SSH_PORT% ...
powershell -NoProfile -ExecutionPolicy Bypass -File "%CLIP_SOURCE_DIR%\install.ps1" -VpsHost "%CLIP_VPS_HOST%" -VpsUser "%CLIP_VPS_USER%" -SshPort %CLIP_SSH_PORT%
if errorlevel 1 (
  echo Clipboard bridge installation failed. You can rerun this later with: install_crs.bat /clip
  goto :eof
)

echo Clipboard bridge installed.
goto :eof

:done
if not "%QUIET%"=="1" pause
