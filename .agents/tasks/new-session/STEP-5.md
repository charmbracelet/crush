# Add `"new_session"` to the tool allowlist and update config tests

Status: COMPLETED

## Sub tasks

1. [x] Add `"new_session"` to the `allToolNames()` list in `internal/config/config.go`, inserting it alphabetically after `"ls"`.
2. [x] Verify `TestConfig_setupAgentsWithNoDisabledTools` passes automatically (it derives from `allToolNames()`).
3. [x] Update `TestConfig_setupAgentsWithDisabledTools` — added `"new_session"` to expected coder `AllowedTools` slice after `"ls"`.
4. [x] Update `TestConfig_setupAgentsWithEveryReadOnlyToolDisabled` — added `"new_session"` to expected coder `AllowedTools` slice after `"lsp_restart"` (since `"ls"` is disabled in this test).
5. [x] Ran `go test ./internal/config/ -run TestConfig_setupAgents` — all three tests pass.
6. [x] Ran `go test ./internal/agent/tools/` — all existing new_session tests pass.
7. [x] Ran `go build .` — builds successfully.

## NOTES

### Changes made

- `internal/config/config.go:720`: Added `"new_session",` after `"ls",` in the `allToolNames()` slice.
- `internal/config/load_test.go:489`: Added `"new_session"` after `"ls"` in the expected `AllowedTools` for `TestConfig_setupAgentsWithDisabledTools`.
- `internal/config/load_test.go:512`: Added `"new_session"` after `"agentic_fetch"` in the expected `AllowedTools` for `TestConfig_setupAgentsWithEveryReadOnlyToolDisabled` (since `"ls"` is in the disabled list for that test, `"new_session"` appears after `"agentic_fetch"`).
