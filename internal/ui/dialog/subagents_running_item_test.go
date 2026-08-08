package dialog

import (
	"testing"

	uistyles "github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// TestRunningSubagentItem_RenderContainsName verifies that the rendered output
// of a RunningSubagentItem contains the agent name and model string.
func TestRunningSubagentItem_RenderContainsName(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	item := NewRunningSubagentItem(&st, RunningSubagentItemData{
		Name:             "my-agent",
		Color:            "blue",
		Model:            "claude-opus-4-7",
		PromptTokens:     100,
		CompletionTokens: 50,
	})

	rendered := item.Render(60)
	plain := stripANSIDialog(rendered)

	require.Contains(t, plain, "my-agent")
	require.Contains(t, plain, "claude-opus-4-7")
}

// TestRunningSubagentItem_RenderContainsTokenCount verifies that the rendered
// output contains the sum of prompt and completion tokens formatted as "N tok".
func TestRunningSubagentItem_RenderContainsTokenCount(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	item := NewRunningSubagentItem(&st, RunningSubagentItemData{
		Name:             "my-agent",
		Color:            "blue",
		Model:            "claude-opus-4-7",
		PromptTokens:     100,
		CompletionTokens: 50,
	})

	rendered := item.Render(60)
	plain := stripANSIDialog(rendered)

	require.Contains(t, plain, "150 tok")
}

// TestRunningSubagentItem_RenderContainsDot verifies that the rendered output
// contains the colored dot produced by styles.SubagentDot for the item's color.
func TestRunningSubagentItem_RenderContainsDot(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	item := NewRunningSubagentItem(&st, RunningSubagentItemData{
		Name:             "my-agent",
		Color:            "blue",
		Model:            "claude-opus-4-7",
		PromptTokens:     100,
		CompletionTokens: 50,
	})

	rendered := item.Render(60)
	dot := uistyles.SubagentDot("blue")

	require.Contains(t, rendered, dot)
}
