package chat

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// nestedToolParent builds an agent or agentic-fetch item holding n finished
// nested glob calls whose patterns are p00.go, p01.go and so on.
func nestedToolParent(t *testing.T, name string, n int) MessageItem {
	t.Helper()

	sty := styles.CharmtonePantera()
	call := message.ToolCall{ID: "parent", Name: name, Input: `{"prompt":"look"}`, Finished: true}
	result := &message.ToolResult{ToolCallID: "parent", Content: "done"}

	nested := make([]ToolMessageItem, 0, n)
	for i := range n {
		id := fmt.Sprintf("child-%02d", i)
		childCall := message.ToolCall{ID: id, Name: "glob", Input: fmt.Sprintf(`{"pattern":"p%02d.go"}`, i), Finished: true}
		nested = append(nested, NewToolMessageItem(&sty, "msg", childCall, &message.ToolResult{ToolCallID: id, Content: "x"}, false, ""))
	}

	if name == "agent" {
		p := NewAgentToolMessageItem(&sty, call, result, false)
		p.SetNestedTools(nested)
		return p
	}
	p := NewAgenticFetchToolMessageItem(&sty, call, result, false)
	p.SetNestedTools(nested)
	return p
}

// TestNestedToolCalls_CollapsedShowsOnlyTheLatest pins that a collapsed
// subagent item draws a bounded number of its tool calls, newest last, with a
// line counting the rest, and that expanding it shows every call.
func TestNestedToolCalls_CollapsedShowsOnlyTheLatest(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"agent", "agentic_fetch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			total := collapsedNestedTools + 5
			parent := nestedToolParent(t, name, total)

			collapsed := ansi.Strip(parent.Render(100))
			require.Contains(t, collapsed, fmt.Sprintf("%d earlier tool calls", total-collapsedNestedTools))
			require.NotContains(t, collapsed, "p00.go")
			require.NotContains(t, collapsed, "p04.go")
			require.Contains(t, collapsed, "p05.go")
			require.Contains(t, collapsed, fmt.Sprintf("p%02d.go", total-1))

			expandable, ok := parent.(Expandable)
			require.True(t, ok)
			expandable.ToggleExpanded()
			expanded := ansi.Strip(parent.Render(100))
			require.NotContains(t, expanded, "earlier tool calls")
			require.Contains(t, expanded, "p00.go")
			require.Contains(t, expanded, fmt.Sprintf("p%02d.go", total-1))
		})
	}
}

// TestNestedToolCalls_AtTheCapShowsEverything covers the boundary: exactly
// the cap hides nothing, so there is no "earlier" line.
func TestNestedToolCalls_AtTheCapShowsEverything(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"agent", "agentic_fetch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out := ansi.Strip(nestedToolParent(t, name, collapsedNestedTools).Render(100))
			require.NotContains(t, out, "earlier tool calls")
			require.Contains(t, out, "p00.go")
		})
	}
}
