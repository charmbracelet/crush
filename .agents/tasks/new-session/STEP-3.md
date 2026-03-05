# Add UI renderer and transition logic for the `new_session` tool in `internal/ui/`

Status: COMPLETED

## Sub tasks

1. [x] Create `internal/ui/chat/new_session.go` with `NewSessionToolMessageItem` and `NewSessionToolRenderContext`.
2. [x] Add `case tools.NewSessionToolName` to the switch in `NewToolMessageItem()` in `internal/ui/chat/tools.go`.
3. [x] Verify compilation.

## NOTES

Created `internal/ui/chat/new_session.go` following the same pattern as `lsp_restart.go`:
- `NewSessionToolRenderContext.RenderTool()` shows "New Session" as the tool header.
- Pending state shows spinner with "New Session" label.
- Compact mode returns just the header.
- Full mode renders tool output body if present.

The case was added in `tools.go` after `LSPRestartToolName` and before the `default` block.
