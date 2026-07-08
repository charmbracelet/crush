[CmdletBinding()]
param(
    [switch]$RemoveDedicatedKey
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$installDir = Join-Path $env:LOCALAPPDATA "CrushClipBridge"
$stopScript = Join-Path $installDir "stop.ps1"
if (Test-Path -LiteralPath $stopScript) {
    & $stopScript
}

$shortcutPath = Join-Path ([Environment]::GetFolderPath("Startup")) "Crush Clipboard Bridge.lnk"
Remove-Item -LiteralPath $shortcutPath -Force -ErrorAction SilentlyContinue

if ($RemoveDedicatedKey) {
    $keyPath = Join-Path (Join-Path $HOME ".ssh") "crush_clip_bridge_ed25519"
    Remove-Item -LiteralPath $keyPath, "$keyPath.pub" -Force -ErrorAction SilentlyContinue
    Write-Warning "The corresponding public key remains in the VPS authorized_keys file."
}

Remove-Item -LiteralPath $installDir -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "Uninstalled Crush Clipboard Bridge."
