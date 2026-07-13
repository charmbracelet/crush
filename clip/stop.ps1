[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$installDir = Join-Path $env:LOCALAPPDATA "CrushClipBridge"
foreach ($name in @("supervisor", "bridge")) {
    $pidPath = Join-Path $installDir "$name.pid"
    if (-not (Test-Path -LiteralPath $pidPath)) {
        continue
    }
    $recordedPid = 0
    if ([int]::TryParse((Get-Content -Raw -LiteralPath $pidPath).Trim(), [ref]$recordedPid)) {
        Stop-Process -Id $recordedPid -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $pidPath -Force -ErrorAction SilentlyContinue
}
Write-Host "Stopped Crush Clipboard Bridge."
