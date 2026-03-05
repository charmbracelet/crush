# Add/update tests for mid-session compaction switching

Status: COMPLETED

## Sub tasks

1. [x] Enhance `mockSessionAgent` to capture `SetCompactionFlags` calls
2. [x] Add `TestSetCompactionFlags` for the real `sessionAgent` implementation
3. [x] Add `TestCompactionModeSwitchRoundTrip` for end-to-end flag switching
4. [x] Run tests to confirm all pass
5. [x] Run `gofumpt` to format

## NOTES

Created `internal/agent/set_compaction_flags_test.go` with two test suites:

### `TestSetCompactionFlags`
- Tests `sessionAgent.SetCompactionFlags()` directly mutates the `*csync.Value[bool]` fields
- Covers auto→llm and llm→auto transitions

### `TestCompactionModeSwitchRoundTrip`
- Tests the full round-trip: `compactionFlags()` derivation → `SetCompactionFlags()` → verify csync reads
- Covers auto→llm, llm→auto, and llm→auto with legacy override
- This mirrors what `UpdateModels` does: derive flags from config, apply via `SetCompactionFlags`

Also enhanced `mockSessionAgent` in `coordinator_test.go` to track `SetCompactionFlags` calls (fields `compactionFlagsCalls` and `compactionFlagsCallCount`).

Note: Direct testing of `UpdateModels` calling `SetCompactionFlags` would require setting up full provider config and provider construction. The round-trip test covers the same logic path without that complexity.
