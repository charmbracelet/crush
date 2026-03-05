# Run full test suite and verify build

Status: COMPLETED

## Sub tasks

1. [x] Run `go build .`
2. [x] Run `go test ./...`
3. [x] Verify only expected failures (VCR cassette mismatches)

## NOTES

- `go build .` succeeds cleanly.
- `go test ./...` passes everywhere except `TestCoderAgent` VCR tests.
- All `TestCoderAgent` subtests fail across all providers (anthropic-sonnet, openai-gpt-5, openrouter-kimi-k2, zai-glm4.6) because the VCR cassettes were recorded before the `<context_status>` injection. The cassettes need to be re-recorded with API keys.
- All new `TestContextStatusMessage*` tests pass.
- No other test regressions.
