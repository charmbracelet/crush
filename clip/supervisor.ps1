[CmdletBinding()]
param(
    [string]$ConfigPath = (Join-Path $PSScriptRoot "config.json")
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$config = Get-Content -Raw -LiteralPath $ConfigPath | ConvertFrom-Json
$logPath = Join-Path $PSScriptRoot "supervisor.log"
$pidPath = Join-Path $PSScriptRoot "supervisor.pid"
$bridgePath = Join-Path $PSScriptRoot "bridge.ps1"
$bridgePidPath = Join-Path $PSScriptRoot "bridge.pid"
$powerShellExe = (Get-Command powershell.exe -ErrorAction Stop).Source

function Write-SupervisorLog {
    param([string]$Message)
    Add-Content -LiteralPath $logPath -Value "$(Get-Date -Format o) $Message"
}

function Test-RecordedProcess {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) {
        return $false
    }
    $recordedPid = 0
    if (-not [int]::TryParse((Get-Content -Raw -LiteralPath $Path).Trim(), [ref]$recordedPid)) {
        return $false
    }
    return $null -ne (Get-Process -Id $recordedPid -ErrorAction SilentlyContinue)
}

if (Test-RecordedProcess -Path $pidPath) {
    exit 0
}
[IO.File]::WriteAllText($pidPath, [string]$PID)

$bridgeProcess = $null
try {
    while ($true) {
        if ($null -eq $bridgeProcess -or $bridgeProcess.HasExited) {
            $bridgeArgs = "-NoProfile -ExecutionPolicy Bypass -STA -File `"$bridgePath`" -ConfigPath `"$ConfigPath`""
            $bridgeProcess = Start-Process -FilePath $powerShellExe -ArgumentList $bridgeArgs -WindowStyle Hidden -PassThru
            Write-SupervisorLog "started clipboard process $($bridgeProcess.Id)"
            Start-Sleep -Seconds 1
        }

        $forward = "127.0.0.1:$($config.remote_port):127.0.0.1:$($config.local_port)"
        $sshArgs = @(
            "-N", "-T",
            "-p", [string]$config.ssh_port,
            "-i", [string]$config.key_path,
            "-o", "BatchMode=yes",
            "-o", "ExitOnForwardFailure=yes",
            "-o", "ServerAliveInterval=20",
            "-o", "ServerAliveCountMax=3",
            "-o", "StrictHostKeyChecking=accept-new",
            "-R", $forward,
            "$($config.vps_user)@$($config.vps_host)"
        )

        Write-SupervisorLog "starting reverse tunnel to $($config.vps_host), remote loopback port $($config.remote_port)"
        try {
            & ssh @sshArgs 2>&1 | ForEach-Object { Write-SupervisorLog "ssh: $_" }
        }
        catch {
            Write-SupervisorLog "ssh failed: $($_.Exception.Message)"
        }
        Write-SupervisorLog "tunnel stopped; retrying in 5 seconds"
        Start-Sleep -Seconds 5
    }
}
finally {
    if ($null -ne $bridgeProcess -and -not $bridgeProcess.HasExited) {
        Stop-Process -Id $bridgeProcess.Id -Force -ErrorAction SilentlyContinue
    }
    if (Test-Path -LiteralPath $bridgePidPath) {
        Remove-Item -LiteralPath $bridgePidPath -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $pidPath -Force -ErrorAction SilentlyContinue
    Write-SupervisorLog "supervisor stopped"
}
