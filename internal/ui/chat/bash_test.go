package chat

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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

// renderTool renders a finished tool call with the given input and result
// metadata, both marshalled to JSON so native paths survive on Windows.
func renderTool(t *testing.T, r ToolRenderer, name string, input, metadata map[string]any, expanded, compact bool) string {
	t.Helper()
	sty := styles.CharmtonePantera()
	in, err := json.Marshal(input)
	require.NoError(t, err)
	var meta []byte
	if metadata != nil {
		meta, err = json.Marshal(metadata)
		require.NoError(t, err)
	}
	return ansi.Strip(r.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall:        message.ToolCall{ID: "tc", Name: name, Input: string(in), Finished: true},
		Result:          &message.ToolResult{ToolCallID: "tc", Content: "ok", Metadata: string(meta)},
		Status:          ToolStatusSuccess,
		ExpandedContent: expanded,
		Compact:         compact,
	}))
}

// TestBashToolMessageItem_CwdHint verifies that a command run outside the
// workspace shows a "cwd=" option, and that it stays quiet otherwise.
func TestBashToolMessageItem_CwdHint(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := &BashToolRenderContext{workingDir: workspace}
	render := func(input, metadata map[string]any) string {
		return renderTool(t, ctx, "bash", input, metadata, false, false)
	}

	out := render(map[string]any{"command": "go test ./...", "working_dir": filepath.Join(workspace, "internal", "ui")}, nil)
	require.Contains(t, out, "go test ./... (cwd=internal/ui)")

	out = render(map[string]any{"command": "ls", "working_dir": filepath.Join("internal", "ui")}, nil)
	require.Contains(t, out, "(cwd=internal/ui)", "relative dirs resolve against the workspace")

	out = render(
		map[string]any{"command": "ls", "working_dir": filepath.Join(workspace, "internal")},
		map[string]any{"working_directory": filepath.Join(workspace, "other")},
	)
	require.Contains(t, out, "(cwd=other)", "result metadata wins over the input")

	out = render(map[string]any{"command": "ls", "working_dir": workspace}, nil)
	require.NotContains(t, out, "cwd=")

	out = render(map[string]any{"command": "cd sub && ls", "working_dir": filepath.Join(workspace, "sub")}, nil)
	require.NotContains(t, out, "cwd=", "a leading cd already says where it runs")

	out = render(
		map[string]any{"command": "make build", "run_in_background": true},
		map[string]any{"background": true, "shell_id": "sh-1", "working_directory": filepath.Join(workspace, "sub"), "description": "build it"},
	)
	require.Contains(t, out, "PID sh-1 build it (cwd=sub)")
}

// TestCwdHint covers the path handling on its own.
func TestCwdHint(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	require.Empty(t, cwdHint(workspace, "", "ls"))
	require.Empty(t, cwdHint("", filepath.Join(workspace, "sub"), "ls"))
	require.Empty(t, cwdHint(workspace, workspace, "ls"))
	require.Equal(t, "sub", cwdHint(workspace, "sub", "ls"))
	require.Equal(t, "../elsewhere", cwdHint(workspace, filepath.Join(filepath.Dir(workspace), "elsewhere"), "ls"))
}

// TestJobOutputToolMessageItem_CwdHint verifies that job output carries the
// cwd hint of the background shell it read from.
func TestJobOutputToolMessageItem_CwdHint(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := &JobOutputToolRenderContext{workingDir: workspace}
	out := renderTool(t, ctx, "job_output",
		map[string]any{"shell_id": "sh-1"},
		map[string]any{"shell_id": "sh-1", "description": "serve", "working_directory": filepath.Join(workspace, "sub")},
		false, false)
	require.Contains(t, out, "serve (cwd=sub)")
}

// TestBashToolMessageItem_DescriptionHeader verifies that the model's
// description leads the header with the command below it.
func TestBashToolMessageItem_DescriptionHeader(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := &BashToolRenderContext{workingDir: workspace}
	render := func(input map[string]any, expanded, compact bool) []string {
		return strings.Split(renderTool(t, ctx, "bash", input, nil, expanded, compact), "\n")
	}
	commandLine := regexp.MustCompile(`^\s+echo hi\s*$`)

	longDesc := map[string]any{"command": "echo hi", "description": strings.Repeat("d", 200)}

	collapsed := render(longDesc, false, false)
	require.Contains(t, collapsed[0], "…")
	require.LessOrEqual(t, lipgloss.Width(collapsed[0]), cappedMessageWidth(120))
	require.Regexp(t, commandLine, collapsed[1], "command sits indented below the description")

	expanded := render(longDesc, true, false)
	joined := strings.Join(expanded, "\n")
	require.NotContains(t, joined, "…", "expanded description wraps instead of truncating")
	require.Regexp(t, `^\s+d+\s*$`, expanded[1])
	require.Regexp(t, commandLine, expanded[len(expanded)-3])

	multi := map[string]any{"command": "echo hi", "description": "run the echo\nthen the suite"}
	require.NotContains(t, strings.Join(render(multi, false, false), "\n"), "then the suite")
	require.Contains(t, strings.Join(render(multi, true, false), "\n"), "then the suite")

	out := render(map[string]any{"command": "sleep 5", "description": "wait", "run_in_background": true}, false, false)
	require.Contains(t, out[0], "wait (background=true)")

	out = render(map[string]any{"command": "echo hi", "working_dir": filepath.Join(workspace, "sub"), "description": "do things"}, false, false)
	require.Contains(t, out[0], "Bash do things (cwd=sub)")

	out = render(map[string]any{"command": "echo hi", "description": "do things"}, false, true)
	require.Contains(t, out[0], "echo hi", "nested calls keep the plain command header")
	require.NotContains(t, strings.Join(out, "\n"), "do things")

	out = render(map[string]any{"command": "echo hi", "description": ""}, false, false)
	require.Contains(t, out[0], "echo hi")
}
