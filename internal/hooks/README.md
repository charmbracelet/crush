# Hooks Package

A Git-like hooks system for Crush that allows users to intercept and modify behavior at key points in the application lifecycle.

## Overview

The hooks package provides a flexible, shell-based system for customizing Crush behavior through executable scripts. Hooks can:

- Add context to LLM requests
- Control tool execution permissions
- Modify prompts and tool parameters
- Audit and log activity
- Execute cleanup on shutdown

### Cross-Platform Support

The hooks system works on **Windows, macOS, and Linux**:

- **Hook Files**: All hooks must be `.sh` files (shell scripts)
- **Shell Execution**: Uses Crush's internal POSIX shell emulator (`mvdan.cc/sh`) on all platforms
- **Hook Discovery**:
  - **Unix/macOS**: `.sh` files must have execute permission (`chmod +x hook.sh`)
  - **Windows**: `.sh` files are automatically recognized (no permission needed)
- **Path Separators**: Use forward slashes (`/`) in hook scripts for cross-platform compatibility

**Example**:
```bash
# Works on Windows, macOS, and Linux
.crush/hooks/pre-tool-use/01-check.sh
```

## Quick Start

### Creating a Hook

1. Create an executable script in `.crush/hooks/{hook-type}/`:

```bash
#!/bin/bash
# .crush/hooks/pre-tool-use/01-block-dangerous.sh

if [ "$CRUSH_TOOL_NAME" = "bash" ]; then
  COMMAND=$(crush_get_tool_input command)
  if [[ "$COMMAND" =~ "rm -rf /" ]]; then
    crush_deny "Blocked dangerous command"
  fi
fi
```

2. Make it executable:

```bash
chmod +x .crush/hooks/pre-tool-use/01-block-dangerous.sh
```

3. The hook will automatically execute when the event occurs.

## Hook Types

### 1. UserPromptSubmit

**When**: After user submits prompt, before sending to LLM  
**Use cases**: Add context, modify prompts, validate input  
**Location**: `.crush/hooks/user-prompt-submit/`

**Available data** (via stdin JSON):
- `prompt` - User's prompt text
- `attachments` - List of attached files
- `model` - Model name
- `is_first_message` - Boolean indicating if this is the first message in the conversation

**Example**:
```bash
#!/bin/bash
# Add git context to every prompt, and README only for first message

BRANCH=$(git branch --show-current 2>/dev/null)
if [ -n "$BRANCH" ]; then
  crush_add_context "Current branch: $BRANCH"
fi

# Only add README context for the first message to avoid repetition
IS_FIRST=$(crush_get_input is_first_message)
if [ "$IS_FIRST" = "true" ] && [ -f "README.md" ]; then
  crush_add_context_file "README.md"
fi
```

### 2. PreToolUse

**When**: After LLM requests tool use, before permission check & execution  
**Use cases**: Auto-approve, deny dangerous commands, audit requests  
**Location**: `.crush/hooks/pre-tool-use/`

**Available data** (via stdin JSON):
- `tool_input` - Tool parameters (object)

**Environment variables**:
- `$CRUSH_TOOL_NAME` - Name of the tool being called
- `$CRUSH_TOOL_CALL_ID` - Unique ID for this tool call

**Example**:
```bash
#!/bin/bash
# Auto-approve read-only tools and modify parameters

case "$CRUSH_TOOL_NAME" in
  view|ls|grep|glob)
    crush_approve "Auto-approved read-only tool"
    ;;
  bash)
    COMMAND=$(crush_get_tool_input command)
    if [[ "$COMMAND" =~ ^(ls|cat|grep) ]]; then
      crush_approve "Auto-approved safe bash command"
    fi
    ;;
  view)
    # Limit file reads to 1000 lines max for performance
    crush_modify_input "limit" "1000"
    ;;
esac
```

### 3. PostToolUse

**When**: After tool executes, before result sent to LLM  
**Use cases**: Filter output, redact secrets, log results  
**Location**: `.crush/hooks/post-tool-use/`

**Available data** (via stdin JSON):
- `tool_input` - Tool parameters (object)
- `tool_output` - Tool result (object with `success`, `content`)
- `execution_time_ms` - How long the tool took

**Environment variables**:
- `$CRUSH_TOOL_NAME` - Name of the tool
- `$CRUSH_TOOL_CALL_ID` - Unique ID for this tool call

**Example**:
```bash
#!/bin/bash
# Redact sensitive information from tool output

# Get tool output using helper (stdin is automatically available)
OUTPUT_CONTENT=$(crush_get_input tool_output | jq -r '.content // empty')

# Check if output contains sensitive patterns
if echo "$OUTPUT_CONTENT" | grep -qE '(password|api[_-]?key|secret|token)'; then
  # Redact sensitive data
  REDACTED=$(echo "$OUTPUT_CONTENT" | sed -E 's/(password|api[_-]?key|secret|token)[[:space:]]*[:=][[:space:]]*[^[:space:]]+/\1=\[REDACTED\]/gi')
  crush_modify_output "content" "$REDACTED"
  crush_log "Redacted sensitive information from $CRUSH_TOOL_NAME output"
fi
```

### 4. Stop

**When**: When agent conversation loop stops or is cancelled  
**Use cases**: Save conversation state, cleanup session resources, archive logs  
**Location**: `.crush/hooks/stop/`

**Available data** (via stdin JSON):
- `reason` - Why the loop stopped (e.g., "completed", "cancelled", "error")
- `session_id` - The session ID that stopped

**Example**:
```bash
#!/bin/bash
# Save conversation summary when agent loop stops

REASON=$(crush_get_input reason)
SESSION_ID=$(crush_get_input session_id)

# Archive session logs
if [ -f ".crush/session-$SESSION_ID.log" ]; then
  ARCHIVE="logs/session-$SESSION_ID-$(date +%Y%m%d-%H%M%S).log"
  mkdir -p logs
  mv ".crush/session-$SESSION_ID.log" "$ARCHIVE"
  gzip "$ARCHIVE"
  crush_log "Archived session logs: $ARCHIVE.gz (reason: $REASON)"
fi
```

## Catch-All Hooks

Place hooks at the **root level** (`.crush/hooks/*.sh`) to run for **ALL hook types**:

```bash
#!/bin/bash
# .crush/hooks/00-global-log.sh
# This runs for every hook type

echo "[$CRUSH_HOOK_TYPE] Session: $CRUSH_SESSION_ID" >> global.log
```

**Execution order**:
1. Catch-all hooks (alphabetically sorted)
2. Type-specific hooks (alphabetically sorted)

Use `$CRUSH_HOOK_TYPE` to determine which event triggered the hook.

## Helper Functions

All hooks have access to these built-in functions (no sourcing required):

### Permission Helpers

#### `crush_approve [message]`
Approve the current tool call (PreToolUse only).

```bash
crush_approve "Auto-approved read-only command"
```

#### `crush_deny [message]`
Deny the current tool call and stop execution (PreToolUse only).

```bash
crush_deny "Blocked dangerous operation"
# Script exits immediately with code 2
```

#### `crush_ask [message]`
Ask user for permission (default behavior).

```bash
crush_ask "This command modifies files, please review"
```

### Context Helpers

#### `crush_add_context "content"`
Add raw text content to LLM context.

```bash
crush_add_context "Project uses React 18 with TypeScript"
```

#### `crush_add_context_file "path"`
Load a file and add its content to LLM context.

```bash
crush_add_context_file "docs/ARCHITECTURE.md"
crush_add_context_file "package.json"
```

### Modification Helpers

#### `crush_modify_prompt "new_prompt"`
Replace the user's prompt (UserPromptSubmit only).

```bash
PROMPT=$(crush_get_prompt)
MODIFIED="$PROMPT\n\nNote: Always use TypeScript."
crush_modify_prompt "$MODIFIED"
```

#### `crush_modify_input "param_name" "value"`
Modify tool input parameters (PreToolUse only).

Values are parsed as JSON when valid, supporting all JSON types (strings, numbers, booleans, arrays, objects).

```bash
# Strings (no quotes needed for simple strings)
crush_modify_input "command" "ls -la"
crush_modify_input "working_dir" "/tmp"

# Numbers (parsed as JSON)
crush_modify_input "offset" "100"
crush_modify_input "limit" "50"

# Booleans (parsed as JSON)
crush_modify_input "run_in_background" "true"
crush_modify_input "replace_all" "false"

# Arrays (JSON format)
crush_modify_input "ignore" '["*.log","*.tmp"]'

# Quoted strings (for strings with spaces or special chars)
crush_modify_input "message" '"hello world"'
```

#### `crush_modify_output "field_name" "value"`
Modify tool output before sending to LLM (PostToolUse only).

```bash
# Redact sensitive information from tool output content
crush_modify_output "content" "[REDACTED - sensitive data removed]"

# Can also modify other fields in the tool_output object
crush_modify_output "success" "false"
```

#### `crush_stop [message]`
Stop execution immediately.

```bash
if [ "$(date +%H)" -lt 9 ]; then
  crush_stop "Crush is only available during business hours"
fi
```

### Input Parsing Helpers

Hooks receive JSON context via stdin, which is automatically saved and available to all helper functions. You can call multiple helpers without manually reading stdin first.

#### `crush_get_input "field_name"`
Get a top-level field from the hook context.

```bash
# Can call multiple times without saving stdin
PROMPT=$(crush_get_input prompt)
MODEL=$(crush_get_input model)
```

#### `crush_get_tool_input "parameter"`
Get a tool parameter (PreToolUse/PostToolUse only).

```bash
# Can call multiple times without saving stdin
COMMAND=$(crush_get_tool_input command)
FILE_PATH=$(crush_get_tool_input file_path)
```

#### `crush_get_prompt`
Get the user's prompt (UserPromptSubmit only).

```bash
PROMPT=$(crush_get_prompt)
if [[ "$PROMPT" =~ "password" ]]; then
  crush_stop "Never include passwords in prompts"
fi
```

### Logging Helper

#### `crush_log "message"`
Write to Crush's log (stderr).

```bash
crush_log "Processing hook for tool: $CRUSH_TOOL_NAME"
```

## Environment Variables

All hooks have access to these environment variables:

### Always Available
- `$CRUSH_HOOK_TYPE` - Type of hook: `user-prompt-submit`, `pre-tool-use`, `post-tool-use`, `stop`
- `$CRUSH_SESSION_ID` - Current session ID
- `$CRUSH_WORKING_DIR` - Working directory

### Tool Hooks (PreToolUse, PostToolUse)
- `$CRUSH_TOOL_NAME` - Name of the tool being called
- `$CRUSH_TOOL_CALL_ID` - Unique ID for this tool call

## Result Communication

Hooks communicate results back to Crush in two ways:

### 1. Environment Variables (Simple)

Export variables to set hook results:

```bash
export CRUSH_PERMISSION=approve
export CRUSH_MESSAGE="Auto-approved"
export CRUSH_CONTINUE=false
export CRUSH_CONTEXT_CONTENT="Additional context"
export CRUSH_CONTEXT_FILES="/path/to/file1.md:/path/to/file2.md"
```

**Available variables**:
- `CRUSH_PERMISSION` - `approve`, `ask`, or `deny`
- `CRUSH_MESSAGE` - User-facing message
- `CRUSH_CONTINUE` - `true` or `false` (stop execution)
- `CRUSH_MODIFIED_PROMPT` - New prompt text
- `CRUSH_MODIFIED_INPUT` - Modified tool input (format: `key=value:key2=value2`, values parsed as JSON)
- `CRUSH_MODIFIED_OUTPUT` - Modified tool output (format: `key=value:key2=value2`, values parsed as JSON)
- `CRUSH_CONTEXT_CONTENT` - Text to add to LLM context
- `CRUSH_CONTEXT_FILES` - Colon-separated file paths

**Note**: `CRUSH_MODIFIED_INPUT` and `CRUSH_MODIFIED_OUTPUT` use `:` as delimiter between pairs. For complex values with multiple fields or nested structures, use JSON output instead (see below).

### 2. JSON Output (Complex)

Echo JSON to stdout for complex modifications:

```bash
echo '{
  "permission": "approve",
  "message": "Modified command",
  "modified_input": {
    "command": "ls -la --color=auto"
  },
  "context_content": "Added context"
}'
```

**JSON fields**:
- `continue` (bool) - Continue execution
- `permission` (string) - `approve`, `ask`, `deny`
- `message` (string) - User-facing message
- `modified_prompt` (string) - New prompt
- `modified_input` (object) - Modified tool parameters
- `modified_output` (object) - Modified tool results
- `context_content` (string) - Context to add
- `context_files` (array) - File paths to load

**Note**: Environment variables and JSON output are merged automatically.

## Exit Codes

- **0** - Success, continue execution
- **1** - Error (PreToolUse: denies permission, others: logs and continues)
- **2** - Deny/stop execution (sets `Continue=false`)

```bash
# Example: Check rate limit
COUNT=$(grep -c "$(date +%Y-%m-%d)" usage.log)
if [ "$COUNT" -gt 100 ]; then
  echo "Rate limit exceeded" >&2
  exit 2  # Stops execution
fi
```

## Hook Ordering

Hooks execute **sequentially** in alphabetical order. Use numeric prefixes to control order:

```
.crush/hooks/
  00-global-log.sh          # Catch-all: runs first for all types
  pre-tool-use/
    01-rate-limit.sh        # Runs first
    02-auto-approve.sh      # Runs second
    99-audit.sh             # Runs last
```

## Result Merging

When multiple hooks execute, their results are merged:

### Permission (Most Restrictive Wins)
- `deny` > `ask` > `approve`
- If any hook denies, the final result is deny

### Continue (AND Logic)
- All hooks must set `Continue=true` (or not set it)
- If any hook sets `Continue=false`, execution stops

### Context (Append)
- Context content from all hooks is concatenated
- Context files from all hooks are combined

### Messages (Append)
- Messages are joined with `; ` separator

### Modified Fields (Last Wins)
- Modified prompt: last hook's value wins
- Modified input/output: maps are merged, last value wins for conflicts

## Configuration

Configure hooks in `crush.json`:

```json
{
  "hooks": {
    "enabled": true,
    "timeout_seconds": 30,
    "directories": [
      "/path/to/custom/hooks",
      ".crush/hooks"
    ],
    "disabled": [
      "pre-tool-use/slow-check.sh",
      "user-prompt-submit/verbose.sh"
    ],
    "environment": {
      "CUSTOM_VAR": "value"
    },
    "inline": {
      "pre-tool-use": [{
        "name": "rate-limit",
        "script": "#!/bin/bash\n# Inline hook script here..."
      }]
    }
  }
}
```

### Configuration Options

- **enabled** (bool) - Enable/disable the entire hooks system (default: `true`)
- **timeout_seconds** (int) - Maximum execution time per hook (default: `30`)
- **directories** ([]string) - Additional directories to search for hooks
- **disabled** ([]string) - List of hook paths to skip (relative to hooks directory)
- **environment** (map) - Environment variables to pass to all hooks
- **inline** (map) - Hooks defined directly in config (by hook type)

## Best Practices

### 1. Keep Hooks Fast
Hooks run synchronously. Keep them under 1 second to avoid slowing down the UI.

```bash
# Bad: Slow network call
curl -X POST https://api.example.com/log

# Good: Log locally, sync in background
echo "$LOG_ENTRY" >> audit.log
```

### 2. Handle Errors Gracefully
Don't let hooks crash. Use error handling:

```bash
BRANCH=$(git branch --show-current 2>/dev/null)
if [ -n "$BRANCH" ]; then
  crush_add_context "Branch: $BRANCH"
fi
```

### 3. Use Descriptive Names
Use numeric prefixes and descriptive names:

```bash
01-security-check.sh      # Good
99-audit-log.sh           # Good
hook.sh                   # Bad
```

### 4. Test Hooks Independently
Run hooks manually to test:

```bash
export CRUSH_HOOK_TYPE=pre-tool-use
export CRUSH_TOOL_NAME=bash
echo '{"tool_input":{"command":"rm -rf /"}}' | .crush/hooks/pre-tool-use/01-block-dangerous.sh
echo "Exit code: $?"
```

### 5. Log for Debugging
Use `crush_log` to debug hook execution:

```bash
crush_log "Checking command: $COMMAND"
if [[ "$COMMAND" =~ "dangerous" ]]; then
  crush_log "Blocking dangerous command"
  crush_deny "Command blocked"
fi
```

### 6. Don't Block on I/O
Avoid blocking operations:

```bash
# Bad: Waits for user input
read -p "Continue? " answer

# Bad: Long-running process
./expensive-analysis.sh

# Good: Quick checks
[ -f ".allowed" ] && crush_approve
```
