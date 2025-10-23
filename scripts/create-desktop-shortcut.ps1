param(
    [string]$BinaryPath,
    [switch]$UseLauncher
)

# PowerShell script to create/update Crush desktop shortcut
Write-Host "Creating/Updating Crush Desktop Shortcut..." -ForegroundColor Green

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$launcherPath = Join-Path $scriptDir "create-crush-launcher.bat"

# Try to find the Crush binary if not provided
if (-not $BinaryPath) {
    # First, try the go install location
    try {
        $gopath = & go env GOPATH
        $goBinary = Join-Path $gopath "bin\crush.exe"
        if (Test-Path $goBinary) {
            $BinaryPath = $goBinary
        }
    } catch {
        Write-Host "Go not found, continuing..." -ForegroundColor Yellow
    }

    # Second, try the current directory
    if (-not $BinaryPath) {
        $currentDirBinary = Join-Path (Get-Location) "crush.exe"
        if (Test-Path $currentDirBinary) {
            $BinaryPath = $currentDirBinary
        }
    }

    # Third, try PATH
    if (-not $BinaryPath) {
        try {
            $pathBinary = Get-Command crush.exe -ErrorAction Stop
            $BinaryPath = $pathBinary.Source
        } catch {
            # Continue without error
        }
    }
}

# Check if launcher exists or if we should use it
if ($UseLauncher) {
    Write-Host "Using launcher script for better environment setup" -ForegroundColor Cyan
    $targetPath = $launcherPath
    $workingDir = Join-Path $scriptDir ".."  # Parent directory (project root)
    $description = "Crush Enhanced - Terminal-based AI assistant (with auto-setup)"
    $shortcutName = "Crush Enhanced.lnk"
} elseif (Test-Path $launcherPath) {
    Write-Host "Using simple launcher for direct execution" -ForegroundColor Cyan
    $simpleLauncherPath = Join-Path $scriptDir "simple-launch.bat"
    $targetPath = $simpleLauncherPath
    $workingDir = Join-Path $scriptDir ".."  # Parent directory (project root)
    $description = "Crush Enhanced - Terminal-based AI assistant (direct)"
    $shortcutName = "Crush Enhanced.lnk"
} elseif (-not $BinaryPath -or -not (Test-Path $BinaryPath)) {
    Write-Host "Error: Crush binary not found." -ForegroundColor Red
    Write-Host "Please specify path with -BinaryPath or ensure one of these exists:" -ForegroundColor Yellow
    Write-Host "  - '$env:GOPATH\bin\crush.exe'"
    Write-Host "  - '.\crush.exe'"
    Write-Host "  - 'crush.exe' in PATH"
    Write-Host "  - Launcher script: '$launcherPath'"
    exit 1
} else {
    Write-Host "Using Crush binary: $BinaryPath" -ForegroundColor Cyan
    $targetPath = $BinaryPath
    $workingDir = $env:USERPROFILE
    $description = "Crush - Terminal-based AI assistant for software development"
    $shortcutName = "Crush.lnk"
}

# Get desktop folder and shortcut path
$desktopFolder = [Environment]::GetFolderPath("Desktop")
$shortcutPath = Join-Path $desktopFolder $shortcutName

# Create or update the shortcut
$WshShell = New-Object -comObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut($shortcutPath)
$Shortcut.TargetPath = $targetPath
$Shortcut.WorkingDirectory = $workingDir
$Shortcut.WindowStyle = 1

# Set icon - try to use the binary's icon, fallback to default
if ($targetPath.EndsWith(".exe")) {
    $Shortcut.IconLocation = "$targetPath, 0"
} else {
    # For batch files, try to find a suitable icon
    if ($BinaryPath -and (Test-Path $BinaryPath)) {
        $Shortcut.IconLocation = "$BinaryPath, 0"
    } else {
        # Default Windows terminal icon
        $Shortcut.IconLocation = "shell32.dll, 162"
    }
}

$Shortcut.Description = $description
$Shortcut.Save()

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Desktop shortcut created/updated successfully!" -ForegroundColor Green
    Write-Host "Location: $shortcutPath" -ForegroundColor Cyan
    Write-Host "Target: $targetPath" -ForegroundColor Cyan
    Write-Host "Working Directory: $workingDir" -ForegroundColor Cyan
    
    if ($targetPath.EndsWith(".bat")) {
        Write-Host "💡 This shortcut uses the launcher script for better environment setup" -ForegroundColor Yellow
    }
} else {
    Write-Host "❌ Failed to create desktop shortcut" -ForegroundColor Red
}
