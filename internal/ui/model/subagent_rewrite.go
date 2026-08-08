package model

import (
	"github.com/charmbracelet/crush/internal/ui/completions"
	"github.com/charmbracelet/crush/internal/workspace"
)

// buildSubagentCaches projects the workspace's active subagents into
// completion items for the @-mention picker. Iteration order matches the
// input so completion ordering is deterministic. An @name in the sent message
// reaches the coder as typed; its prompt tells it to dispatch to that
// subagent.
func buildSubagentCaches(active []workspace.SubagentInfo) []completions.SubagentCompletionValue {
	items := make([]completions.SubagentCompletionValue, len(active))
	for i, sa := range active {
		items[i] = completions.SubagentCompletionValue{Name: sa.Name, Description: sa.Description}
	}
	return items
}
