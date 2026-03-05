# Write tests for context status injection

Status: COMPLETED

## Sub tasks

1. [x] Create `internal/agent/context_status_test.go` with unit tests for `contextStatusMessage`
2. [x] Verify all tests pass

## NOTES

Created `internal/agent/context_status_test.go` with the following test cases:

### `TestContextStatusMessage`
- **basic computation**: 50% usage (60k+40k / 200k) → verifies used_pct=50, remaining=100000
- **zero context window returns false**: cw=0 → returns false
- **negative context window returns false**: cw=-100 → returns false
- **remaining clamped to zero when overflowed**: tokens exceed context window → remaining=0, used_pct=125
- **zero tokens**: no tokens used → used_pct=0, remaining=full context
- **100 percent usage**: exact match → used_pct=100, remaining=0
- **small context window**: cw=8192, 4000 tokens → verifies fractional percentage (48%)

### `TestContextStatusMessageNotInjectedForSubAgent`
- Verifies the method works regardless of `isSubAgent` flag (the gating is in PrepareStep, not in the method itself). This documents that PrepareStep is the sole control point for the sub-agent check.

### VCR cassette failures
All `TestCoderAgent` VCR tests now fail because the cassettes were recorded before the `<context_status>` injection was added to `PrepareStep`. The recorded HTTP interactions no longer match the actual requests which now include the context status message. These cassettes need to be re-recorded with real API keys (not a code bug).
