# Plan: Address Copilot PR Review Feedback (issue-2358-review)

## Context

PR #2365 (branch `issue-2358`) changed the `agent` tool's sub-agent to use the
small model. Copilot left two review comments that are both valid.

## Feedback Items

### 1. Use keyed struct literal for `SessionAgentOptions` (coordinator.go:376-387)

The `SessionAgentOptions` is initialized with a positional (unkeyed) struct
literal. This is brittle — if the struct fields are ever reordered, the
positional literal silently breaks. With the new `primary` variable naming
(where "LargeModel" may actually hold the small model), readability suffers
further.

**Fix**: Convert to a keyed struct literal.

### 2. LargeModel fallback lost when primary is small (coordinator.go:369-373)

When `agent.Model == SelectedModelTypeSmall`, `primary = small`, and the struct
passes `primary` as `LargeModel` and `small` as `SmallModel`. Both slots end up
holding the same small model. This means `generateTitle`'s fallback path (try
small, then fall back to large) retries with the identical model — no real
fallback.

**Fix**: Always pass the actual large model as `LargeModel` and the actual small
model as `SmallModel`. Instead, control which model is used for *primary
execution* via the prompt/provider selection only — i.e., build the system
prompt and select the `SystemPromptPrefix` using `primary`, but keep the
`LargeModel` field always pointing to the real large model.

## Changes

1. **`internal/agent/coordinator.go` — `buildAgent()`**: Switch to keyed struct
   literal. Keep `LargeModel: large` and `SmallModel: small` always. Use
   `primary` only for `SystemPromptPrefix` selection and `prompt.Build()` calls.
2. **Tests**: Run existing tests to verify no regressions.
