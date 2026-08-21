package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// testWorkspace is a minimal [workspace.Workspace] stub: the dialogs only
// reach Config().
type testWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *testWorkspace) Config() *config.Config { return w.cfg }

func newTestCommandsDialog(t *testing.T, catalog []skills.CatalogEntry) *Commands {
	t.Helper()
	s := styles.CharmtonePantera()
	com := &common.Common{
		Styles:    &s,
		Workspace: &testWorkspace{cfg: &config.Config{}},
	}
	d, err := NewCommands(com, "test-session", true, false, false, []commands.CustomCommand{}, []commands.MCPPrompt{}, catalog)
	require.NoError(t, err)
	return d
}

// TestCommandsSkillsTabListsTheFullCatalog — the Skills tab shows every
// discovered skill (not just the user-invocable subset), name + description,
// and selecting one attaches it to the conversation.
func TestCommandsSkillsTabListsTheFullCatalog(t *testing.T) {
	catalog := []skills.CatalogEntry{
		{ID: "al-biruni", Name: "al-biruni", Description: "two minds, one brain"},
		{ID: "superpowers", Name: "superpowers", Description: "the whole toolbox"},
		{ID: "ctx-out", Name: "ctx-out", Description: "context management"},
	}
	d := newTestCommandsDialog(t, catalog)

	require.Equal(t, SystemCommands, d.selected, "opens on System by default")
	d.SelectSkillsTab()
	require.Equal(t, SkillsCommands, d.selected, "SelectSkillsTab jumps to the Skills tab")

	items := d.list.FilteredItems()
	require.Len(t, items, len(catalog), "all skills are listed, user-invocable or not")

	names := map[string]string{}
	for _, it := range items {
		ci, ok := it.(*CommandItem)
		require.True(t, ok, "skill items are CommandItems")
		names[ci.title] = ci.description
	}
	require.Equal(t, "two minds, one brain", names["al-biruni"])
	require.Equal(t, "the whole toolbox", names["superpowers"])

	// selecting the first skill returns an attach action, not a raw command
	action := items[0].(*CommandItem).Action()
	attach, ok := action.(ActionAttachSkill)
	require.True(t, ok, "selecting a skill attaches it: %T", action)
	require.Equal(t, "al-biruni", attach.Name)
}

// TestCommandsSkillsTabEmptyCatalog — with no skills the tab is a no-op, so
// the dialog stays on System.
func TestCommandsSkillsTabEmptyCatalog(t *testing.T) {
	d := newTestCommandsDialog(t, nil)
	d.SelectSkillsTab()
	require.Equal(t, SystemCommands, d.selected)
}
