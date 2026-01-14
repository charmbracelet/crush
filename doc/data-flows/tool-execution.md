# Data Flow: Tool Execution

This flow describes how the AI agent calls a tool (built-in or MCP) and how the permission system and results are handled.

```mermaid
sequenceDiagram
    participant LLM as AI Provider
    participant SA as Session Agent
    participant P as Permission Service
    participant TUI as TUI (Permission Dialog)
    participant Tool as Tool Implementation
    participant Target as External System (OS/LSP/MCP)

    LLM-->>SA: Tool Call Request (name, input)
    SA->>P: Request Permission
    alt Not Auto-Approved
        P->>TUI: Show Permission Dialog
        TUI-->>P: User Approved
    end
    P-->>SA: Permission Granted
    
    SA->>Tool: Run(input)
    Tool->>Target: Execute Action
    Target-->>Tool: Return Raw Result
    Tool-->>SA: Return ToolResponse
    SA->>LLM: Send Tool Result
```

## Tool Categories

### Built-in Tools
Implemented directly in Go within `internal/agent/tools/`.
- **File Tools:** `view`, `ls`, `grep`, `edit`, `multiedit`, `write`.
- **System Tools:** `bash`, `download`, `fetch`.
- **LSP Tools:** `lsp_diagnostics`, `lsp_references`.

### MCP Tools
Dynamically discovered from external MCP servers.
- **Naming:** Prefixed with `mcp_[server_name]_`.
- **Discovery:** Happens at application startup and refreshes on MCP events.
- **Execution:** Calls the external MCP server over the configured transport (stdio, HTTP, SSE).

## Permission System
- **Default:** Always prompts the user for confirmation.
- **Auto-Approval:** Can be configured for specific tools in `crush.json` via `permissions.allowed_tools`.
- **YOLO Mode:** Enabled with `--yolo` flag, skips all permission prompts.
