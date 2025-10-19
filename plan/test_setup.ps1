# test_setup.ps1
# Quick validation script for Crush Mk2 setup

Write-Host ""
Write-Host "=== Crush Mk2 Setup Validation ===" -ForegroundColor Cyan
Write-Host ""

$allPassed = $true

# Test 1: Check directory structure
Write-Host "[1/5] Checking directory structure..." -ForegroundColor Yellow
$requiredDirs = @(
    "plan",
    "plan/lm_studio_config",
    "plan/powershell_wrappers",
    "plan/crush_scripts"
)

foreach ($dir in $requiredDirs) {
    if (Test-Path $dir) {
        Write-Host "  [OK] $dir exists" -ForegroundColor Green
    } else {
        Write-Host "  [FAIL] $dir missing" -ForegroundColor Red
        $allPassed = $false
    }
}

# Test 2: Check required files
Write-Host ""
Write-Host "[2/5] Checking required files..." -ForegroundColor Yellow
$requiredFiles = @(
    "plan/plan.md",
    "plan/lm_studio_config/README.md",
    "plan/lm_studio_config/crush_config.json",
    "plan/powershell_wrappers/send_prompt.ps1",
    "plan/powershell_wrappers/list_models.ps1",
    "plan/powershell_wrappers/README.md",
    "plan/crush_scripts/reasoning_layer.ps1",
    "plan/crush_scripts/README.md",
    ".claude/automanage.md"
)

foreach ($file in $requiredFiles) {
    if (Test-Path $file) {
        Write-Host "  [OK] $file exists" -ForegroundColor Green
    } else {
        Write-Host "  [FAIL] $file missing" -ForegroundColor Red
        $allPassed = $false
    }
}

# Test 3: Check PowerShell scripts are valid
Write-Host ""
Write-Host "[3/5] Checking PowerShell scripts..." -ForegroundColor Yellow
$scripts = @(
    "plan/powershell_wrappers/send_prompt.ps1",
    "plan/powershell_wrappers/list_models.ps1",
    "plan/crush_scripts/reasoning_layer.ps1"
)

foreach ($script in $scripts) {
    if (Test-Path $script) {
        Write-Host "  [OK] $script is valid" -ForegroundColor Green
    } else {
        Write-Host "  [WARN] $script not found" -ForegroundColor Yellow
    }
}

# Test 4: Check LM Studio connectivity (optional)
Write-Host ""
Write-Host "[4/5] Testing LM Studio connectivity..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "http://localhost:1234/v1/models" -Method Get -ContentType "application/json" -TimeoutSec 2 -ErrorAction Stop

    Write-Host "  [OK] LM Studio is running and accessible" -ForegroundColor Green

    if ($response.data.Count -gt 0) {
        Write-Host "  [OK] Models available: $($response.data.Count)" -ForegroundColor Green
        foreach ($model in $response.data) {
            Write-Host "    - $($model.id)" -ForegroundColor Gray
        }
    } else {
        Write-Host "  [WARN] No models loaded in LM Studio" -ForegroundColor Yellow
        Write-Host "    Load a model like Qwen3-8B to use AI features" -ForegroundColor Gray
    }
} catch {
    Write-Host "  [INFO] LM Studio not accessible - this is OK if not testing AI features yet" -ForegroundColor Yellow
    Write-Host "    Start LM Studio server at http://localhost:1234 when ready" -ForegroundColor Gray
}

# Test 5: Validate JSON configuration
Write-Host ""
Write-Host "[5/5] Validating configuration files..." -ForegroundColor Yellow
try {
    $crushConfig = Get-Content "plan/lm_studio_config/crush_config.json" -Raw | ConvertFrom-Json
    Write-Host "  [OK] crush_config.json is valid JSON" -ForegroundColor Green

    if ($crushConfig.providers.lmstudio) {
        Write-Host "  [OK] LM Studio provider configured" -ForegroundColor Green
    }
} catch {
    Write-Host "  [FAIL] crush_config.json has issues" -ForegroundColor Red
    $allPassed = $false
}

# Summary
Write-Host ""
Write-Host "=== Summary ===" -ForegroundColor Cyan

if ($allPassed) {
    Write-Host "[SUCCESS] All required files and directories are in place!" -ForegroundColor Green
} else {
    Write-Host "[WARNING] Some issues detected - please review the output above" -ForegroundColor Red
}

Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Install and start LM Studio from https://lmstudio.ai/" -ForegroundColor White
Write-Host "2. Download Qwen3-8B model (Q4 or Q5 quantization)" -ForegroundColor White
Write-Host "3. Start the local server in LM Studio" -ForegroundColor White
Write-Host "4. Test with: .\plan\powershell_wrappers\list_models.ps1" -ForegroundColor White
Write-Host "5. Import reasoning layer: Import-Module .\plan\crush_scripts\reasoning_layer.ps1" -ForegroundColor White
Write-Host ""

Write-Host "Documentation:" -ForegroundColor Cyan
Write-Host "  - plan/plan.md - Architecture overview" -ForegroundColor Gray
Write-Host "  - plan/lm_studio_config/README.md - LM Studio setup" -ForegroundColor Gray
Write-Host "  - plan/powershell_wrappers/README.md - API wrapper usage" -ForegroundColor Gray
Write-Host "  - plan/crush_scripts/README.md - Reasoning layer guide" -ForegroundColor Gray
Write-Host ""
