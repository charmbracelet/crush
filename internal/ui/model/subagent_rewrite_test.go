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
	require.Len(t, m.activeSubagentItems, 2)

	// Discovery change drops beta — cache must reflect it on rebuild.
	ws.active = []workspace.SubagentInfo{{Name: "alpha"}}
	m.rebuildSubagentCaches()
	require.Len(t, m.activeSubagentItems, 1, "removed subagent must drop from cache")
	require.Equal(t, "alpha", m.activeSubagentItems[0].Name)
}

func TestBuildSubagentCaches(t *testing.T) {
	t.Parallel()

	t.Run("empty_input", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, buildSubagentCaches(nil))
	})

	t.Run("populates_items", func(t *testing.T) {
		t.Parallel()
		got := buildSubagentCaches([]workspace.SubagentInfo{
			{Name: "code-reviewer", Description: "reviews code"},
			{Name: "tester", Description: "writes tests"},
		})

		require.Len(t, got, 2)
		require.Equal(t, "code-reviewer", got[0].Name)
		require.Equal(t, "reviews code", got[0].Description)
		require.Equal(t, "tester", got[1].Name)
	})

	t.Run("preserves_input_order", func(t *testing.T) {
		t.Parallel()
		got := buildSubagentCaches([]workspace.SubagentInfo{
			{Name: "zeta"},
			{Name: "alpha"},
			{Name: "mu"},
		})
		require.Equal(t, "zeta", got[0].Name)
		require.Equal(t, "alpha", got[1].Name)
		require.Equal(t, "mu", got[2].Name)
	})
}
