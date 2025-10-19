# Crush Scripts - Reasoning Layer

This directory contains scripts that add a reasoning and task management layer to Crush, integrating with LM Studio for AI-powered planning and code generation.

## Overview

The reasoning layer provides:
- **Task Management**: Add, list, update, and remove development tasks
- **AI-Powered Planning**: Break down high-level tasks into actionable steps using LM Studio
- **Code Generation**: Generate code snippets and implementations
- **Local AI Integration**: All AI features run locally through LM Studio (no cloud API calls)

## Files

- `reasoning_layer.ps1` - Main PowerShell module with task management and AI features
- `README.md` - This file

## Quick Start

### 1. Prerequisites

- LM Studio running with a model loaded (e.g., Qwen3-8B)
- Local server started in LM Studio on `http://localhost:1234`
- PowerShell 5.1+ or PowerShell Core 7+

### 2. Load the Module

```powershell
# Navigate to the scripts directory
cd C:\dev\Crush\plan\crush_scripts

# Import the module
Import-Module .\reasoning_layer.ps1 -Force
```

### 3. Basic Usage

```powershell
# Add a task
Add-Task -Description "Implement user authentication with JWT" -Priority high

# List all tasks
Get-TaskList

# Get AI breakdown of a task
Invoke-TaskReason -TaskId 1

# Update task status
Update-TaskStatus -TaskId 1 -Status in_progress

# Generate code
Invoke-CodeGeneration -Description "Create a Python function to validate email addresses"
```

## Commands

### Task Management

#### Add-Task
Add a new task to the task list.

```powershell
Add-Task -Description "Your task description" [-Priority high|medium|low] [-Tags @("tag1", "tag2")]

# Examples:
Add-Task -Description "Implement user authentication"
Add-Task -Description "Fix login bug" -Priority high
Add-Task -Description "Refactor database layer" -Priority low -Tags @("refactor", "backend")
```

**Parameters:**
- `-Description` (required): Task description
- `-Priority` (optional): high, medium, or low (default: medium)
- `-Tags` (optional): Array of tags for categorization

#### Get-TaskList
Display all tasks with optional filtering.

```powershell
Get-TaskList [-Status pending|in_progress|completed|blocked] [-Priority high|medium|low] [-Tag "tag"]

# Examples:
Get-TaskList                           # Show all tasks
Get-TaskList -Status pending           # Show only pending tasks
Get-TaskList -Priority high            # Show only high priority tasks
Get-TaskList -Tag "backend"            # Show tasks tagged with "backend"
```

**Parameters:**
- `-Status` (optional): Filter by status
- `-Priority` (optional): Filter by priority
- `-Tag` (optional): Filter by tag

#### Update-TaskStatus
Update the status of a task.

```powershell
Update-TaskStatus -TaskId <id> -Status pending|in_progress|completed|blocked

# Examples:
Update-TaskStatus -TaskId 1 -Status in_progress
Update-TaskStatus -TaskId 2 -Status completed
Update-TaskStatus -TaskId 3 -Status blocked
```

**Parameters:**
- `-TaskId` (required): Task ID to update
- `-Status` (required): New status

#### Remove-Task
Remove a task from the list.

```powershell
Remove-Task -TaskId <id>

# Example:
Remove-Task -TaskId 5
```

**Parameters:**
- `-TaskId` (required): Task ID to remove

### AI Features

#### Invoke-TaskReason
Use AI to break down a high-level task into specific, actionable steps.

```powershell
Invoke-TaskReason -TaskId <id>

# Example:
Invoke-TaskReason -TaskId 1
```

**Parameters:**
- `-TaskId` (required): Task ID to analyze

**What it does:**
1. Sends the task description to LM Studio
2. Requests a detailed breakdown into actionable steps
3. Displays the AI-generated breakdown
4. Optionally adds the steps as subtasks

**Example workflow:**
```powershell
# Add a high-level task
Add-Task -Description "Build a REST API for user management" -Priority high

# Get AI breakdown
Invoke-TaskReason -TaskId 1

# AI will provide steps like:
# 1. Design API endpoints and data models
# 2. Set up project structure with framework
# 3. Implement user CRUD operations
# 4. Add authentication middleware
# 5. Write unit tests
# 6. Document API with OpenAPI/Swagger
```

#### Invoke-CodeGeneration
Generate code using AI based on a description.

```powershell
Invoke-CodeGeneration -Description "what to generate" [-Language "language"] [-Context "additional context"]

# Examples:
Invoke-CodeGeneration -Description "Create a Python function to validate email addresses"
Invoke-CodeGeneration -Description "Implement JWT token verification" -Language "JavaScript"
Invoke-CodeGeneration -Description "Database connection pool" -Language "Go" -Context "Using PostgreSQL"
```

**Parameters:**
- `-Description` (required): What code to generate
- `-Language` (optional): Programming language (default: auto-detect)
- `-Context` (optional): Additional context for generation

## Aliases

For convenience, shorter aliases are available:

```powershell
task:add            # Add-Task
task:list           # Get-TaskList
task:update         # Update-TaskStatus
task:remove         # Remove-Task
task:ai:reason      # Invoke-TaskReason
task:ai:code        # Invoke-CodeGeneration
```

**Usage with aliases:**
```powershell
task:add -Description "Implement caching layer"
task:list -Status pending
task:ai:reason -TaskId 1
task:ai:code -Description "Redis connection manager in Python"
```

## Data Storage

Tasks are stored in `.crush_tasks.json` in the current directory. This file is automatically created on first use.

**File format:**
```json
{
  "tasks": [
    {
      "id": 1,
      "description": "Task description",
      "priority": "high",
      "status": "pending",
      "tags": ["tag1"],
      "created": "2025-01-15 10:30:00",
      "updated": "2025-01-15 10:30:00",
      "subtasks": []
    }
  ],
  "nextId": 2,
  "created": "2025-01-15 10:30:00"
}
```

## Workflow Examples

### Example 1: Feature Development

```powershell
# Import the module
Import-Module .\reasoning_layer.ps1 -Force

# Add a high-level feature task
Add-Task -Description "Add OAuth2 authentication to API" -Priority high

# Get AI breakdown
Invoke-TaskReason -TaskId 1
# Choose 'y' to add as subtasks

# List tasks to see the breakdown
Get-TaskList

# Start working
Update-TaskStatus -TaskId 1 -Status in_progress

# Generate specific code as needed
Invoke-CodeGeneration -Description "OAuth2 token validation middleware" -Language "Go"

# Mark complete when done
Update-TaskStatus -TaskId 1 -Status completed
```

### Example 2: Bug Fixing

```powershell
# Add bug fix tasks
Add-Task -Description "Fix memory leak in image processor" -Priority high -Tags @("bug", "performance")
Add-Task -Description "Fix race condition in cache invalidation" -Priority high -Tags @("bug", "concurrency")

# List high-priority bugs
Get-TaskList -Priority high -Tag "bug"

# Get AI insights on approach
Invoke-TaskReason -TaskId 1

# Generate diagnostic code
Invoke-CodeGeneration -Description "Memory profiling setup for Go application"
```

### Example 3: Refactoring

```powershell
# Add refactoring task
Add-Task -Description "Refactor authentication module to use dependency injection" -Priority medium -Tags @("refactor")

# Get detailed plan
Invoke-TaskReason -TaskId 1

# Generate example code
Invoke-CodeGeneration -Description "Dependency injection container pattern in Go"

# Track progress
Update-TaskStatus -TaskId 1 -Status in_progress
```

## Configuration

### Customizing LM Studio URL

Edit `reasoning_layer.ps1` and modify:

```powershell
$script:LMStudioUrl = "http://localhost:1234/v1"  # Change port if needed
```

### Customizing Model

Edit `reasoning_layer.ps1` and modify:

```powershell
$script:DefaultModel = "qwen3-8b"  # Change to your model ID
```

### Customizing Task File Location

Edit `reasoning_layer.ps1` and modify:

```powershell
$script:TaskFile = ".crush_tasks.json"  # Change path if needed
```

## Troubleshooting

### Module not loading

```powershell
# Ensure you're in the correct directory
cd C:\dev\Crush\plan\crush_scripts

# Force reload
Import-Module .\reasoning_layer.ps1 -Force

# Check execution policy
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### AI features not working

1. **Check LM Studio is running:**
   ```powershell
   # Test the API
   Invoke-RestMethod http://localhost:1234/v1/models
   ```

2. **Verify model is loaded:**
   - Open LM Studio
   - Go to "Local Server" tab
   - Ensure a model is loaded and server is started

3. **Check the model ID:**
   - Run the test above and note the model ID
   - Update `$script:DefaultModel` if needed

### Tasks not persisting

- Ensure you have write permissions in the current directory
- Check that `.crush_tasks.json` is being created
- Verify the file isn't locked by another process

## Integration with Crush

This reasoning layer is designed to complement Crush's capabilities:

1. **Use Crush for:** File operations, git operations, running commands
2. **Use Reasoning Layer for:** Task planning, AI-assisted development, local code generation

**Example combined workflow:**
```powershell
# Use reasoning layer to plan
Import-Module .\reasoning_layer.ps1 -Force
Add-Task -Description "Implement rate limiting middleware"
Invoke-TaskReason -TaskId 1

# Use Crush for implementation
# (Run crush and let it help with file operations, testing, etc.)

# Use reasoning layer for code snippets
Invoke-CodeGeneration -Description "Rate limiting with token bucket algorithm" -Language "Go"

# Update task status
Update-TaskStatus -TaskId 1 -Status completed
```

## Advanced Usage

### Batch Task Creation

```powershell
$tasks = @(
    "Implement user registration endpoint",
    "Add email verification flow",
    "Create password reset functionality",
    "Add session management"
)

foreach ($task in $tasks) {
    Add-Task -Description $task -Priority high -Tags @("auth", "api")
}
```

### Export Task Report

```powershell
# Generate a report of all pending tasks
Get-TaskList -Status pending | ConvertTo-Json | Out-File pending-tasks-report.json
```

### Custom AI Prompts

Modify the `Invoke-TaskReason` or `Invoke-CodeGeneration` functions to customize the system messages and prompts for your specific needs.

## Resources

- [LM Studio Documentation](https://lmstudio.ai/docs)
- [PowerShell Documentation](https://docs.microsoft.com/powershell/)
- [Crush Documentation](https://github.com/charmbracelet/crush)

## Contributing

This is an experimental scaffold. Improvements welcome:
- Better error handling
- Progress tracking for subtasks
- Time estimation
- Integration with external task trackers
- Export to different formats (Markdown, CSV, etc.)
