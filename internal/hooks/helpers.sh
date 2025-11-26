#!/bin/bash
# Crush Hook Helper Functions
# These functions are automatically available in all hooks.
# No need to source this file - it's prepended automatically.

# Permission helpers

# Approve the current tool call.
# Usage: crush_approve ["message"]
crush_approve() {
  export CRUSH_PERMISSION=approve
  [ -n "$1" ] && export CRUSH_MESSAGE="$1"
}

# Deny the current tool call.
# Usage: crush_deny ["message"]
crush_deny() {
  export CRUSH_PERMISSION=deny
  export CRUSH_CONTINUE=false
  [ -n "$1" ] && export CRUSH_MESSAGE="$1"
  exit 2
}

# Context helpers

# Add raw text content to LLM context.
# Usage: crush_add_context "content"
crush_add_context() {
  export CRUSH_CONTEXT_CONTENT="$1"
}

# Add a file to be loaded into LLM context.
# Usage: crush_add_context_file "/path/to/file.md"
crush_add_context_file() {
  if [ -z "$CRUSH_CONTEXT_FILES" ]; then
    export CRUSH_CONTEXT_FILES="$1"
  else
    export CRUSH_CONTEXT_FILES="$CRUSH_CONTEXT_FILES:$1"
  fi
}

# Modification helpers

# Modify the user prompt (UserPromptSubmit hooks only).
# Usage: crush_modify_prompt "new prompt text"
crush_modify_prompt() {
  export CRUSH_MODIFIED_PROMPT="$1"
}

# Modify tool input parameters (PreToolUse hooks only).
# Values are parsed as JSON when valid, supporting strings, numbers, booleans, arrays, objects.
# Usage: crush_modify_input "param_name" "value"
# Examples:
#   crush_modify_input "command" "ls -la"
#   crush_modify_input "offset" "100"
#   crush_modify_input "run_in_background" "true"
#   crush_modify_input "ignore" '["*.log","*.tmp"]'
crush_modify_input() {
  local key="$1"
  local value="$2"
  if [ -z "$CRUSH_MODIFIED_INPUT" ]; then
    export CRUSH_MODIFIED_INPUT="$key=$value"
  else
    export CRUSH_MODIFIED_INPUT="$CRUSH_MODIFIED_INPUT:$key=$value"
  fi
}

# Modify tool output (PostToolUse hooks only).
# Usage: crush_modify_output "field_name" "value"
crush_modify_output() {
  local key="$1"
  local value="$2"
  if [ -z "$CRUSH_MODIFIED_OUTPUT" ]; then
    export CRUSH_MODIFIED_OUTPUT="$key=$value"
  else
    export CRUSH_MODIFIED_OUTPUT="$CRUSH_MODIFIED_OUTPUT:$key=$value"
  fi
}

# Stop execution.
# Usage: crush_stop ["message"]
crush_stop() {
  export CRUSH_CONTINUE=false
  [ -n "$1" ] && export CRUSH_MESSAGE="$1"
  exit 1
}

