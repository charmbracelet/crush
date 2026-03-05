# Add "Select compaction method" menu item to the main Commands menu in the TUI

Status: COMPLETED

## Sub tasks

1. [x] Add `ActionSelectCompactionMethod` action type to `dialog/actions.go`
2. [x] Create `dialog/compaction.go` with full dialog implementation (modeled after `reasoning.go`)
3. [x] Add "Select Compaction Method" menu item in `dialog/commands.go` `defaultCommands()`
4. [x] Add `CompactionID` case in dialog dispatch in `model/ui.go`
5. [x] Add `openCompactionDialog()` method in `model/ui.go`
6. [x] Wire `ActionSelectCompactionMethod` handler in `model/ui.go` action type-switch
7. [x] Build and test

## NOTES

- Created `internal/ui/dialog/compaction.go` — full dialog with `CompactionID = "compaction"`, two items: "Auto-compaction" and "LLM/User-driven compaction".
- Action handler updates `cfg.Options.CompactionMethod` and calls `UpdateAgentModel` to rebuild the agent with new flags.
- Menu item placed after the sidebar toggle, always visible (not conditional on model type).
- All packages build clean, tests pass.
