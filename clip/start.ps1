[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$installDir = Join-Path $env:LOCALAPPDATA "CrushClipBridge"
$supervisor = Join-Path $installDir "supervisor.ps1"
$config = Join-Path $installDir "config.json"
$pidPath = Join-Path $installDir "supervisor.pid"

if (-not (Test-Path -LiteralPath $supervisor) -or -not (Test-Path -LiteralPath $config)) {
    throw "Crush Clipboard Bridge is not installed. Run install.ps1 first."
}

if (Test-Path -LiteralPath $pidPath) {
    $recordedPid = 0
    if ([int]::TryParse((Get-Content -Raw -LiteralPath $pidPath).Trim(), [ref]$recordedPid) -and
        $null -ne (Get-Process -Id $recordedPid -ErrorAction SilentlyContinue)) {
        Write-Host "Crush Clipboard Bridge is already running (PID $recordedPid)."
        exit 0
    }
}

$powerShellExe = (Get-Command powershell.exe -ErrorAction Stop).Source
$arguments = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$supervisor`" -ConfigPath `"$config`""
$process = Start-Process -FilePath $powerShellExe -ArgumentList $arguments -WindowStyle Hidden -PassThru
Write-Host "Started Crush Clipboard Bridge (PID $($process.Id))."
