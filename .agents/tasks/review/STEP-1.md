# Fix context status message to use system role instead of user role

Status: COMPLETED

## Sub tasks

1. [x] Change `fantasy.NewUserMessage` to `fantasy.NewSystemMessage` in `contextStatusMessage()` at `agent.go:947`
2. [x] Update `context_status_test.go` to expect `fantasy.MessageRoleSystem` (2 occurrences)
3. [x] Run tests — all pass

## NOTES

- Changed `agent.go:947`: `NewUserMessage` → `NewSystemMessage`
- Changed `context_status_test.go:33` and `:203`: `MessageRoleUser` → `MessageRoleSystem`
- Comment on line 932 already says "system message" so no comment change needed.
