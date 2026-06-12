package compact

import (
	"strings"
)

// headerNames defines the canonical section ordering.
var headerNames = []string{
	"Session Goal",
	"User Preferences",
	"Files And Changes",
	"Commits",
	"Type Catalog",
	"Outstanding Context",
	"Earlier Turns",
}

const separator = "\n\n---\n\n"

// MergePrevious merges a previous summary with a fresh one using
// per-section policies. Sticky sections accumulate, volatile sections
// are replaced, union sections dedup.
func MergePrevious(prev, fresh string) string {
	var headers []string
	for _, h := range headerNames {
		freshSec := sectionOf(fresh, h)
		prevSec := sectionOf(prev, h)
		merged := mergeHeaderSection(h, prevSec, freshSec)
		if merged != "" {
			headers = append(headers, merged)
		}
	}

	prevBrief := briefOf(prev)
	freshBrief := briefOf(fresh)
	mergedBrief := mergeBriefTranscript(prevBrief, freshBrief)

	var parts []string
	if len(headers) > 0 {
		parts = append(parts, strings.Join(headers, "\n\n"))
	}
	if mergedBrief != "" {
		parts = append(parts, CapBrief(mergedBrief))
	}

	return strings.Join(parts, separator)
}

func mergeHeaderSection(header, prev, fresh string) string {
	switch header {
	case "Outstanding Context", "Type Catalog":
		return fresh // volatile
	case "Files And Changes":
		return mergeFileLines(prev, fresh) // union
	default:
		return mergeStickySection(header, prev, fresh)
	}
}

func mergeStickySection(header, prev, fresh string) string {
	if prev == "" && fresh == "" {
		return ""
	}
	isClean := func(l string) bool {
		return strings.HasPrefix(l, "- ")
	}
	prevLines := filterLines(prev, isClean)
	freshLines := filterLines(fresh, isClean)

	// Dedup.
	seen := make(map[string]bool)
	var contentLines []string
	for _, l := range prevLines {
		if !seen[l] {
			seen[l] = true
			contentLines = append(contentLines, l)
		}
	}
	for _, l := range freshLines {
		if !seen[l] {
			seen[l] = true
			contentLines = append(contentLines, l)
		}
	}

	cap := 15
	switch header {
	case "Session Goal":
		cap = 8
	case "Commits":
		cap = 8
	case "Earlier Turns":
		cap = 15
	}

	if len(contentLines) > cap {
		contentLines = contentLines[len(contentLines)-cap:]
	}
	if len(contentLines) == 0 {
		return ""
	}
	return "[" + header + "]\n" + strings.Join(contentLines, "\n")
}

func mergeFileLines(prev, fresh string) string {
	categories := []string{"Modified", "Created", "Read"}
	merged := make(map[string]map[string]bool)
	for _, cat := range categories {
		merged[cat] = make(map[string]bool)
	}

	for _, text := range []string{prev, fresh} {
		if text == "" {
			continue
		}
		for line := range strings.SplitSeq(text, "\n") {
			for _, cat := range categories {
				prefix := "- " + cat + ": "
				if !strings.HasPrefix(line, prefix) {
					continue
				}
				rest := line[len(prefix):]
				// Strip breadcrumb markers.
				rest = strings.ReplaceAll(rest, ", +recall: ", ", ")
				for p := range strings.SplitSeq(rest, ",") {
					p = strings.TrimSpace(p)
					if p != "" && !strings.HasPrefix(p, "+recall:") {
						merged[cat][p] = true
					}
				}
			}
		}
	}

	// Dedup: if already modified, drop from created.
	for p := range merged["Modified"] {
		delete(merged["Created"], p)
	}

	capSet := func(m map[string]bool, limit int) string {
		arr := make([]string, 0, len(m))
		for k := range m {
			arr = append(arr, k)
		}
		if len(arr) <= limit {
			return strings.Join(arr, ", ")
		}
		kept := arr[:limit]
		omitted := arr[limit:]
		return strings.Join(kept, ", ") + ", +recall: " + strings.Join(omitted, ", ")
	}

	var lines []string
	if len(merged["Modified"]) > 0 {
		lines = append(lines, "- Modified: "+capSet(merged["Modified"], 10))
	}
	if len(merged["Created"]) > 0 {
		lines = append(lines, "- Created: "+capSet(merged["Created"], 10))
	}
	if len(merged["Read"]) > 0 {
		lines = append(lines, "- Read: "+capSet(merged["Read"], 10))
	}
	if len(lines) == 0 {
		return ""
	}
	return "[Files And Changes]\n" + strings.Join(lines, "\n")
}

func mergeBriefTranscript(prev, fresh string) string {
	if prev == "" {
		return fresh
	}
	if fresh == "" {
		return prev
	}
	return prev + "\n\n" + fresh
}

func sectionOf(text, header string) string {
	tag := "[" + header + "]"
	start := strings.Index(text, tag)
	if start < 0 {
		return ""
	}
	after := text[start:]
	end := len(after)
	for _, h := range headerNames {
		if h == header {
			continue
		}
		idx := strings.Index(after, "["+h+"]")
		if idx > 0 && idx < end {
			end = idx
		}
	}
	sepIdx := strings.Index(after, separator)
	if sepIdx > 0 && sepIdx < end {
		end = sepIdx
	}
	return strings.TrimSpace(after[:end])
}

func briefOf(text string) string {
	_, after, ok := strings.Cut(text, separator)
	if !ok {
		return ""
	}
	return strings.TrimSpace(after)
}

func filterLines(text string, pred func(string) bool) []string {
	var out []string
	for l := range strings.SplitSeq(text, "\n") {
		if pred(l) {
			out = append(out, l)
		}
	}
	return out
}
