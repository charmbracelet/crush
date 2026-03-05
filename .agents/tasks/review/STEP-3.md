# Clamp usedPct to 100 and add stale-token comment

Status: COMPLETED

## Sub tasks

1. [x] Add doc comment about stale token counts (one-step lag) to `contextStatusMessage()`
2. [x] Use `max()` builtin for remaining clamp (modernize hint)
3. [x] Clamp `usedPct` to 100 with `min()`
4. [x] Update overflow test to expect `used_pct:100` instead of `125`
5. [x] Run tests — all pass

## NOTES

- `agent.go:932-946`: Rewrote function header comment and clamping logic.
- `context_status_test.go:97`: Updated expected value from 125 to 100.
- The `max`/`min` builtins resolve the gopls modernize hint too.
