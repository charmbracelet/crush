# PowerShell Wrappers for LM Studio

This directory contains PowerShell scripts to interact with LM Studio's local API server.

## Scripts

### send_prompt.ps1

Sends a prompt to the local LM Studio server and returns the response.

**Usage:**

```powershell
# Basic usage
.\send_prompt.ps1 -Prompt "Write a hello world in Python"

# With custom model
.\send_prompt.ps1 -Prompt "Explain async/await in JavaScript" -Model "qwen3-8b"

# With custom temperature
.\send_prompt.ps1 -Prompt "Generate a creative story" -Temperature 0.9

# With pretty output (shows metadata)
.\send_prompt.ps1 -Prompt "What is 2+2?" -Pretty

# All parameters
.\send_prompt.ps1 `
    -Prompt "Generate a ROS2 launch file for dual CSI cameras" `
    -Model "qwen3-8b" `
    -Temperature 0.3 `
    -MaxTokens 4096 `
    -BaseUrl "http://localhost:1234/v1" `
    -Pretty
```

**Parameters:**

- `-Prompt` (required): The prompt to send to the model
- `-Model` (optional): Model ID (default: "qwen3-8b")
- `-Temperature` (optional): Temperature for generation (default: 0.7)
- `-MaxTokens` (optional): Maximum tokens to generate (default: 2048)
- `-BaseUrl` (optional): Base URL for LM Studio API (default: "http://localhost:1234/v1")
- `-Pretty` (switch): Show formatted output with metadata

**Examples:**

```powershell
# Code generation with low temperature for precision
.\send_prompt.ps1 `
    -Prompt "Write a Python function to calculate fibonacci numbers" `
    -Temperature 0.3 `
    -Pretty

# Creative writing with higher temperature
.\send_prompt.ps1 `
    -Prompt "Write a short sci-fi story about AI" `
    -Temperature 0.9 `
    -MaxTokens 4096

# Quick question
.\send_prompt.ps1 -Prompt "What is the capital of France?"
```

### list_models.ps1

Lists all available models from the LM Studio server.

**Usage:**

```powershell
# Basic usage (default localhost:1234)
.\list_models.ps1

# Custom base URL
.\list_models.ps1 -BaseUrl "http://localhost:1234/v1"
```

**Parameters:**

- `-BaseUrl` (optional): Base URL for LM Studio API (default: "http://localhost:1234/v1")

**Example Output:**

```
=== Available Models ===

Model ID: qwen3-8b
  Object: model
  Owned By: lm-studio

Total models: 1
```

## Setup

### Prerequisites

1. **LM Studio** must be installed and running
2. **Local server** must be started in LM Studio
3. **Model** must be loaded (e.g., Qwen3-8B)
4. **PowerShell** 5.1+ or PowerShell Core 7+ (Windows)

### First-Time Setup

1. Download and install LM Studio from https://lmstudio.ai/
2. Download a Qwen3-8B model (Q4 or Q5 quantization)
3. Start the local server:
   - Open LM Studio
   - Go to "Local Server" tab
   - Select your model
   - Click "Start Server"
4. Verify the server is running at http://localhost:1234

### Testing the Scripts

```powershell
# Navigate to the wrappers directory
cd C:\dev\Crush\plan\powershell_wrappers

# Test listing models
.\list_models.ps1

# Test sending a prompt
.\send_prompt.ps1 -Prompt "Hello, how are you?" -Pretty
```

## Integration with Crush

These scripts can be called from Crush's reasoning layer or used standalone for testing.

### Example: Task Planning

```powershell
$task = "Implement user authentication with JWT"
$prompt = @"
You are a software architect. Break down this task into concrete steps:

Task: $task

Provide a numbered list of specific, actionable steps.
"@

.\send_prompt.ps1 -Prompt $prompt -Temperature 0.3 -Pretty
```

### Example: Code Generation

```powershell
$request = "Create a Python function that validates email addresses using regex"
.\send_prompt.ps1 -Prompt $request -Temperature 0.2 -MaxTokens 1024
```

## Troubleshooting

### "Cannot connect to LM Studio"

**Solutions:**
1. Verify LM Studio is running
2. Check the local server is started
3. Confirm the port is 1234 (or adjust scripts)
4. Check Windows Firewall isn't blocking localhost connections

### "Model not found"

**Solutions:**
1. Load a model in LM Studio's server tab
2. Check the model ID matches what you're requesting
3. Run `list_models.ps1` to see available models

### PowerShell execution policy error

```powershell
# Run PowerShell as Administrator and execute:
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Slow responses

**Solutions:**
1. Reduce `-MaxTokens` parameter
2. Use Q4 quantization instead of Q5
3. Ensure GPU layers are fully offloaded in LM Studio
4. Close other GPU-intensive applications

## Advanced Usage

### Piping to Files

```powershell
# Save response to a file
.\send_prompt.ps1 -Prompt "Write a README for a Python project" | Out-File README.md
```

### Batch Processing

```powershell
# Process multiple prompts
$prompts = @(
    "Explain variables in Python",
    "Explain functions in Python",
    "Explain classes in Python"
)

foreach ($prompt in $prompts) {
    Write-Host "Processing: $prompt"
    .\send_prompt.ps1 -Prompt $prompt -Temperature 0.3
    Write-Host "---"
}
```

### Custom System Messages

Modify `send_prompt.ps1` to include a system message:

```powershell
# Add before the user message in the messages array:
@{
    role = "system"
    content = "You are a helpful coding assistant specialized in Python."
}
```

## API Reference

The scripts use the OpenAI-compatible API format that LM Studio provides:

**Endpoint:** `http://localhost:1234/v1/chat/completions`

**Request Format:**
```json
{
  "model": "qwen3-8b",
  "messages": [
    {"role": "user", "content": "Your prompt here"}
  ],
  "temperature": 0.7,
  "max_tokens": 2048
}
```

**Response Format:**
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "qwen3-8b",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Response text here"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}
```

## Resources

- [LM Studio Documentation](https://lmstudio.ai/docs)
- [OpenAI API Reference](https://platform.openai.com/docs/api-reference/chat)
- [PowerShell Documentation](https://docs.microsoft.com/powershell/)
