1. [x] Update `internal/agent/tools/new_session.md` with a `<when_to_use>` section referencing context status and default 75% threshold.
2. [x] Inject `<context_status>` system message in `PrepareStep` callback in `internal/agent/agent.go`.
3. [x] Write tests for context status injection.
4. [x] Run full test suite and verify build.
5. [x] Fix VCR cassette test failures by adding an option to skip `<context_status>` injection in tests.
