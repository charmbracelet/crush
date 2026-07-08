[CmdletBinding()]
param(
    [string]$VpsHost = "157.173.127.84",
    [string]$VpsUser = "root",
    [ValidateRange(1, 65535)][int]$SshPort = 22,
    [ValidateRange(1024, 65535)][int]$LocalPort = 47831,
    [ValidateRange(1024, 65535)][int]$RemotePort = 48731,
    [switch]$NoAutoStart
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

foreach ($command in @("ssh", "ssh-keygen")) {
    if ($null -eq (Get-Command $command -ErrorAction SilentlyContinue)) {
        throw "$command is required. Install the Windows OpenSSH Client optional feature first."
    }
}

$installDir = Join-Path $env:LOCALAPPDATA "CrushClipBridge"
$sshDir = Join-Path $HOME ".ssh"
$keyPath = Join-Path $sshDir "crush_clip_bridge_ed25519"
$tokenPath = Join-Path $installDir "bridge.token"
$configPath = Join-Path $installDir "config.json"
$target = "$VpsUser@$VpsHost"

New-Item -ItemType Directory -Path $installDir, $sshDir -Force | Out-Null
foreach ($file in @("bridge.ps1", "supervisor.ps1", "start.ps1", "stop.ps1", "uninstall.ps1", "README.md")) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination (Join-Path $installDir $file) -Force
}

if (-not (Test-Path -LiteralPath $keyPath)) {
    Write-Host "Creating a dedicated SSH key: $keyPath"
    $keyComment = "crush-clip-$env:COMPUTERNAME"
    $keygenCommand = 'ssh-keygen -q -t ed25519 -f "{0}" -N "" -C "{1}"' -f $keyPath, $keyComment
    & cmd.exe /d /c $keygenCommand
    if ($LASTEXITCODE -ne 0) {
        throw "ssh-keygen failed."
    }
}

$testArgs = @(
    "-p", [string]$SshPort,
    "-i", $keyPath,
    "-o", "BatchMode=yes",
    "-o", "StrictHostKeyChecking=accept-new",
    $target,
    "true"
)
& ssh @testArgs *> $null
if ($LASTEXITCODE -ne 0) {
    Write-Host "The dedicated key is not installed on the VPS."
    Write-Host "Enter the VPS password once if SSH asks for it. OpenSSH will hide the input."
    $publicKey = (Get-Content -Raw -LiteralPath "$keyPath.pub").Trim()
    $publicKey64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($publicKey))
    $remoteInstall = "set -eu; umask 077; mkdir -p ~/.ssh; chmod 700 ~/.ssh; tmp=~/.ssh/crush_clip_bridge.pub.tmp; printf '%s' '$publicKey64' | base64 -d > `$tmp; touch ~/.ssh/authorized_keys; grep -qxFf `$tmp ~/.ssh/authorized_keys || cat `$tmp >> ~/.ssh/authorized_keys; rm -f `$tmp; chmod 600 ~/.ssh/authorized_keys"
    & ssh -p $SshPort -o StrictHostKeyChecking=accept-new $target $remoteInstall
    if ($LASTEXITCODE -ne 0) {
        throw "Could not install the SSH key. Confirm password login is enabled, then rerun install.ps1."
    }

    & ssh @testArgs
    if ($LASTEXITCODE -ne 0) {
        throw "The dedicated SSH key was installed but key-only login still failed."
    }
}

$readTokenCommand = 'test -s "$HOME/.config/crush/clip-bridge.token" && cat "$HOME/.config/crush/clip-bridge.token" || true'
$remoteToken = (& ssh -p $SshPort -i $keyPath -o BatchMode=yes -o StrictHostKeyChecking=accept-new $target $readTokenCommand | Out-String).Trim()
if ([string]::IsNullOrWhiteSpace($remoteToken)) {
    $random = [Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $bytes = New-Object byte[] 32
        $random.GetBytes($bytes)
        $remoteToken = [Convert]::ToBase64String($bytes).TrimEnd("=").Replace("+", "-").Replace("/", "_")
    }
    finally {
        $random.Dispose()
    }
    $writeTokenCommand = 'umask 077; mkdir -p "$HOME/.config/crush"; tr -d "\r\n" > "$HOME/.config/crush/clip-bridge.token"'
    $remoteToken | & ssh -p $SshPort -i $keyPath -o BatchMode=yes $target $writeTokenCommand
    if ($LASTEXITCODE -ne 0) {
        throw "Could not install the bridge token on the VPS."
    }
}
[IO.File]::WriteAllText($tokenPath, $remoteToken)

$remoteConfig = @{ url = "http://127.0.0.1:$RemotePort"; token_file = '~/.config/crush/clip-bridge.token' } | ConvertTo-Json -Compress
$writeConfigCommand = 'umask 077; mkdir -p "$HOME/.config/crush"; cat > "$HOME/.config/crush/clip-bridge.json"'
$remoteConfig | & ssh -p $SshPort -i $keyPath -o BatchMode=yes $target $writeConfigCommand
if ($LASTEXITCODE -ne 0) {
    throw "Could not install the bridge config on the VPS."
}

$config = [ordered]@{
    vps_host   = $VpsHost
    vps_user   = $VpsUser
    ssh_port   = $SshPort
    local_port = $LocalPort
    remote_port = $RemotePort
    key_path   = $keyPath
    token_path = $tokenPath
}
$config | ConvertTo-Json | Set-Content -LiteralPath $configPath -Encoding UTF8

$startupDir = [Environment]::GetFolderPath("Startup")
$shortcutPath = Join-Path $startupDir "Crush Clipboard Bridge.lnk"
$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = (Get-Command powershell.exe -ErrorAction Stop).Source
$shortcut.Arguments = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$(Join-Path $installDir 'supervisor.ps1')`" -ConfigPath `"$configPath`""
$shortcut.WorkingDirectory = $installDir
$shortcut.WindowStyle = 7
$shortcut.Description = "Crush clipboard bridge and private SSH reverse tunnel"
$shortcut.Save()

if (-not $NoAutoStart) {
    & (Join-Path $installDir "start.ps1")
}

Write-Host "Installed Crush Clipboard Bridge in $installDir"
Write-Host "The VPS port is loopback-only: 127.0.0.1:$RemotePort"
Write-Host "It will start automatically when this Windows user signs in."
