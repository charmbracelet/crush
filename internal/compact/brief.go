package compact

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	truncateUser      = 256
	truncateAssistant = 200
	briefMaxLines     = 120
	tuiSafeLineChars  = 120
	toolCallsPerTurn  = 8
)

// Self-talk prefixes stripped from assistant messages.
var selfTalkPrefixRe = regexp.MustCompile(`(?i)^\s*(?:hmm|wait|actually|oh|okay|ok|well|so)[,.!\s-]+`)

// Tool summary field mappings for one-liner generation.
var toolSummaryFields = map[string]string{
	"view": "file_path", "edit": "file_path", "write": "file_path",
	"multiedit": "file_path", "grep": "pattern", "glob": "pattern",
}

// BriefSection is a labeled group of transcript lines.
type BriefSection struct {
	Header string
	Lines  []string
}

// BuildBriefSections creates structured transcript sections from blocks.
func BuildBriefSections(blocks []NormalizedBlock) []BriefSection {
	var sections []BriefSection
	lastHeader := ""

	push := func(header, line string) {
		if header == lastHeader && len(sections) > 0 {
			sections[len(sections)-1].Lines = append(sections[len(sections)-1].Lines, line)
			return
		}
		sections = append(sections, BriefSection{Header: header, Lines: []string{line}})
		lastHeader = header
	}

	for _, b := range blocks {
		switch b.Kind {
		case BlockUser:
			text := truncateContent(b.Text, truncateUser)
			if text != "" {
				push("[user]", text)
			}
			lastHeader = "[user]"

		case BlockBash:
			cmd := compressBash(b.Command)
			if cmd != "" {
				push("[user]", "$ "+cmd)
			}
			lastHeader = "[user]"

		case BlockAssistant:
			raw := b.Text
			for range 2 {
				stripped := selfTalkPrefixRe.ReplaceAllString(raw, "")
				if stripped == raw {
					break
				}
				raw = stripped
			}
			text := truncateContent(raw, truncateAssistant)
			if text != "" {
				push("[assistant]", text)
			}

		case BlockToolCall:
			if b.Name == "" {
				continue
			}
			summary := toolOneLiner(b.Name, b.Args)
			push("[assistant]", summary)

		case BlockToolResult:
			if b.IsError {
				body := FirstLine(b.ResultText, 150)
				if body != "" && body != "(no output)" {
					header := "[tool_error] " + b.Name
					push(header, body)
					lastHeader = header
				}
			}
		}
	}

	// Cap tool calls per assistant turn.
	capToolCalls(sections)

	return sections
}

// StringifyBrief renders brief sections into text.
func StringifyBrief(sections []BriefSection) string {
	var out []string
	for i, sec := range sections {
		if i > 0 {
			prevIsTools := sections[i-1].Header == "[assistant]" && allToolLines(sections[i-1].Lines)
			curIsTools := sec.Header == "[assistant]" && allToolLines(sec.Lines)
			if !(prevIsTools && curIsTools) {
				out = append(out, "")
			}
		}
		out = append(out, sec.Header)
		out = append(out, sec.Lines...)
	}
	return strings.Join(out, "\n")
}

// CapBrief limits the transcript to briefMaxLines, preserving section headers.
func CapBrief(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) <= briefMaxLines {
		return text
	}
	omitted := len(lines) - briefMaxLines
	kept := lines[len(lines)-briefMaxLines:]
	// Find first section header to avoid cutting mid-section.
	firstHeader := -1
	for i, l := range kept {
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			firstHeader = i
			break
		}
	}
	if firstHeader > 0 {
		kept = kept[firstHeader:]
	}
	crumb := "...(" + strconv.Itoa(omitted) + " earlier lines omitted)"
	return crumb + "\n\n" + strings.Join(kept, "\n")
}

// TurnInfo holds a one-line summary for a conversational turn.
type TurnInfo struct {
	Summary string
}

// IdentifyTurns groups blocks into conversational turns and produces
// one-liner summaries for each. This is the heaviest compression zone.
func IdentifyTurns(blocks []NormalizedBlock) []TurnInfo {
	var turns []TurnInfo
	var currentUserText string
	var toolActions []string
	hasUser := false

	flush := func() {
		if !hasUser && len(toolActions) == 0 {
			return
		}
		turns = append(turns, TurnInfo{
			Summary: synthesizeTurnSummary(currentUserText, toolActions),
		})
		currentUserText = ""
		toolActions = toolActions[:0]
		hasUser = false
	}

	for _, b := range blocks {
		switch b.Kind {
		case BlockUser:
			flush()
			currentUserText = truncateContentWords(b.Text, 12)
			hasUser = true
		case BlockBash:
			flush()
			currentUserText = "$ " + compressBash(b.Command)
			hasUser = true
		case BlockToolCall:
			if b.Name == "" {
				continue
			}
			path := extractPathFromArgs(b.Args)
			isWrite := fileWriteTools[b.Name]
			if isWrite && path != "" {
				toolActions = append(toolActions, "edited "+shortenPath(path))
			} else if path != "" {
				toolActions = append(toolActions, b.Name+" "+shortenPath(path))
			} else if b.Name == "bash" {
				cmd := compressBash(b.Args["command"])
				if cmd != "" {
					toolActions = append(toolActions, "ran "+cmd)
				}
			} else {
				toolActions = append(toolActions, b.Name)
			}
		}
	}
	flush()
	return turns
}

func synthesizeTurnSummary(userText string, toolActions []string) string {
	var parts []string
	if userText != "" && len(userText) > 3 {
		parts = append(parts, Clip(userText, 50))
	}
	unique := dedupStrings(toolActions)
	if len(unique) > 5 {
		unique = unique[:5]
	}
	if len(unique) > 0 {
		parts = append(parts, strings.Join(unique, ", "))
	}
	if len(parts) == 0 {
		return "(no actions)"
	}
	return strings.Join(parts, " → ")
}

func toolOneLiner(name string, args map[string]string) string {
	field := toolSummaryFields[name]
	if field != "" {
		if v := args[field]; v != "" {
			return "* " + name + " \"" + v + "\""
		}
	}
	path := extractPathFromArgs(args)
	if path != "" {
		return "* " + name + " \"" + path + "\""
	}
	if name == "bash" {
		cmd := compressBash(args["command"])
		return "* bash \"" + cmd + "\""
	}
	if q := args["query"]; q != "" {
		return "* " + name + " \"" + Clip(q, 60) + "\""
	}
	return "* " + name
}

func compressBash(raw string) string {
	lines := strings.Split(raw, "\n")
	cmd := ""
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" {
			cmd = t
			break
		}
	}
	if cmd == "" {
		return raw
	}
	// Strip cd prefix.
	if strings.HasPrefix(cmd, "cd ") {
		if idx := strings.Index(cmd, " && "); idx >= 0 {
			cmd = cmd[idx+4:]
		}
	}
	const bashCap = 120
	if len(cmd) > bashCap {
		return cmd[:bashCap-3] + "..."
	}
	return cmd
}

func truncateContent(text string, max int) string {
	flat := strings.Join(strings.Fields(text), " ")
	if len(flat) <= max {
		return flat
	}
	return flat[:max] + "...(truncated)"
}

func truncateContentWords(text string, wordLimit int) string {
	words := strings.Fields(text)
	if len(words) <= wordLimit {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:wordLimit], " ") + "..."
}

func shortenPath(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return p
}

func extractPathFromArgs(args map[string]string) string {
	if p := args["file_path"]; p != "" {
		return p
	}
	if p := args["path"]; p != "" {
		return p
	}
	return ""
}

func capToolCalls(sections []BriefSection) {
	for i := range sections {
		if sections[i].Header != "[assistant]" {
			continue
		}
		var toolIdxs []int
		for j, l := range sections[i].Lines {
			if strings.HasPrefix(l, "* ") {
				toolIdxs = append(toolIdxs, j)
			}
		}
		if len(toolIdxs) <= toolCallsPerTurn {
			continue
		}
		dropCount := len(toolIdxs) - toolCallsPerTurn
		dropSet := make(map[int]bool)
		for _, idx := range toolIdxs[:dropCount] {
			dropSet[idx] = true
		}
		var next []string
		inserted := false
		for j, l := range sections[i].Lines {
			if dropSet[j] {
				continue
			}
			if !inserted && j == toolIdxs[dropCount] {
				next = append(next, "* ("+strconv.Itoa(dropCount)+" earlier tool-call entries omitted)")
				inserted = true
			}
			next = append(next, l)
		}
		sections[i].Lines = next
	}
}

func allToolLines(lines []string) bool {
	for _, l := range lines {
		if !strings.HasPrefix(l, "* ") {
			return false
		}
	}
	return true
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
