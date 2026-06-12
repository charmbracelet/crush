package compact

import (
	"strings"
)

// FormatSummary renders SectionData into a cache-friendly summary string.
// Stable sections come first (cacheable prefix), volatile sections last.
func FormatSummary(data SectionData) string {
	stable := filterEmpty([]string{
		section("Session Goal", data.SessionGoal),
		section("User Preferences", data.UserPreferences),
		section("Files And Changes", data.FilesAndChanges),
		section("Commits", data.Commits),
	})

	volatile := filterEmpty([]string{
		section("Type Catalog", data.TypeCatalog),
		section("Outstanding Context", data.OutstandingContext),
		section("Earlier Turns", data.TurnSummaries),
	})

	allHeaders := append(stable, volatile...)

	var parts []string
	if len(allHeaders) > 0 {
		parts = append(parts, strings.Join(allHeaders, "\n\n"))
	}
	if data.BriefTranscript != "" {
		parts = append(parts, CapBrief(data.BriefTranscript))
	}

	if len(parts) == 0 {
		return ""
	}

	return wrapLongLines(strings.Join(parts, "\n\n---\n\n"), tuiSafeLineChars)
}

func section(title string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("[" + title + "]\n")
	for i, item := range items {
		b.WriteString("- " + item)
		if i < len(items)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func filterEmpty(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func wrapLongLines(text string, maxChars int) string {
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		if len(line) <= maxChars {
			out = append(out, line)
			continue
		}
		out = append(out, wrapLine(line, maxChars)...)
	}
	return strings.Join(out, "\n")
}

func wrapLine(line string, maxChars int) []string {
	if len(line) <= maxChars {
		return []string{line}
	}
	var wrapped []string
	remaining := line
	for len(remaining) > maxChars {
		splitAt := strings.LastIndexByte(remaining[:maxChars], ' ')
		if splitAt < maxChars/2 {
			splitAt = maxChars
		}
		wrapped = append(wrapped, strings.TrimRight(remaining[:splitAt], " "))
		remaining = strings.TrimLeft(remaining[splitAt:], " ")
	}
	if remaining != "" {
		wrapped = append(wrapped, remaining)
	}
	return wrapped
}
