package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// activeSubagentsWorkspace stubs ActiveSubagents for rebuildSubagentCaches.
type activeSubagentsWorkspace struct {
	workspace.Workspace
	active []workspace.SubagentInfo
}

func (w *activeSubagentsWorkspace) ActiveSubagents() []workspace.SubagentInfo { return w.active }

// TestRebuildSubagentCaches verifies the handler invoked on a subagents.Event
// rebuilds the @-mention caches from the workspace's current active list, so a
// removed subagent stops being offered without a restart.
func TestRebuildSubagentCaches(t *testing.T) {
	t.Parallel()

	ws := &activeSubagentsWorkspace{active: []workspace.SubagentInfo{{Name: "alpha"}, {Name: "beta"}}}
	m := &UI{com: &common.Common{Workspace: ws}}

	m.rebuildSubagentCaches()
	require.True(t, m.activeSubagentNames["alpha"])
	require.True(t, m.activeSubagentNames["beta"])
	require.Len(t, m.activeSubagentItems, 2)

	// Discovery change drops beta — cache must reflect it on rebuild.
	ws.active = []workspace.SubagentInfo{{Name: "alpha"}}
	m.rebuildSubagentCaches()
	require.True(t, m.activeSubagentNames["alpha"])
	require.False(t, m.activeSubagentNames["beta"], "removed subagent must drop from cache")
	require.Len(t, m.activeSubagentItems, 1)
}

func TestBuildSubagentCaches(t *testing.T) {
	t.Parallel()

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()
		items, names := buildSubagentCaches(nil)
		require.Empty(t, items)
		require.Empty(t, names)
		require.NotNil(t, names, "names map must be allocated even when empty")
	})

	t.Run("populates_both_caches", func(t *testing.T) {
		t.Parallel()
		got, names := buildSubagentCaches([]workspace.SubagentInfo{
			{Name: "code-reviewer", Description: "reviews code"},
			{Name: "tester", Description: "writes tests"},
		})

		require.Len(t, got, 2)
		require.Equal(t, "code-reviewer", got[0].Name)
		require.Equal(t, "reviews code", got[0].Description)
		require.Equal(t, "tester", got[1].Name)
		require.True(t, names["code-reviewer"])
		require.True(t, names["tester"])
		require.False(t, names["missing"])
	})

	t.Run("preserves_input_order", func(t *testing.T) {
		t.Parallel()
		got, _ := buildSubagentCaches([]workspace.SubagentInfo{
			{Name: "zeta"},
			{Name: "alpha"},
			{Name: "mu"},
		})
		require.Equal(t, "zeta", got[0].Name)
		require.Equal(t, "alpha", got[1].Name)
		require.Equal(t, "mu", got[2].Name)
	})
}

// TestSendMessageRewriteFlow verifies the integration between the cached
// activeSubagentNames produced at UI init and the rewriteSubagentPrompt call
// at the head of sendMessage. Failing this test would mean the caches do not
// line up with the rewrite logic.
func TestSendMessageRewriteFlow(t *testing.T) {
	t.Parallel()

	_, names := buildSubagentCaches([]workspace.SubagentInfo{
		{Name: "code-reviewer", Description: "Reviews code."},
	})

	got := rewriteSubagentPrompt("@code-reviewer review staged", names)
	require.Equal(t, `Use the agent tool with subagent_type="code-reviewer" to handle this request: review staged`, got)

	// Unknown name passes through unchanged.
	got = rewriteSubagentPrompt("@missing do thing", names)
	require.Equal(t, "@missing do thing", got)
}

// TestRewriteSubagentPrompt covers the pure helper rewriteSubagentPrompt which
// detects an `@name rest` prefix pattern and rewrites it into the canonical
// agent-tool dispatch form when name is in the provided active-names set.
func TestRewriteSubagentPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		activeNames map[string]bool
		want        string
	}{
		{
			name:        "no_at_prefix",
			content:     "just a normal message",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "just a normal message",
		},
		{
			name:        "at_unknown_name",
			content:     "@unknown do something",
			activeNames: map[string]bool{},
			want:        "@unknown do something",
		},
		{
			name:        "at_known_single_word",
			content:     "@code-reviewer review staged",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        `Use the agent tool with subagent_type="code-reviewer" to handle this request: review staged`,
		},
		{
			name:        "at_known_multiword_rest",
			content:     "@tester write tests for the auth module please",
			activeNames: map[string]bool{"tester": true},
			want:        `Use the agent tool with subagent_type="tester" to handle this request: write tests for the auth module please`,
		},
		{
			name:        "at_name_no_space_after",
			content:     "@code-reviewer",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "@code-reviewer",
		},
		{
			name:        "at_name_only_whitespace_after",
			content:     "@code-reviewer   ",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "@code-reviewer   ",
		},
		{
			name:        "empty_content",
			content:     "",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "",
		},
		{
			name:        "at_name_with_leading_space",
			content:     "  @code-reviewer review it",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        "  @code-reviewer review it",
		},
		{
			name:        "nil_active_names_does_not_panic",
			content:     "@code-reviewer review it",
			activeNames: nil,
			want:        "@code-reviewer review it",
		},
		{
			name:        "newline_as_separator",
			content:     "@code-reviewer\nreview this",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        `Use the agent tool with subagent_type="code-reviewer" to handle this request: review this`,
		},
		{
			name:        "multiple_at_mentions_only_first_rewritten",
			content:     "@code-reviewer review and @tester test",
			activeNames: map[string]bool{"code-reviewer": true, "tester": true},
			want:        `Use the agent tool with subagent_type="code-reviewer" to handle this request: review and @tester test`,
		},
		{
			name:        "tab_as_separator",
			content:     "@code-reviewer\treview this",
			activeNames: map[string]bool{"code-reviewer": true},
			want:        `Use the agent tool with subagent_type="code-reviewer" to handle this request: review this`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rewriteSubagentPrompt(tt.content, tt.activeNames)
			require.Equal(t, tt.want, got)
		})
	}
}
