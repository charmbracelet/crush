# Plan: Persist Compaction Method Setting

## Approach

Follow the exact same pattern as `SetCompactMode()`:

1. Add a `SetCompactionMethod()` method to `Config` that:
   - Mutates `Options.CompactionMethod` in memory.
   - Calls `SetConfigField("options.compaction_method", method)` to persist.
2. Update the UI handler in `ui.go` to call `SetCompactionMethod()` instead
   of directly assigning the field.
3. Add a test for `SetCompactionMethod()`.
4. Verify existing tests still pass.
