package agent

import (
	"sort"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
)

// collectDeferredToolHints collects deferred tool entries for prompt inclusion.
// Unlike the previous implementation, this does not limit the number of entries
// since we only output tool names (minimal token cost, similar to Claude Code).
func collectDeferredToolHints(entries map[string]tools.RegistryEntry, disabledSet map[string]struct{}) []tools.RegistryEntry {
	if len(entries) == 0 {
		return nil
	}

	hints := make([]tools.RegistryEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.Metadata.IsDeferred() {
			continue
		}
		if entry.Exposed {
			continue
		}
		if _, disabled := disabledSet[entry.Name]; disabled {
			continue
		}
		hints = append(hints, entry)
	}

	sort.Slice(hints, func(i, j int) bool {
		return hints[i].Name < hints[j].Name
	})
	return hints
}

// appendDeferredToolsPromptSection appends a section listing deferred tool names.
// Following Claude Code's approach, we only list tool names (not descriptions/hints)
// to minimize token usage while keeping all tools discoverable via tool_search.
func appendDeferredToolsPromptSection(basePrompt string, deferredEntries []tools.RegistryEntry) string {
	if len(deferredEntries) == 0 {
		return basePrompt
	}

	// Build tool name list (only names, minimal token cost)
	names := make([]string, len(deferredEntries))
	for i, entry := range deferredEntries {
		names[i] = entry.Name
	}

	lines := []string{
		"<available_deferred_tools>",
		"The following deferred tools are available. Use tool_search with a query to get their full schemas before calling them.",
		"",
		strings.Join(names, ", "),
		"</available_deferred_tools>",
	}

	section := strings.Join(lines, "\n")
	trimmedBase := strings.TrimSpace(basePrompt)
	if trimmedBase == "" {
		return section
	}
	return trimmedBase + "\n\n" + section
}
