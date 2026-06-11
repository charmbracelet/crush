package model

import (
	"strings"

	"github.com/charmbracelet/crush/internal/ui/completions"
	"github.com/charmbracelet/crush/internal/workspace"
)

// buildSubagentCaches projects the workspace's active subagents into the two
// shapes the UI consumes: completion items (for the @-mention picker) and a
// name set (for sendMessage rewriting). Iteration order matches the input so
// completion ordering is deterministic.
func buildSubagentCaches(active []workspace.SubagentInfo) ([]completions.SubagentCompletionValue, map[string]bool) {
	items := make([]completions.SubagentCompletionValue, len(active))
	names := make(map[string]bool, len(active))
	for i, sa := range active {
		items[i] = completions.SubagentCompletionValue{Name: sa.Name, Description: sa.Description}
		names[sa.Name] = true
	}
	return items, names
}

// rebuildSubagentCaches refreshes the @-mention completion caches from the
// workspace's current active subagents. Called when Library discovery changes.
func (m *UI) rebuildSubagentCaches() {
	m.activeSubagentItems, m.activeSubagentNames = buildSubagentCaches(m.com.Workspace.ActiveSubagents())
}

// rewriteSubagentPrompt detects the pattern `@name rest` at the start of
// content and rewrites it to a delegation instruction when name is a known
// active subagent. Returns content unchanged if the pattern doesn't match.
func rewriteSubagentPrompt(content string, activeNames map[string]bool) string {
	if !strings.HasPrefix(content, "@") {
		return content
	}
	name, prompt, ok := strings.Cut(content[1:], " ")
	if !ok {
		return content
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return content
	}
	if !activeNames[name] {
		return content
	}
	return `Use the agent tool with subagent_type="` + name + `" to handle this request: ` + prompt
}
