package chat

import (
	"fmt"
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

	// The background start header carries the chip too.
	out = render(`{"command":"make build","run_in_background":true}`,
		`{"background":true,"shell_id":"sh-1","working_directory":"/workspace/sub","description":"build it"}`)
	require.Contains(t, out, "PID sh-1")
	require.Contains(t, out, "cd sub/")
}

// TestRenderDirChip verifies the chip logic on its own: when it applies,
// how relative and outside paths resolve, and how it truncates.
func TestRenderDirChip(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	// No chip without a dir, without a workspace, in the workspace itself,
	// or at a width that leaves the command no room.
	require.Empty(t, renderDirChip(&sty, "/workspace", "", 80))
	require.Empty(t, renderDirChip(&sty, "", "/workspace/sub", 80))
	require.Empty(t, renderDirChip(&sty, "/workspace", "/workspace", 80))
	require.Empty(t, renderDirChip(&sty, "/workspace", "/workspace/sub", 15))

	// Relative dirs resolve against the workspace.
	require.Equal(t, "cd sub/", ansi.Strip(renderDirChip(&sty, "/workspace", "sub", 80)))

	// Dirs outside the workspace keep the leading ../.
	require.Equal(t, "cd ../elsewhere/", ansi.Strip(renderDirChip(&sty, "/workspace", "/elsewhere", 80)))

	// Deep paths truncate to half the width.
	deep := strings.Repeat("aaaaaaaaaa/", 12)
	chip := ansi.Strip(renderDirChip(&sty, "/workspace", "/workspace/"+deep, 40))
	require.Contains(t, chip, "…")
	require.NotContains(t, chip, deep)
}

// TestJobOutputToolMessageItem_WorkingDirChip verifies that the job
// output header carries the working directory chip of the background
// shell it read from.
func TestJobOutputToolMessageItem_WorkingDirChip(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &JobOutputToolRenderContext{workingDir: "/workspace"}
	tc := message.ToolCall{
		ID:       "job-out",
		Name:     "job_output",
		Input:    `{"shell_id":"sh-1"}`,
		Finished: true,
	}
	result := &message.ToolResult{
		ToolCallID: "job-out",
		Content:    "out",
		Metadata:   `{"shell_id":"sh-1","working_directory":"/workspace/sub"}`,
	}
	out := ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
		ToolCall: tc,
		Result:   result,
		Status:   ToolStatusSuccess,
	}))
	require.Contains(t, out, "cd sub/")
}

// TestBashToolMessageItem_DescriptionHeader verifies that the
// model-provided description leads the header: truncated to one line
// when collapsed, wrapped when expanded, with the command indented below.
func TestBashToolMessageItem_DescriptionHeader(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	ctx := &BashToolRenderContext{workingDir: "/workspace"}
	// The icon and tool name the description and command indent under.
	indent := strings.Repeat(" ", lipgloss.Width(toolBasePrefix(&sty, ToolStatusSuccess, "Bash", nil)))
	capped := cappedMessageWidth(120)

	render := func(input string, expanded, compact bool) string {
		tc := message.ToolCall{
			ID:       "bash-desc-h",
			Name:     "bash",
			Input:    input,
			Finished: true,
		}
		return ansi.Strip(ctx.RenderTool(&sty, 120, &ToolRenderOpts{
			ToolCall:        tc,
			Result:          &message.ToolResult{ToolCallID: "bash-desc-h", Content: "hi"},
			Status:          ToolStatusSuccess,
			ExpandedContent: expanded,
			Compact:         compact,
		}))
	}

	longDesc := strings.Repeat("d", 200)
	longDescIn := `{"command":"echo hi","description":"` + longDesc + `"}`

	// Collapsed: the description truncates to one line and the command
	// follows, indented under the tool name.
	collapsed := strings.Split(render(longDescIn, false, false), "\n")
	require.Contains(t, collapsed[0], "…")
	require.LessOrEqual(t, lipgloss.Width(collapsed[0]), capped)
	require.True(t, strings.HasPrefix(collapsed[1], indent+"echo hi"),
		"command must be indented under the tool name: %q", collapsed[1])

	// Expanded: the description wraps, never truncates.
	expandedOut := strings.Split(render(longDescIn, true, false), "\n")
	require.NotContains(t, strings.Join(expandedOut, "\n"), "…",
		"expanded description must wrap, not truncate")
	require.True(t, strings.HasPrefix(expandedOut[1], indent+"d"),
		"wrapped description must be indented under the tool name: %q", expandedOut[1])
	require.Contains(t, strings.Join(expandedOut, "\n"), "\n"+indent+"echo hi")

	// Only the first line of a multi-line description shows when
	// collapsed; the whole description shows when expanded.
	multiDescIn := `{"command":"echo hi","description":"run the echo\nthen the suite in detail"}`
	out := render(multiDescIn, false, false)
	require.Contains(t, out, "run the echo")
	require.NotContains(t, out, "then the suite")

	out = render(multiDescIn, true, false)
	require.Contains(t, out, "run the echo")
	require.Contains(t, out, "then the suite in detail")

	// The "background=true" pair rides on the description line.
	out = render(`{"command":"sleep 5","description":"wait","run_in_background":true}`, false, false)
	require.Contains(t, out, "wait (background=true)")

	// The chip sits between the tool name and the description.
	out = render(`{"command":"echo hi","working_dir":"/workspace/sub","description":"do things"}`, false, false)
	require.Contains(t, out, "Bash cd sub/ do things")

	// Nested tool calls (compact) keep the plain command header.
	out = render(`{"command":"echo hi","description":"do things"}`, false, true)
	require.Contains(t, out, "echo hi")
	require.NotContains(t, out, "do things")

	// A present-but-empty description keeps the plain command header too.
	out = render(`{"command":"echo hi","description":""}`, false, false)
	require.Contains(t, strings.Split(out, "\n")[0], "echo hi")
}
