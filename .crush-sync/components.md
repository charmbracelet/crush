# Component Classification for Cliffy Sync

## SYNC Components (Keep in sync with Crush)

These benefit from Crush's active development:

- `internal/llm/agent/` - Core agent loop, streaming, events
- `internal/llm/tools/` - All tool implementations
- `internal/lsp/` - LSP client and handlers
- `internal/fsext/` - File system utilities
- `internal/message/` - Message types

**Sync frequency:** Monthly or on critical updates

## DIVERGED Components (Cliffy-specific)

These are intentionally different:

- `cmd/cliffy/` - Entry point and CLI args
- `internal/config/` - Headless-optimized config
- `internal/runner/` - Direct execution, no sessions
- `internal/output/` - Streaming output formatter

**Never sync** - but watch for ideas

## REMOVED Components (Don't sync)

We intentionally don't have:

- `internal/tui/` - Interactive UI
- `internal/db/` - Session persistence
- `internal/session/` - Session management
- `internal/permission/` - Interactive permissions

**Skip entirely** in sync process