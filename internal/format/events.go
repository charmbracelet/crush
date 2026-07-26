package format

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// EventPrinter writes compact one-line event summaries to an io.Writer.
// It is designed for non-interactive mode (`crush run --show-events`)
// where tool calls, results, and assistant text are printed as single
// lines to stderr while the full assistant text streams to stdout.
type EventPrinter struct {
	w           io.Writer
	seenToolIDs map[string]bool
}

// NewEventPrinter creates an EventPrinter that writes to w.
func NewEventPrinter(w io.Writer) *EventPrinter {
	return &EventPrinter{
		w:           w,
		seenToolIDs: make(map[string]bool),
	}
}

// PrintToolCall emits a one-line summary of a tool call.
// Only finished tool calls are printed; partial (streaming) ones
// are tracked to avoid duplicate output when the finished version
// arrives.
func (p *EventPrinter) PrintToolCall(name, id, input string, finished bool) {
	if !finished {
		return
	}
	if p.seenToolIDs[id] {
		return
	}
	p.seenToolIDs[id] = true
	fmt.Fprintf(p.w, "-> %s: %s\n", name, briefToolInput(name, input))
}

// PrintToolResult emits a one-line summary of a tool result.
func (p *EventPrinter) PrintToolResult(name, content string, isError bool) {
	if isError {
		fmt.Fprintf(p.w, "x  %s: %s\n", name, firstLine(content, 80))
		return
	}
	summary := firstLine(content, 60)
	if summary == "" {
		fmt.Fprintf(p.w, "ok %s\n", name)
		return
	}
	fmt.Fprintf(p.w, "ok %s: %s\n", name, summary)
}

// PrintAssistantText emits the first line of an assistant text block,
// truncated to 80 characters.
func (p *EventPrinter) PrintAssistantText(text string) {
	line := firstLine(text, 80)
	if line == "" {
		return
	}
	fmt.Fprintf(p.w, ">> %s\n", line)
}

// briefToolInput extracts a short human-readable summary from a tool
// call's JSON input, based on the tool name.
func briefToolInput(toolName, inputJSON string) string {
	if inputJSON == "" {
		return ""
	}

	// Try to parse as a generic map first.
	var raw map[string]any
	if err := json.Unmarshal([]byte(inputJSON), &raw); err != nil {
		return truncate(strings.TrimSpace(inputJSON), 60)
	}

	switch toolName {
	case "bash":
		if cmd, ok := raw["command"].(string); ok {
			return truncate(cmd, 60)
		}
	case "edit", "write":
		if fp, ok := raw["file_path"].(string); ok {
			return fp
		}
	case "view":
		if fp, ok := raw["file_path"].(string); ok {
			return fp
		}
	case "glob":
		if pat, ok := raw["pattern"].(string); ok {
			return pat
		}
	case "grep":
		if pat, ok := raw["pattern"].(string); ok {
			return pat
		}
	}

	// Fallback: first key/value pair.
	for k, v := range raw {
		return fmt.Sprintf("%s=%v", k, truncate(fmt.Sprint(v), 40))
	}

	return truncate(inputJSON, 60)
}

// firstLine returns the first non-empty line of s, truncated to maxLen.
func firstLine(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	return truncate(s, maxLen)
}

// truncate shortens s to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
