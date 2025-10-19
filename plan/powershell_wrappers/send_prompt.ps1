# send_prompt.ps1
# PowerShell wrapper to send prompts to LM Studio local server
# Usage: .\send_prompt.ps1 -Prompt "Your prompt here" [-Model "qwen3-8b"] [-Temperature 0.7]

param(
    [Parameter(Mandatory=$true)]
    [string]$Prompt,

    [Parameter(Mandatory=$false)]
    [string]$Model = "qwen3-8b",

    [Parameter(Mandatory=$false)]
    [double]$Temperature = 0.7,

    [Parameter(Mandatory=$false)]
    [int]$MaxTokens = 2048,

    [Parameter(Mandatory=$false)]
    [string]$BaseUrl = "http://localhost:1234/v1",

    [Parameter(Mandatory=$false)]
    [switch]$Pretty
)

# Construct the request body
$body = @{
    model = $Model
    messages = @(
        @{
            role = "user"
            content = $Prompt
        }
    )
    temperature = $Temperature
    max_tokens = $MaxTokens
} | ConvertTo-Json -Depth 10

try {
    # Send the request
    Write-Host "Sending prompt to LM Studio at $BaseUrl..." -ForegroundColor Cyan
    Write-Host "Model: $Model | Temperature: $Temperature | Max Tokens: $MaxTokens" -ForegroundColor Gray
    Write-Host ""

    $response = Invoke-RestMethod -Uri "$BaseUrl/chat/completions" `
        -Method Post `
        -Body $body `
        -ContentType "application/json" `
        -ErrorAction Stop

    # Extract the response content
    $content = $response.choices[0].message.content

    if ($Pretty) {
        # Pretty print the full response
        Write-Host "=== Response ===" -ForegroundColor Green
        Write-Host $content
        Write-Host ""
        Write-Host "=== Metadata ===" -ForegroundColor Yellow
        Write-Host "Tokens Used: $($response.usage.total_tokens) (Prompt: $($response.usage.prompt_tokens), Completion: $($response.usage.completion_tokens))"
        Write-Host "Model: $($response.model)"
        Write-Host "Finish Reason: $($response.choices[0].finish_reason)"
    } else {
        # Just output the content
        Write-Output $content
    }

    return $response

} catch {
    Write-Host "Error communicating with LM Studio:" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red

    if ($_.Exception.Response) {
        $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $reader.BaseStream.Position = 0
        $reader.DiscardBufferedData()
        $responseBody = $reader.ReadToEnd()
        Write-Host "Response: $responseBody" -ForegroundColor Red
    }

    exit 1
}
