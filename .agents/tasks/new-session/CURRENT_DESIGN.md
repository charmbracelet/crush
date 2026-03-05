# Current Design: `new_session` tool

## Purpose
Introduce a `new_session` tool that allows an LLM agent to start a new session programmatically. This serves to free up the context window while keeping the user's project context intact. The agent can provide a summary of the work done so far and what steps remain, creating a clean slate for continuation without degrading performance or exceeding token limits.

## Relevant Codebase Areas

### 1. Tool Creation and Registration
- **Location:** `internal/agent/tools/`
- Tool interfaces are built inside this package and returned as `fantasy.AgentTool`.
- Standard tools include `BashTool`, `EditTool`, `ViewTool`, etc.
- A new file `internal/agent/tools/new_session.go` will be needed to define the tool.

### 2. Coordinator & Agent Bootstrapping
- **Location:** `internal/agent/coordinator.go` -> `buildTools()`
- Tools are appended to the `allTools` slice inside `buildTools()`.
- The new session tool should be registered here.
- The coordinator maintains the list of sessions and can potentially be injected into the tool (or trigger a callback) to instantiate a new session explicitly and switch to it.

### 3. Session Context
- **Location:** `internal/agent/tools/tools.go`
- Uses context keys like `SessionIDContextKey` and `MessageIDContextKey`.
- Tool execution is stateless except for these context keys.
- To execute a new session securely, the new tool might need to signal to the coordinator that the current LLM run should be aborted/completed, and a new empty session should replace it, prepended with the summary provided by the tool.

### 4. UI Rendering
- **Location:** `internal/ui/chat/tools.go` & other module files like `internal/ui/chat/file.go`.
- The UI layer currently renders tools explicitly via `ToolRenderContext`. 
- For `new_session`, a renderer might be needed to display something like "Creating new session..." in the UI smoothly without alarming the user.
- Relevant UI keys/commands for new session start are found in `internal/ui/model/ui.go` and `internal/ui/dialog/sessions.go`.

### 5. Task Skills & Planning
- Located in `internal/agent/tasks/`.
- Pre-filled context needs to ensure it plays well with task tracking files. It should effectively behave as though the user typed "Continue from summary: <...>".

## Known Context Management
- Config checks context lengths (e.g., schemas). Summarization exists (`DisableAutoSummarize`) to try to compact messages.
- The `new_session` tool is an explicit override that allows the model to "turn the page" rather than relying purely on backend compression techniques.