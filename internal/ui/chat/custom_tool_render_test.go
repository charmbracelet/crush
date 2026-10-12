package chat

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// A plugin tool renders with its own title and summary instead of its name
// and raw JSON input. Not parallel: it swaps the package-level lookup.
func TestCustomToolRendersTitleAndSummary(t *testing.T) {
	original := CustomToolLookup
	t.Cleanup(func() { CustomToolLookup = original })
	CustomToolLookup = func(name string) (config.CustomToolConfig, bool) {
		return config.CustomToolConfig{
			Description: "GitHub",
			Actions: map[string]config.CustomToolAction{"pr_list": {
				Description: "List",
				Params:      map[string]string{"limit": "Max"},
				Title:       "List PRs",
				Summary:     `Latest {{plural .limit 20 "pull request"}}`,
			}},
		}, name == "GitHub"
	}

	sty := styles.CharmtonePantera()
	call := message.ToolCall{ID: "c1", Name: "GitHub", Input: `{"action":"pr_list","limit":"1"}`, Finished: true}
	out := stripStyle(NewGenericToolMessageItem(&sty, call, &message.ToolResult{ToolCallID: "c1", Content: "4048 fix"}, false).Render(120))
	require.Contains(t, out, "List PRs")
	require.Contains(t, out, "Latest 1 pull request")
	require.NotContains(t, out, `"action"`)

	call.Name = "other"
	out = stripStyle(NewGenericToolMessageItem(&sty, call, nil, false).Render(120))
	require.Contains(t, out, `"action"`, "tools without templates keep the plain rendering")
}
