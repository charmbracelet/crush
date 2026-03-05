# Normalize CompactionMethod zero value in config defaults

Status: COMPLETED

## Sub tasks

1. [x] Find `setDefaults()` in `internal/config/load.go:342`
2. [x] Add `cmp.Or` normalization of `CompactionMethod` zero value → `CompactionAuto` at line 416
3. [x] Build passes
4. [x] Add `TestSetDefaults_NormalizesCompactionMethodZeroValue` test
5. [x] Add `TestSetDefaults_PreservesExplicitCompactionMethod` test
6. [x] Assert `CompactionAuto` in existing `TestConfig_setDefaults`
7. [x] All tests pass

## NOTES

Added normalization in `setDefaults()` at `internal/config/load.go:415-417`:
```go
c.Options.CompactionMethod = CompactionMethod(
    cmp.Or(string(c.Options.CompactionMethod), string(CompactionAuto)),
)
```

This ensures the zero value `""` is always normalized to `"auto"` so downstream comparisons (like `== CompactionLLM`) work consistently.
