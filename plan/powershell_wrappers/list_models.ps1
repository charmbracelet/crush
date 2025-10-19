# list_models.ps1
# PowerShell wrapper to list available models from LM Studio
# Usage: .\list_models.ps1 [-BaseUrl "http://localhost:1234/v1"]

param(
    [Parameter(Mandatory=$false)]
    [string]$BaseUrl = "http://localhost:1234/v1"
)

try {
    Write-Host "Querying LM Studio at $BaseUrl for available models..." -ForegroundColor Cyan
    Write-Host ""

    # Query the models endpoint
    $response = Invoke-RestMethod -Uri "$BaseUrl/models" `
        -Method Get `
        -ContentType "application/json" `
        -ErrorAction Stop

    # Display the models
    if ($response.data -and $response.data.Count -gt 0) {
        Write-Host "=== Available Models ===" -ForegroundColor Green
        Write-Host ""

        foreach ($model in $response.data) {
            Write-Host "Model ID: " -NoNewline -ForegroundColor Yellow
            Write-Host $model.id
            Write-Host "  Object: $($model.object)"
            if ($model.owned_by) {
                Write-Host "  Owned By: $($model.owned_by)"
            }
            Write-Host ""
        }

        Write-Host "Total models: $($response.data.Count)" -ForegroundColor Cyan
    } else {
        Write-Host "No models found. Make sure LM Studio is running and a model is loaded." -ForegroundColor Yellow
    }

    return $response

} catch {
    Write-Host "Error communicating with LM Studio:" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red

    Write-Host ""
    Write-Host "Troubleshooting:" -ForegroundColor Yellow
    Write-Host "1. Make sure LM Studio is running"
    Write-Host "2. Check that the local server is started in LM Studio"
    Write-Host "3. Verify the server is running on port 1234 (or adjust -BaseUrl)"
    Write-Host "4. Ensure a model is loaded in the server tab"

    exit 1
}
