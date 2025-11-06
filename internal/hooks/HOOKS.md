# Hooks Guide

⚠️ **Security Warning**: Hooks run automatically with your user's permissions and have full access to your filesystem and environment. Only configure hooks from trusted sources and review all commands before adding them.

Hooks are user-defined shell commands that execute at various points in Crush's lifecycle. They provide deterministic control over Crush's behavior, ensuring certain actions always occur rather than relying on the LLM to choose to run them.

## Hook Events

Crush provides several lifecycle events where hooks can run:

### Tool Events
- **`pre_tool_use`**: Runs before tool calls. If a hook fails (non-zero exit code), the tool execution is blocked.
- **`post_tool_use`**: Runs after tool calls complete, can be used to process results or trigger actions.

### Session Events
- **`user_prompt_submit`**: Runs when the user submits a prompt, before processing
- **`stop`**: Runs when Crush finishes responding to a prompt
- **`subagent_stop`**: Runs when subagent tasks complete (e.g., fetch tool, agent tool)

### Other Events
- **`pre_compact`**: Runs before running a compact operation
- **`permission_requested`**: Runs when a permission is requested from the user

## Configuration Format

Hooks are configured in your Crush configuration file. Configuration files are searched in the following order:

1. `.crush.json` (project-specific, hidden)
2. `crush.json` (project-specific)
3. `$HOME/.config/crush/crush.json` (global, Linux/macOS)
   or `%LOCALAPPDATA%\crush\crush.json` (global, Windows)

Example configuration:

```json
{
  "hooks": {
    "pre_tool_use": [
      {
        "matcher": "bash",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '.tool_name + \": \" + .tool_input.command' >> ~/crush-commands.log",
            "timeout": 5
          }
        ]
      }
    ],
    "post_tool_use": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "echo \"Tool $(jq -r .tool_name) completed\" | notify-send \"Crush Hook\""
          }
        ]
      }
    ],
    "stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo \"Prompt completed. Tokens used: $(jq -r .tokens_used)\""
          }
        ]
      }
    ]
  }
}
```

## Hook Context

Each hook receives a JSON context object via stdin containing information about the event:

```json
{
  "event_type": "pre_tool_use",
  "session_id": "abc123",
  "tool_name": "bash",
  "tool_input": {
    "command": "echo hello",
    "description": "Print hello"
  },
  "tool_result": "",
  "tool_error": false,
  "user_prompt": "",
  "timestamp": "2025-10-30T12:00:00Z",
  "working_dir": "/path/to/project",
  "message_id": "msg123",
  "provider": "anthropic",
  "model": "claude-3-5-sonnet-20241022",
  "tokens_used": 1000,
  "tokens_input": 500
}
```

### Context Fields by Event Type

Different events include different fields:

- **pre_tool_use**: `event_type`, `session_id`, `tool_name`, `tool_input`, `message_id`, `provider`, `model`
- **post_tool_use**: `event_type`, `session_id`, `tool_name`, `tool_input`, `tool_result`, `tool_error`, `message_id`, `provider`, `model`
- **user_prompt_submit**: `event_type`, `session_id`, `user_prompt`, `provider`, `model`
- **stop**: `event_type`, `session_id`, `message_id`, `provider`, `model`, `tokens_used`, `tokens_input`

All events include: `event_type`, `timestamp`, `working_dir`

## Environment Variables

Hooks also receive environment variables:

- `CRUSH_HOOK_CONTEXT`: Full JSON context as a string
- `CRUSH_HOOK_EVENT`: The event type (e.g., "PreToolUse")
- `CRUSH_SESSION_ID`: The session ID (if applicable)
- `CRUSH_TOOL_NAME`: The tool name (for tool events)

## Hook Configuration

### Matchers

For tool events (`pre_tool_use`, `post_tool_use`), you can specify matchers to target specific tools:

- `"bash"` - Only matches the bash tool
- `"edit"` - Only matches the edit tool
- `"edit|write|multiedit"` - Matches any of the specified tools (pipe-separated)
- `"*"` or `""` - Matches all tools

For non-tool events, leave the matcher empty or use `"*"`.

### Hook Command

Each hook has these properties:
- `type`: Currently only `"command"` is supported
- `command`: Shell command to execute. Receives JSON context via stdin
- `timeout`: (optional) Maximum execution time in seconds (default: 30, max: 300)

**Important**: When processing JSON with `jq`, be aware that `tool_result` fields can contain large content or special characters that may cause parse errors. For reliability:
- Use `cat` instead of `jq` to output raw JSON: `cat >> hooks.log`
- Extract only specific fields: `jq -r '.tool_name, .session_id'`
- For `post_tool_use` hooks, tool results can be very large (e.g., entire file contents)

## Examples

### Log All Bash Commands

```json
{
  "hooks": {
    "pre_tool_use": [
      {
        "matcher": "bash",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '.timestamp + \" - \" + .tool_input.command' >> ~/crush-bash.log"
          }
        ]
      }
    ]
  }
}
```

### Auto-format Files After Editing

```json
{
  "hooks": {
    "post_tool_use": [
      {
        "matcher": "edit|write|multiedit",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r .tool_input.file_path | xargs prettier --write"
          }
        ]
      }
    ]
  }
}
```

### Notify on Completion

Note: Examples use macOS-specific tools. For cross-platform alternatives, use `notify-send` (Linux) or custom scripts.

```json
{
  "hooks": {
    "stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "osascript -e 'display notification \"Crush completed\" with title \"Crush\"'"
          }
        ]
      }
    ]
  }
}
```

### Track Token Usage

```json
{
  "hooks": {
    "stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '\"\\(.timestamp): \\(.tokens_used) tokens\"' >> ~/crush-tokens.log"
          }
        ]
      }
    ]
  }
}
```

### Validate Tool Usage

```json
{
  "hooks": {
    "pre_tool_use": [
      {
        "matcher": "bash",
        "hooks": [
          {
            "type": "command",
            "command": "if jq -e '.tool_input.command | contains(\"rm -rf\")' > /dev/null; then echo \"Dangerous command detected\" >&2; exit 1; fi"
          }
        ]
      }
    ]
  }
}
```

### Multiple Hooks

You can execute multiple hooks for the same event:

```json
{
  "hooks": {
    "post_tool_use": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r .tool_name >> ~/crush-tools.log"
          },
          {
            "type": "command",
            "command": "if jq -e .tool_error > /dev/null; then echo 'Error in tool' | pbcopy; fi"
          }
        ]
      }
    ]
  }
}
```

### Debug: Log All Hook Events

For debugging or monitoring, log complete JSON for all events:

```json
{
  "hooks": {
    "pre_tool_use": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "(echo \"[$(date '+%Y-%m-%d %H:%M:%S')] pre_tool_use:\" && cat && echo \"\") >> hooks.log"
          }
        ]
      }
    ],
    "post_tool_use": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "(echo \"[$(date '+%Y-%m-%d %H:%M:%S')] post_tool_use:\" && cat && echo \"\") >> hooks.log"
          }
        ]
      }
    ]
  }
}
```

Note: Using `cat` avoids potential jq parsing errors with large or complex tool results.

### Track Subagent Completion

```json
{
  "hooks": {
    "subagent_stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo \"Subagent task completed: $(jq -r .tool_name)\" | tee -a ~/crush-subagent.log"
          }
        ]
      }
    ]
  }
}
```

### Pre-Compact Notification

```json
{
  "hooks": {
    "pre_compact": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "osascript -e 'display notification \"Compacting conversation...\" with title \"Crush\"'"
          }
        ]
      }
    ]
  }
}
```

### Permission Requested Notification

```json
{
  "hooks": {
    "permission_requested": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '\"Permission requested: \\(.tool_name) \\(.permission_action) \\(.permission_path)\"' | tee -a ~/crush-permissions.log"
          }
        ]
      }
    ]
  }
}
```

## Best Practices

1. **Keep hooks fast**: Hooks run synchronously and can slow down Crush if they take too long.
2. **Set appropriate timeouts**: Use shorter timeouts (1-5 seconds) for quick operations.
3. **Handle errors gracefully**: Hooks should not crash or hang.
4. **Use jq for JSON processing**: The context is piped to stdin as JSON.
5. **Test hooks independently**: Run your shell commands manually with test data before configuring them.
6. **Use absolute paths**: Hooks run in the project directory, but absolute paths are more reliable.
7. **Consider privacy**: Don't log sensitive information like API keys or passwords.

## Debugging Hooks

Hooks log errors and warnings to Crush's log output. To see hook execution:

1. Run Crush with debug logging enabled: `crush --debug`
2. Check the logs for hook-related messages
3. Test hook shell commands manually:
   ```bash
   echo '{"event_type":"pre_tool_use","tool_name":"bash"}' | jq -r '.tool_name'
   ```

## Limitations

- Hooks must complete within their timeout (default 30 seconds)
- Hooks run in a shell environment and require shell utilities (bash, jq, etc.)
- Hooks cannot modify Crush's internal state
- Hook errors are logged but don't stop Crush execution (except for pre_tool_use)
- Interactive hooks are not supported
