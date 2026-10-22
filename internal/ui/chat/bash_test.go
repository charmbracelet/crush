package chat

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestBashToolMessageItem_CommandAlwaysExpanded guards the contract
// that the bash command is always rendered expanded (newlines
// preserved) regardless of the expansion state, while the output body
// remains expandable and defaults to collapsed.
func TestBashToolMessageItem_CommandAlwaysExpanded(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{
		ID:       "bash1",
		Name:     "bash",
		Input:    `{"command":"echo one\necho two\necho three"}`,
		Finished: true,
	}

	render := func(expanded bool) string {
		ctx := &BashToolRenderContext{}
		return ctx.RenderTool(&sty, 120, &ToolRenderOpts{
			ToolCall:        tc,
			Result:          &message.ToolResult{ToolCallID: "bash1", Content: "out"},
			Status:          ToolStatusSuccess,
			ExpandedContent: expanded,
		})
	}

	for _, expanded := range []bool{false, true} {
		out := ansi.Strip(render(expanded))
		require.Contains(t, out, "echo one")
		require.Contains(t, out, "echo two")
		require.Contains(t, out, "echo three")
	}
}

// TestBashToolMessageItem_LongCommandNotTruncated verifies that a
// long bash command wraps in the header instead of being truncated
// with an ellipsis, even when the output is collapsed.
func TestBashToolMessageItem_LongCommandNotTruncated(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	longCmd := "echo " + strings.Repeat("a", 200)
	tc := message.ToolCall{
		ID:       "bash3",
		Name:     "bash",
		Input:    `{"command":"` + longCmd + `"}`,
		Finished: true,
	}
	ctx := &BashToolRenderContext{}
	out := ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall:        tc,
		Result:          &message.ToolResult{ToolCallID: "bash3", Content: "out"},
		Status:          ToolStatusSuccess,
		ExpandedContent: false,
	}))

	require.NotContains(t, out, "…", "collapsed bash command must not be truncated")
	require.Contains(t, out, longCmd[strings.LastIndex(longCmd, "a")-10:], "end of command must be visible")
}

// TestBashToolMessageItem_OutputDefaultsCollapsed verifies that bash
// output is truncated when collapsed and fully shown when expanded.
func TestBashToolMessageItem_OutputDefaultsCollapsed(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{
		ID:       "bash2",
		Name:     "bash",
		Input:    `{"command":"seq 20"}`,
		Finished: true,
	}
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line%d", i))
	}
	result := &message.ToolResult{ToolCallID: "bash2", Content: strings.Join(lines, "\n")}

	item := NewBashToolMessageItem(&sty, tc, result, false, "")
	bash, ok := item.(*BashToolMessageItem)
	require.True(t, ok, "NewBashToolMessageItem must return a *BashToolMessageItem")
	require.False(t, bash.expandedContent, "bash items must default to collapsed")

	renderCollapsed := ansi.Strip(bash.RawRender(120))
	require.Contains(t, renderCollapsed, "line10")
	require.NotContains(t, renderCollapsed, "line20")
	require.Contains(t, renderCollapsed, "… (10 lines hidden)")

	require.True(t, bash.ToggleExpanded(), "first toggle should report expanded")
	require.Contains(t, ansi.Strip(bash.RawRender(120)), "line20")
}

// TestToolOutputPlainContent_SingleHiddenLineShown verifies that
// collapsing never hides just one line: content one line over the
// collapsed limit is shown in full, while more over that truncates.
func TestToolOutputPlainContent_SingleHiddenLineShown(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	var lines []string
	for i := 1; i <= 11; i++ {
		lines = append(lines, fmt.Sprintf("line%d", i))
	}
	out := ansi.Strip(toolOutputPlainContent(&sty, strings.Join(lines, "\n"), 80, false))
	require.Contains(t, out, "line11")
	require.NotContains(t, out, "hidden")

	lines = append(lines, "line12")
	out = ansi.Strip(toolOutputPlainContent(&sty, strings.Join(lines, "\n"), 80, false))
	require.NotContains(t, out, "line12")
	require.Contains(t, out, "2 lines hidden")
}

// TestBashToolMessageItem_WorkingDirChip verifies that the working
// directory is shown as a trailing "cd <relpath>" chip when the command
// ran outside the workspace directory, and omitted when it ran in the
// workspace itself.
func TestBashToolMessageItem_WorkingDirChip(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &BashToolRenderContext{workingDir: "/workspace"}
	render := func(input, metadata string) string {
		tc := message.ToolCall{
			ID:       "bash-wd",
			Name:     "bash",
			Input:    input,
			Finished: true,
		}
		result := &message.ToolResult{ToolCallID: "bash-wd", Content: "ok", Metadata: metadata}
		return ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
			ToolCall: tc,
			Result:   result,
			Status:   ToolStatusSuccess,
		}))
	}

	out := render(`{"command":"go test ./...","working_dir":"/workspace/internal/ui"}`, "")
	require.Contains(t, out, "Bash cd internal/ui/ ", "chip must sit right after the tool name")
	require.Contains(t, out, "go test ./...")
	require.NotContains(t, out, "cd=")

	// Relative working dirs are resolved against the workspace.
	out = render(`{"command":"ls","working_dir":"internal/ui"}`, "")
	require.Contains(t, out, "cd internal/ui/")

	// The result metadata wins over the tool call input.
	out = render(`{"command":"ls","working_dir":"/workspace/internal/ui"}`, `{"working_directory":"/workspace/other"}`)
	require.Contains(t, out, "cd other/")
	require.NotContains(t, out, "cd internal/ui")

	// No chip when the command ran in the workspace itself.
	out = render(`{"command":"ls","working_dir":"/workspace"}`, "")
	require.NotContains(t, out, "cd ")
}

// TestBashToolMessageItem_DescriptionHeader verifies that the
// model-provided description leads the header: truncated to one line
// when collapsed, wrapped when expanded, with the command indented
// below.
func TestBashToolMessageItem_DescriptionHeader(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	longDesc := strings.Repeat("d", 200)
	tc := message.ToolCall{
		ID:       "bash-desc-h",
		Name:     "bash",
		Input:    `{"command":"echo hi","description":"` + longDesc + `"}`,
		Finished: true,
	}
	result := &message.ToolResult{ToolCallID: "bash-desc-h", Content: "hi"}
	ctx := &BashToolRenderContext{}

	opts := func(expanded bool) *ToolRenderOpts {
		return &ToolRenderOpts{
			ToolCall:        tc,
			Result:          result,
			Status:          ToolStatusSuccess,
			ExpandedContent: expanded,
		}
	}

	collapsed := strings.Split(ansi.Strip(ctx.RenderTool(&sty, 120, opts(false))), "\n")
	require.GreaterOrEqual(t, len(collapsed), 4)
	require.Contains(t, collapsed[0], "…")
	require.Less(t, lipglossWidth(collapsed[0]), 121)
	require.Contains(t, collapsed[1], "echo hi")
	require.True(t, strings.HasPrefix(collapsed[1], "  "), "command must be indented under the tool name")
	require.Contains(t, strings.Join(collapsed, "\n"), "hi")

	expanded := strings.Split(ansi.Strip(ctx.RenderTool(&sty, 120, opts(true))), "\n")
	require.GreaterOrEqual(t, len(expanded), 5)
	require.NotContains(t, expanded[0], "…", "expanded description must wrap, not truncate")
	require.NotContains(t, expanded[1], "…", "expanded description must wrap, not truncate")
	require.True(t, strings.HasPrefix(expanded[1], strings.Repeat(" ", 7)+"d"), "wrapped description must be indented under the tool name")
	require.Contains(t, expanded[2], "echo hi")
	require.Contains(t, strings.Join(expanded, "\n"), "hi")
}

// lipglossWidth is a small helper so the test does not import lipgloss
// directly for a single width check.
func lipglossWidth(s string) int {
	return ansi.StringWidth(s)
}
