# External Systems Integration

## Language Server Protocol (LSP)
Crush integrates with LSPs to provide semantic awareness of the codebase.

- **Communication:** JSON-RPC over stdio.
- **Capabilities Used:**
  - `textDocument/documentSymbol`: To understand the structure of a file.
  - `textDocument/references`: To find where a symbol is used.
  - `textDocument/publishDiagnostics`: To catch errors and warnings in real-time.
- **Configuration:** Defined in `crush.json` per language (e.g., `gopls` for Go).

## Model Context Protocol (MCP)
Extends the agent's capabilities with external tools.

- **Transports:** `stdio`, `http`, `sse`.
- **Functionality:** Discovers available tools from MCP servers and makes them available to the agent.
- **Security:** Requires user permission for tool execution unless explicitly allowed in configuration.

## Sourcegraph
Integrated for remote code search across public repositories.

- **Interface:** GraphQL API.
- **Usage:** Provides wide-scale code patterns and examples from the open-source ecosystem.

## OS Shell
- **Interface:** Bash (via `mvdan.cc/sh`).
- **Functionality:** Allows the agent to run build commands, tests, and other shell operations.
- **Safety:** Runs in a controlled environment with user-defined permissions.
