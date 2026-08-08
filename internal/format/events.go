package format

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// EventPrinter writes compact one-line event summaries to an io.Writer.
// It is designed for non-interactive mode (`crush run --show-events`)
// where tool calls, results, and assistant text are printed as single
// lines to stderr while the full assistant text streams to stdout.
//
// Styling mirrors the TUI's tool-call rendering: uppercase tool names
// with background colors per event type, and the same status icons
// (● pending, ✓ success, × error).
type EventPrinter struct {
	w           io.Writer
	seenToolIDs map[string]bool

	// Styles (initialized from charmtone palette to match the TUI).
	iconPending  lipgloss.Style
	iconSuccess  lipgloss.Style
	iconError    lipgloss.Style
	toolName     lipgloss.Style
	toolNameErr  lipgloss.Style
	toolNameOk   lipgloss.Style
	params       lipgloss.Style
	resultText   lipgloss.Style
	errorText    lipgloss.Style
	assistantTag lipgloss.Style
	assistantTxt lipgloss.Style
}

// NewEventPrinter creates an EventPrinter that writes to w.
func NewEventPrinter(w io.Writer) *EventPrinter {
	base := lipgloss.NewStyle()

	return &EventPrinter{
		w:           w,
		seenToolIDs: make(map[string]bool),

		// Icons matching the TUI: ● (pending), ✓ (success), × (error).
		iconPending: base.Foreground(charmtone.Guac).SetString("●"),
		iconSuccess: base.Foreground(charmtone.Julep).SetString("✓"),
		iconError:   base.Foreground(charmtone.Sriracha).SetString("×"),

		// Tool name: uppercase, padded, with background color per status.
		toolName: base.
			Background(charmtone.Iron).
			Foreground(charmtone.Salt).
			Bold(true).
			Padding(0, 1),
		toolNameOk: base.
			Background(charmtone.BBQ).
			Foreground(charmtone.Julep).
			Bold(true).
			Padding(0, 1),
		toolNameErr: base.
			Background(charmtone.Coral).
			Foreground(charmtone.Butter).
			Bold(true).
			Padding(0, 1),

		// Parameters / result text.
		params:     base.Foreground(charmtone.Smoke),
		resultText: base.Foreground(charmtone.Smoke),
		errorText:  base.Foreground(charmtone.Sriracha),

		// Assistant text summary.
		assistantTag: base.
			Background(charmtone.Charple).
			Foreground(charmtone.Butter).
			Bold(true).
			Padding(0, 1),
		assistantTxt: base.Foreground(charmtone.Sash),
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

	summary := briefToolInput(name, input)
	if summary != "" {
		fmt.Fprintf(
			p.w, "%s %s %s\n",
			p.iconPending.String(),
			p.toolName.Render(strings.ToUpper(name)),
			p.params.Render(summary),
		)
	} else {
		fmt.Fprintf(
			p.w, "%s %s\n",
			p.iconPending.String(),
			p.toolName.Render(strings.ToUpper(name)),
		)
	}
}

// PrintToolResult emits a one-line summary of a tool result.
func (p *EventPrinter) PrintToolResult(name, content string, isError bool) {
	summary := firstLine(content, 60)
	if isError {
		icon := p.iconError
		nm := p.toolNameErr.Render(strings.ToUpper(name))
		if summary != "" {
			fmt.Fprintf(p.w, "%s %s %s\n", icon.String(), nm, p.errorText.Render(summary))
		} else {
			fmt.Fprintf(p.w, "%s %s\n", icon.String(), nm)
		}
		return
	}
	icon := p.iconSuccess
	nm := p.toolNameOk.Render(strings.ToUpper(name))
	if summary != "" {
		fmt.Fprintf(p.w, "%s %s %s\n", icon.String(), nm, p.resultText.Render(summary))
	} else {
		fmt.Fprintf(p.w, "%s %s\n", icon.String(), nm)
	}
}

// PrintAssistantText emits the first line of an assistant text block,
// truncated to 80 characters.
func (p *EventPrinter) PrintAssistantText(text string) {
	line := firstLine(text, 80)
	if line == "" {
		return
	}
	fmt.Fprintf(
		p.w, "%s %s\n",
		p.assistantTag.Render("AI"),
		p.assistantTxt.Render(line),
	)
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
