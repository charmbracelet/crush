# Validate empty summary in new_session tool

Status: COMPLETED

## Sub tasks

1. [x] Add `strings.TrimSpace` validation in `new_session.go` handler
2. [x] Return `fantasy.ToolResponse{IsError: true}` for empty/whitespace summaries
3. [x] Update `TestNewSessionToolEmptySummary` to expect tool error, not Go error
4. [x] Add `TestNewSessionToolWhitespaceSummary` for whitespace-only input
5. [x] Run tests — all pass

## NOTES

- Added `strings` import to `new_session.go`
- Empty or whitespace-only summaries now return a tool error the LLM can see and retry, instead of propagating `NewSessionError` which would cause `ErrEmptyPrompt` in the UI.
