package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func renderTerminalAction(t *testing.T, action, text string, result *message.ToolResult) string {
	t.Helper()

	sty := styles.CharmtonePantera()
	input, err := json.Marshal(tools.TerminalParams{Action: action, Text: text})
	require.NoError(t, err)

	tc := message.ToolCall{
		ID:       "t1",
		Name:     "terminal",
		Input:    string(input),
		Finished: true,
	}
	ctx := &TerminalToolRenderContext{}
	return ctx.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: tc,
		Result:   result,
		Status:   ToolStatusSuccess,
	})
}

func terminalResult(t *testing.T, action, content string) *message.ToolResult {
	t.Helper()

	meta, err := json.Marshal(tools.TerminalResponseMetadata{Action: action, SessionID: "abc123"})
	require.NoError(t, err)
	return &message.ToolResult{
		ToolCallID: "t1",
		Content:    content,
		Metadata:   string(meta),
	}
}

// TestTerminalToolHidesScreenBodies guards that read and write results do
// not dump the screen: the docked terminal is already showing it live.
func TestTerminalToolHidesScreenBodies(t *testing.T) {
	t.Parallel()

	screen := "<screen>\nsome screen\n</screen>\nSession abc123: running"
	for _, action := range []string{"read", "write"} {
		out := ansi.Strip(renderTerminalAction(t, action, "y", terminalResult(t, action, screen)))
		require.Contains(t, out, "Terminal", "the header row is present")
		require.NotContains(t, out, "some screen", "%s bodies duplicate the live panel", action)
	}
}

// TestTerminalToolShowsSessionRecordBodies keeps the transcript bodies for
// start and kill: those are the record of the session once it is gone.
func TestTerminalToolShowsSessionRecordBodies(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"start", "kill"} {
		out := ansi.Strip(renderTerminalAction(t, action, "", terminalResult(t, action, "transcript line")))
		require.Contains(t, out, "transcript line", "%s keeps its body", action)
	}
}

// TestTerminalToolErrorsKeepTheirBody makes sure a failed read still
// explains itself even though successful ones stay quiet.
func TestTerminalToolErrorsKeepTheirBody(t *testing.T) {
	t.Parallel()

	result := terminalResult(t, "read", "session not found")
	result.IsError = true
	out := ansi.Strip(renderTerminalAction(t, "read", "", result))
	require.Contains(t, out, "session not found")
}

// TestTerminalToolColoredHeader checks the write header is colored: the
// action chip and the keystrokes the agent sent carry theme colors.
func TestTerminalToolColoredHeader(t *testing.T) {
	t.Parallel()

	styled := renderTerminalAction(t, "write", "y", terminalResult(t, "write", ""))
	require.True(t, strings.Contains(styled, "\x1b[38;2;"), "the action chip carries a theme color")
	require.True(t, strings.Contains(styled, "send \x1b["), "the keystrokes are accented")
}
