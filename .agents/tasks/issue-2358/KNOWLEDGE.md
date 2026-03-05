# Knowledge: issue-2358

## Post-PR Review Fix (issue-2358-review)

After the initial PR (#2365) was submitted, Copilot flagged two valid issues
in `buildAgent()` (`internal/agent/coordinator.go`):

1. **Use keyed struct literals for `SessionAgentOptions`.**
   The positional literal was brittle and hard to read — especially since the
   `primary` variable (which could be the small model) was being passed into a
   field named `LargeModel`.

2. **Passing `primary` as `LargeModel` broke the fallback in `generateTitle`.**
   When `agent.Model == SelectedModelTypeSmall`, both `LargeModel` and
   `SmallModel` ended up holding the same small model. This meant
   `generateTitle`'s retry path (try small → fall back to large) would just
   retry with the identical model.

**Fix**: Always pass the real `large` and `small` models into `LargeModel` and
`SmallModel` respectively. Use `primary` only for selecting the
`SystemPromptPrefix` and building the system prompt via `prompt.Build()`. This
keeps the cost/speed benefit of routing sub-agent execution through the small
model while preserving the large-model fallback for auxiliary tasks like title
generation.
