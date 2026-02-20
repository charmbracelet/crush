# Configuration Development Guide

## Merge Rules

When adding new fields to config structs (`Config`, `Options`, `MCPConfig`, `LSPConfig`, `TUIOptions`, `Tools`, `ProviderConfig`), you **must** update the corresponding `merge()` method in `config.go` and add test cases to `merge_test.go`.

### Merge Behavior Patterns

Each field type has a specific merge strategy:

| Type | Strategy | Example |
|------|----------|---------|
| **Booleans** | `true` if any config has `true` | `Disabled`, `Debug`, `Progress` |
| **Strings** | Later value replaces earlier | `Model`, `InitializeAs`, `TrailerStyle` |
| **Slices (paths/tools)** | Appended, sorted, deduped | `SkillsPaths`, `DisabledTools` |
| **Slices (args)** | Later replaces earlier entirely | `Args` in LSPConfig |
| **Maps** | Merged, later values overwrite keys | `Env`, `Headers`, `Options` |
| **Timeouts** | Max value wins | `Timeout` in MCPConfig/LSPConfig |
| **Pointers** | Later non-nil replaces earlier | `MaxTokens`, `Temperature` |
| **Structs** | Call sub-struct's `merge()` method | `TUI`, `Tools` |

### Adding a New Config Field

1. Add the field to the appropriate struct in `config.go`
2. Add merge logic to the struct's `merge()` method following the patterns above
3. Add a test case in `merge_test.go` verifying the merge behavior
4. Run `go test ./internal/config/... -v -run TestConfigMerging` to verify
