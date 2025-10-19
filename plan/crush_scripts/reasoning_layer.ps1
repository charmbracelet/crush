# reasoning_layer.ps1
# Task management and AI reasoning layer for Crush
# Integrates with LM Studio for task planning and code generation

# Configuration
$script:TaskFile = ".crush_tasks.json"
$script:LMStudioUrl = "http://localhost:1234/v1"
$script:DefaultModel = "qwen3-8b"

# Initialize task storage
function Initialize-TaskStorage {
    if (-not (Test-Path $script:TaskFile)) {
        $initialData = @{
            tasks = @()
            nextId = 1
            created = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
        }
        $initialData | ConvertTo-Json -Depth 10 | Out-File $script:TaskFile -Encoding UTF8
    }
}

# Load tasks from storage
function Get-Tasks {
    Initialize-TaskStorage
    $content = Get-Content $script:TaskFile -Raw | ConvertFrom-Json
    return $content
}

# Save tasks to storage
function Save-Tasks {
    param($TaskData)
    $TaskData | ConvertTo-Json -Depth 10 | Out-File $script:TaskFile -Encoding UTF8
}

# Add a new task
function Add-Task {
    param(
        [Parameter(Mandatory=$true)]
        [string]$Description,

        [Parameter(Mandatory=$false)]
        [string]$Priority = "medium",

        [Parameter(Mandatory=$false)]
        [string[]]$Tags = @()
    )

    $taskData = Get-Tasks

    $newTask = @{
        id = $taskData.nextId
        description = $Description
        priority = $Priority
        status = "pending"
        tags = $Tags
        created = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
        updated = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
        subtasks = @()
    }

    $taskData.tasks += $newTask
    $taskData.nextId++

    Save-Tasks -TaskData $taskData

    Write-Host "Task added successfully!" -ForegroundColor Green
    Write-Host "ID: $($newTask.id)" -ForegroundColor Cyan
    Write-Host "Description: $Description" -ForegroundColor White
    Write-Host "Priority: $Priority" -ForegroundColor Yellow

    return $newTask
}

# List all tasks
function Get-TaskList {
    param(
        [Parameter(Mandatory=$false)]
        [string]$Status = "",

        [Parameter(Mandatory=$false)]
        [string]$Priority = "",

        [Parameter(Mandatory=$false)]
        [string]$Tag = ""
    )

    $taskData = Get-Tasks
    $tasks = $taskData.tasks

    # Apply filters
    if ($Status) {
        $tasks = $tasks | Where-Object { $_.status -eq $Status }
    }
    if ($Priority) {
        $tasks = $tasks | Where-Object { $_.priority -eq $Priority }
    }
    if ($Tag) {
        $tasks = $tasks | Where-Object { $_.tags -contains $Tag }
    }

    if ($tasks.Count -eq 0) {
        Write-Host "No tasks found." -ForegroundColor Yellow
        return
    }

    Write-Host ""
    Write-Host "=== Task List ===" -ForegroundColor Cyan
    Write-Host ""

    foreach ($task in $tasks) {
        $statusColor = switch ($task.status) {
            "pending" { "Yellow" }
            "in_progress" { "Cyan" }
            "completed" { "Green" }
            default { "White" }
        }

        $prioritySymbol = switch ($task.priority) {
            "high" { "[!]" }
            "medium" { "[~]" }
            "low" { "[.]" }
            default { "[ ]" }
        }

        Write-Host "[$($task.id)] $prioritySymbol " -NoNewline
        Write-Host $task.description -ForegroundColor $statusColor
        Write-Host "    Status: $($task.status) | Priority: $($task.priority) | Updated: $($task.updated)" -ForegroundColor Gray

        if ($task.subtasks -and $task.subtasks.Count -gt 0) {
            Write-Host "    Subtasks: $($task.subtasks.Count)" -ForegroundColor DarkGray
        }

        Write-Host ""
    }

    return $tasks
}

# Update task status
function Update-TaskStatus {
    param(
        [Parameter(Mandatory=$true)]
        [int]$TaskId,

        [Parameter(Mandatory=$true)]
        [ValidateSet("pending", "in_progress", "completed", "blocked")]
        [string]$Status
    )

    $taskData = Get-Tasks
    $task = $taskData.tasks | Where-Object { $_.id -eq $TaskId }

    if (-not $task) {
        Write-Host "Task ID $TaskId not found." -ForegroundColor Red
        return
    }

    $task.status = $Status
    $task.updated = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")

    Save-Tasks -TaskData $taskData

    Write-Host "Task $TaskId status updated to: $Status" -ForegroundColor Green
}

# Remove a task
function Remove-Task {
    param(
        [Parameter(Mandatory=$true)]
        [int]$TaskId
    )

    $taskData = Get-Tasks
    $taskData.tasks = $taskData.tasks | Where-Object { $_.id -ne $TaskId }

    Save-Tasks -TaskData $taskData

    Write-Host "Task $TaskId removed successfully." -ForegroundColor Green
}

# Call LM Studio API
function Invoke-LMStudio {
    param(
        [Parameter(Mandatory=$true)]
        [string]$Prompt,

        [Parameter(Mandatory=$false)]
        [string]$SystemMessage = "",

        [Parameter(Mandatory=$false)]
        [double]$Temperature = 0.3,

        [Parameter(Mandatory=$false)]
        [int]$MaxTokens = 2048
    )

    $messages = @()

    if ($SystemMessage) {
        $messages += @{
            role = "system"
            content = $SystemMessage
        }
    }

    $messages += @{
        role = "user"
        content = $Prompt
    }

    $body = @{
        model = $script:DefaultModel
        messages = $messages
        temperature = $Temperature
        max_tokens = $MaxTokens
    } | ConvertTo-Json -Depth 10

    try {
        $response = Invoke-RestMethod -Uri "$script:LMStudioUrl/chat/completions" `
            -Method Post `
            -Body $body `
            -ContentType "application/json" `
            -ErrorAction Stop

        return $response.choices[0].message.content
    } catch {
        Write-Host "Error communicating with LM Studio:" -ForegroundColor Red
        Write-Host $_.Exception.Message -ForegroundColor Red
        return $null
    }
}

# AI-powered task breakdown
function Invoke-TaskReason {
    param(
        [Parameter(Mandatory=$true)]
        [int]$TaskId
    )

    $taskData = Get-Tasks
    $task = $taskData.tasks | Where-Object { $_.id -eq $TaskId }

    if (-not $task) {
        Write-Host "Task ID $TaskId not found." -ForegroundColor Red
        return
    }

    Write-Host "Analyzing task: $($task.description)" -ForegroundColor Cyan
    Write-Host "Querying LM Studio for step breakdown..." -ForegroundColor Gray
    Write-Host ""

    $systemMessage = @"
You are a software architect and task planner. Break down high-level tasks into specific, actionable steps.
Each step should be:
1. Concrete and specific
2. Testable/verifiable
3. Ordered logically
4. Independent where possible

Format your response as a numbered list.
"@

    $prompt = @"
Break down this software development task into specific, actionable steps:

Task: $($task.description)
Priority: $($task.priority)

Provide a detailed breakdown with concrete steps.
"@

    $response = Invoke-LMStudio -Prompt $prompt -SystemMessage $systemMessage -Temperature 0.3 -MaxTokens 2048

    if ($response) {
        Write-Host "=== AI Task Breakdown ===" -ForegroundColor Green
        Write-Host ""
        Write-Host $response
        Write-Host ""

        # Ask if user wants to add as subtasks
        $addSubtasks = Read-Host "Add these as subtasks? (y/n)"

        if ($addSubtasks -eq 'y') {
            # Parse numbered list and add as subtasks
            $lines = $response -split "`n" | Where-Object { $_ -match "^\d+\." }

            foreach ($line in $lines) {
                $subtaskDesc = $line -replace "^\d+\.\s*", ""
                $task.subtasks += @{
                    description = $subtaskDesc
                    status = "pending"
                    created = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
                }
            }

            $task.updated = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
            Save-Tasks -TaskData $taskData

            Write-Host "Subtasks added successfully!" -ForegroundColor Green
        }
    }
}

# AI-powered code generation
function Invoke-CodeGeneration {
    param(
        [Parameter(Mandatory=$true)]
        [string]$Description,

        [Parameter(Mandatory=$false)]
        [string]$Language = "auto",

        [Parameter(Mandatory=$false)]
        [string]$Context = ""
    )

    Write-Host "Generating code for: $Description" -ForegroundColor Cyan
    Write-Host ""

    $systemMessage = @"
You are an expert software developer. Generate clean, well-documented, production-ready code.
Follow best practices for the language.
Include comments explaining complex logic.
Provide usage examples when appropriate.
"@

    $prompt = "Generate code: $Description"

    if ($Language -ne "auto") {
        $prompt += "`nLanguage: $Language"
    }

    if ($Context) {
        $prompt += "`nContext: $Context"
    }

    $response = Invoke-LMStudio -Prompt $prompt -SystemMessage $systemMessage -Temperature 0.2 -MaxTokens 4096

    if ($response) {
        Write-Host "=== Generated Code ===" -ForegroundColor Green
        Write-Host ""
        Write-Host $response
        Write-Host ""
    }
}

# Note: This script can be imported with: Import-Module .\reasoning_layer.ps1
# The functions will be available in your session

# Display help on module load
Write-Host ""
Write-Host "=== Crush Reasoning Layer Loaded ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Available commands:" -ForegroundColor Yellow
Write-Host "  Add-Task -Description 'task' [-Priority high|medium|low]"
Write-Host "  Get-TaskList [-Status pending|in_progress|completed]"
Write-Host "  Update-TaskStatus -TaskId [id] -Status [status]"
Write-Host "  Remove-Task -TaskId [id]"
Write-Host "  Invoke-TaskReason -TaskId [id]"
Write-Host "  Invoke-CodeGeneration -Description 'what to generate'"
Write-Host ""
