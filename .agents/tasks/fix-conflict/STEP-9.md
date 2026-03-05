# Add SetCompactionFlags to SessionAgent interface and implement it

Status: COMPLETED

## Sub tasks

1. [x] Add `SetCompactionFlags(disableAutoSummarize, disableContextStatus bool)` to the `SessionAgent` interface
2. [x] Implement `SetCompactionFlags` on `*sessionAgent`
3. [x] Add no-op stub on `mockSessionAgent` in `coordinator_test.go`
4. [x] Run build and tests to verify

## NOTES

- Added `SetCompactionFlags(disableAutoSummarize, disableContextStatus bool)` to interface at `agent.go:82`
- Implementation on `*sessionAgent` at `agent.go:1050-1053`, uses `.Set()` on the csync values
- Mock stub added at `coordinator_test.go:30`
- Build passes, all agent tests pass
