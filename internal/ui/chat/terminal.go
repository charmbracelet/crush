package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// -----------------------------------------------------------------------------
// Terminal tool
// -----------------------------------------------------------------------------

// TerminalToolMessageItem renders a terminal tool call: which action ran,
// on which session, and what it produced.
type TerminalToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*TerminalToolMessageItem)(nil)

// NewTerminalToolMessageItem creates a new [TerminalToolMessageItem].
func NewTerminalToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	base := newBaseToolMessageItem(sty, toolCall, result, &TerminalToolRenderContext{}, canceled)
	return &TerminalToolMessageItem{baseToolMessageItem: base}
}

// TerminalToolRenderContext renders terminal tool messages.
type TerminalToolRenderContext struct{}

// RenderTool implements the [ToolRenderer] interface.
func (t *TerminalToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	if opts.IsPending() {
		return pendingTool(sty, "Terminal", opts.Anim, opts.Compact)
	}

	var params tools.TerminalParams
	_ = json.Unmarshal([]byte(opts.ToolCall.Input), &params)

	var meta tools.TerminalResponseMetadata
	if opts.HasResult() {
		_ = json.Unmarshal([]byte(opts.Result.Metadata), &meta)
	}

	sessionID := meta.SessionID
	if sessionID == "" {
		sessionID = params.SessionID
	}
	if sessionID == "" {
		sessionID = "active"
	}

	action := meta.Action
	if action == "" {
		action = params.Action
	}

	return renderTerminalTool(sty, opts, cappedWidth, terminalActionLabel(action), sessionID, terminalActionDetail(meta, params), toolResultContent(opts))
}

// terminalActionLabel renders an action name for a header.
func terminalActionLabel(action string) string {
	if action == "" {
		return "Terminal"
	}
	return strings.ToUpper(action[:1]) + action[1:]
}

// terminalActionDetail describes what the action did, shown next to the
// session ID in the header.
func terminalActionDetail(meta tools.TerminalResponseMetadata, params tools.TerminalParams) string {
	switch meta.Action {
	case "start":
		return fmt.Sprintf("run %s", params.Command)
	case "read":
		if meta.Exited {
			if meta.ExitCode != 0 {
				return fmt.Sprintf("exited with code %d", meta.ExitCode)
			}
			return "exited"
		}
		return "read screen"
	case "write":
		return fmt.Sprintf("send %s", quoteKeystrokes(params.Text))
	case "kill":
		return "terminate session"
	default:
		return ""
	}
}

// toolResultContent returns the result body when there is one.
func toolResultContent(opts *ToolRenderOpts) string {
	if !opts.HasResult() {
		return ""
	}
	return opts.Result.Content
}

// quoteKeystrokes renders control characters readably for the transcript.
func quoteKeystrokes(text string) string {
	replacer := strings.NewReplacer(
		"\r", "\\r",
		"\n", "\\n",
		"\t", "\\t",
		"\x1b", "\\e",
		"\x03", "^C",
	)
	quoted := replacer.Replace(text)
	if quoted == "" {
		quoted = "(empty)"
	}
	return fmt.Sprintf("%q", quoted)
}
