package agent

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/memory"
)

const (
	autoRecallMemoryLimit      = 3
	autoRecallHistoryLimit     = 3
	autoRecallSectionCharLimit = 600
)

func buildAutoRecall(historySvc history.Service, memorySvc memory.Service) func(context.Context, string, string) string {
	if historySvc == nil && memorySvc == nil {
		return nil
	}
	return func(ctx context.Context, sessionID, prompt string) string {
		return buildAutoRecallBlock(ctx, historySvc, memorySvc, sessionID, prompt)
	}
}

func buildAutoRecallBlock(ctx context.Context, historySvc history.Service, memorySvc memory.Service, sessionID, prompt string) string {
	query := strings.TrimSpace(prompt)
	if query == "" {
		return ""
	}

	sections := make([]string, 0, 2)
	if memorySvc != nil {
		scope, includeMemory := autoRecallMemoryScope(ctx)
		if includeMemory {
			search := memory.SearchParams{Query: query, Limit: autoRecallMemoryLimit}
			if scope != "" {
				search.Scope = scope
			}
			entries, err := memorySvc.Search(ctx, search)
			if err == nil && len(entries) > 0 {
				sections = append(sections, formatAutoRecallMemory(entries))
			}
		}
	}
	if historySvc != nil {
		results, err := historySvc.SearchMessages(ctx, history.SearchParams{Query: query, SessionID: sessionID, Limit: autoRecallHistoryLimit})
		if err == nil && len(results) > 0 {
			sections = append(sections, formatAutoRecallHistory(results))
		}
	}
	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

func formatAutoRecallMemory(entries []memory.Entry) string {
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, "Relevant long-term memory:")
	for _, entry := range entries {
		value := truncateRecallText(strings.TrimSpace(entry.Value), autoRecallSectionCharLimit)
		lines = append(lines, fmt.Sprintf("- %s: %s", formatAutoRecallMemoryLabel(entry), value))
	}
	return strings.Join(lines, "\n")
}

func formatAutoRecallMemoryLabel(entry memory.Entry) string {
	label := strings.TrimSpace(entry.Key)
	qualifiers := make([]string, 0, 2)

	switch {
	case entry.Category != "" && entry.Type != "":
		qualifiers = append(qualifiers, fmt.Sprintf("%s/%s", strings.TrimSpace(entry.Category), strings.TrimSpace(entry.Type)))
	case entry.Category != "":
		qualifiers = append(qualifiers, strings.TrimSpace(entry.Category))
	case entry.Type != "":
		qualifiers = append(qualifiers, strings.TrimSpace(entry.Type))
	}

	if len(entry.Tags) > 0 {
		tags := make([]string, 0, len(entry.Tags))
		for _, tag := range entry.Tags {
			trimmed := strings.TrimSpace(tag)
			if trimmed == "" {
				continue
			}
			tags = append(tags, "#"+trimmed)
		}
		if len(tags) > 0 {
			qualifiers = append(qualifiers, strings.Join(tags, " "))
		}
	}

	if len(qualifiers) == 0 {
		return label
	}
	return fmt.Sprintf("%s (%s)", label, strings.Join(qualifiers, "; "))
}

func formatAutoRecallHistory(results []history.MessageSearchResult) string {
	lines := make([]string, 0, len(results)+1)
	lines = append(lines, "Relevant session history:")
	for _, result := range results {
		text := truncateRecallText(strings.TrimSpace(result.Text), autoRecallSectionCharLimit)
		lines = append(lines, fmt.Sprintf("- [%s] %s", result.Role, text))
	}
	return strings.Join(lines, "\n")
}

func autoRecallMemoryScope(ctx context.Context) (string, bool) {
	memoryPolicy := strings.ToLower(strings.TrimSpace(tools.GetAgentMemoryFromContext(ctx)))
	isolationPolicy := strings.ToLower(strings.TrimSpace(tools.GetAgentIsolationFromContext(ctx)))

	switch memoryPolicy {
	case "ephemeral":
		return "", false
	case "isolated", "session":
		return "session", true
	case "project":
		return "project", true
	}

	switch isolationPolicy {
	case "session", "process":
		return "session", true
	case "workspace":
		return "project", true
	}

	return "", true
}

func truncateRecallText(text string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "…"
}
