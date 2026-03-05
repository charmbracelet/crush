# Fix `buildAgent()` to use keyed struct literal and preserve large model fallback

Status: COMPLETED

## Sub tasks

1. [x] Convert `SessionAgentOptions` init to keyed struct literal
2. [x] Always pass `large` as `LargeModel` and `small` as `SmallModel`
3. [x] Keep `primary` only for `SystemPromptPrefix` and `prompt.Build()` calls
4. [x] Build and verify no compile errors

## NOTES

Changed `coordinator.go:375-387`. The fix addresses both Copilot comments:

1. **Keyed literal**: Switched from positional to keyed struct fields, making
   the mapping explicit and resilient to field reordering.
2. **Fallback preserved**: `LargeModel` always gets the real `large` model and
   `SmallModel` always gets the real `small` model. The `primary` variable is
   now only used for selecting `SystemPromptPrefix` and building the system
   prompt — it no longer affects which model is stored in `LargeModel`.
   This means `generateTitle`'s fallback (try small → fall back to large)
   still works correctly even when the agent is configured for small model.
