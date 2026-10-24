package chat

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// The tests below switch the theme the way the UI does: the shared
// *styles.Styles is overwritten in place and the item caches are cleared.

// switchedStyles returns the default theme recolored with the given primary
// and foregrounds.
func switchedStyles(t *testing.T, primary string) styles.Styles {
	t.Helper()
	s, err := styles.HostThemeStyles("", map[string]string{
		"primary":        primary,
		"fg_base":        "#ff0101",
		"fg_subtle":      "#ff0202",
		"fg_more_subtle": "#ff0303",
		"fg_most_subtle": "#ff0404",
		"success":        "#ff0505",
	}, nil)
	require.NoError(t, err)
	return s
}

// sgrColor returns the truecolor sequence fragment for "#rrggbb".
func sgrColor(color string) string {
	var r, g, b int
	_, _ = fmt.Sscanf(color, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("2;%d;%d;%dm", r, g, b)
}

func TestClearItemCaches_ReachesNestedTools(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"agent", "agentic_fetch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sty := styles.CharmtonePantera()
			parentCall := message.ToolCall{ID: "parent", Name: name, Input: `{"prompt":"look"}`, Finished: true}
			parentResult := &message.ToolResult{ToolCallID: "parent", Content: "done"}
			childCall := message.ToolCall{ID: "child", Name: "glob", Input: `{"pattern":"*.go"}`, Finished: true}
			child := NewToolMessageItem(&sty, "msg", childCall, &message.ToolResult{ToolCallID: "child", Content: "main.go"}, false, "")

			var parent MessageItem
			switch name {
			case "agent":
				p := NewAgentToolMessageItem(&sty, parentCall, parentResult, false)
				p.SetNestedTools([]ToolMessageItem{child})
				parent = p
			default:
				p := NewAgenticFetchToolMessageItem(&sty, parentCall, parentResult, false)
				p.SetNestedTools([]ToolMessageItem{child})
				parent = p
			}

			// Rendering the parent caches the child's render too.
			require.NotContains(t, parent.Render(80), sgrColor("#ff0404")+"*.go")

			sty = switchedStyles(t, "#010203")
			ClearItemCaches([]MessageItem{parent})

			// The pattern is drawn by the nested tool alone, in
			// fg_most_subtle.
			require.Contains(t, parent.Render(80), sgrColor("#ff0404")+"*.go",
				"nested tool kept its old-theme render")
		})
	}
}

func TestHighlight_UsesCurrentSelectionStyle(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	h := defaultHighlighter(&sty, nil)
	h.SetHighlight(0, MessageLeftPaddingTotal, 0, MessageLeftPaddingTotal+5)

	require.NotContains(t, h.renderHighlighted("hello world", 20, 1), sgrColor("#010203"))
	sty = switchedStyles(t, "#010203")
	require.Contains(t, h.renderHighlighted("hello world", 20, 1), sgrColor("#010203"))
}

func TestPendingAssistantItem_RestylesSpinner(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	p := NewPendingAssistantItem(&sty, "s1")
	for range 3 {
		p.Advance()
	}
	// RawRender is the spinner alone: its frames are pre-rendered, so
	// without a restyle they keep the old gradient.
	before := p.RawRender(80)

	sty = switchedStyles(t, "#010203")
	ClearItemCaches([]MessageItem{p})
	require.NotEqual(t, before, p.RawRender(80), "spinner kept its old-theme gradient")
}
