# test_reasoning_layer.ps1
# Test script for the reasoning layer module

Write-Host ""
Write-Host "=== Testing Crush Reasoning Layer ===" -ForegroundColor Cyan
Write-Host ""

# Test 1: Import the module
Write-Host "[1/5] Importing reasoning layer module..." -ForegroundColor Yellow
try {
    Import-Module .\plan\crush_scripts\reasoning_layer.ps1 -Force -ErrorAction Stop
    Write-Host "  [OK] Module imported successfully" -ForegroundColor Green
} catch {
    Write-Host "  [FAIL] Failed to import module" -ForegroundColor Red
    Write-Host "  Error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test 2: Add a task
Write-Host ""
Write-Host "[2/5] Testing Add-Task..." -ForegroundColor Yellow
try {
    $task = Add-Task -Description "Test task: Implement JWT authentication" -Priority high -Tags @("auth", "backend")
    Write-Host "  [OK] Task added with ID: $($task.id)" -ForegroundColor Green
} catch {
    Write-Host "  [FAIL] Failed to add task" -ForegroundColor Red
    Write-Host "  Error: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 3: List tasks
Write-Host ""
Write-Host "[3/5] Testing Get-TaskList..." -ForegroundColor Yellow
try {
    $tasks = Get-TaskList
    Write-Host "  [OK] Task list retrieved" -ForegroundColor Green
} catch {
    Write-Host "  [FAIL] Failed to list tasks" -ForegroundColor Red
    Write-Host "  Error: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 4: Test AI Task Reasoning
Write-Host ""
Write-Host "[4/5] Testing AI Task Reasoning..." -ForegroundColor Yellow
Write-Host "  This will query LM Studio to break down the task..." -ForegroundColor Gray
try {
    # Use a simple task for testing
    Invoke-TaskReason -TaskId 1
    Write-Host "  [OK] AI task reasoning completed" -ForegroundColor Green
} catch {
    Write-Host "  [FAIL] AI task reasoning failed" -ForegroundColor Red
    Write-Host "  Error: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 5: Test Code Generation
Write-Host ""
Write-Host "[5/5] Testing AI Code Generation..." -ForegroundColor Yellow
Write-Host "  Generating a simple Python function..." -ForegroundColor Gray
try {
    Invoke-CodeGeneration -Description "Create a Python function to validate email addresses using regex" -Language "Python"
    Write-Host "  [OK] Code generation completed" -ForegroundColor Green
} catch {
    Write-Host "  [FAIL] Code generation failed" -ForegroundColor Red
    Write-Host "  Error: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
Write-Host "=== Testing Complete ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Cleanup: Removing test task file..." -ForegroundColor Gray
if (Test-Path ".crush_tasks.json") {
    Remove-Item ".crush_tasks.json" -Force
    Write-Host "  [OK] Test data cleaned up" -ForegroundColor Green
}
Write-Host ""
