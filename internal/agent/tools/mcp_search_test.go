package tools

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/stretchr/testify/require"
)

func TestRenderSearchResultListsAndConfirms(t *testing.T) {
	refs := []mcp.ToolRef{
		{
			Server:      "github",
			Tool:        "create_issue",
			Name:        "mcp_github_create_issue",
			Description: "Create an issue in a repository",
			Required:    []string{"owner", "repo", "title"},
			Lazy:        true,
		},
		{
			Server: "github",
			Tool:   "list_issues",
			Name:   "mcp_github_list_issues",
		},
	}

	out := renderSearchResult(refs, 0, 41, nil)

	// The model needs to know the tools arrive on the next step, not that
	// something went wrong.
	require.Contains(t, out, "callable on your next step")
	require.Contains(t, out, "continue the task now")
	require.Contains(t, out, "- mcp_github_create_issue: Create an issue in a repository [required: owner, repo, title]")
	require.Contains(t, out, "- mcp_github_list_issues")
	require.Contains(t, out, "LOADED: mcp_github_create_issue,mcp_github_list_issues")
	require.Contains(t, out, "Hidden MCP tools remaining: 41")
	require.NotContains(t, out, "already loaded")
}

func TestRenderSearchResultNotesAlreadyLoaded(t *testing.T) {
	refs := []mcp.ToolRef{{Name: "mcp_github_view", Tool: "view", Server: "github"}}
	out := renderSearchResult(refs, 1, 0, nil)
	require.Contains(t, out, "(1 were already loaded)")
	require.Contains(t, out, "Hidden MCP tools remaining: 0")
}

func TestRenderSearchResultWithoutMatches(t *testing.T) {
	// With tools still hidden, point at the servers so the model retries
	// rather than giving up.
	out := renderSearchResult(nil, 0, 12, []string{"- github: 12 tools hidden, use mcp_search to load them"})
	require.Contains(t, out, "No matching MCP tools found")
	require.Contains(t, out, "- github: 12 tools hidden")

	// Nothing hidden at all is a different answer: retrying is pointless.
	out = renderSearchResult(nil, 0, 0, []string{"- github: 12 tools hidden"})
	require.Contains(t, out, "No MCP tools are hidden right now")
	require.False(t, strings.Contains(out, "- github"),
		"an unanswerable retry hint must not be shown")
}
