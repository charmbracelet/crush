package dialog

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type stubSkillsWorkspace struct {
	workspace.Workspace
	entries []skills.CatalogEntry
	states  []*skills.SkillState
	cfg     *config.Config
	listErr error
}

func (w *stubSkillsWorkspace) ListSkills(_ context.Context) ([]skills.CatalogEntry, error) {
	return w.entries, w.listErr
}

func (w *stubSkillsWorkspace) GetSkillStates() []*skills.SkillState {
	return w.states
}

func (w *stubSkillsWorkspace) Config() *config.Config {
	return w.cfg
}

func newTestSkillsDialog(t *testing.T, entries []skills.CatalogEntry, states []*skills.SkillState, disabled []string) *Skills {
	t.Helper()
	s := styles.CharmtonePantera()
	cfg := &config.Config{
		Options: &config.Options{
			DisabledSkills: disabled,
		},
	}
	com := &common.Common{
		Workspace: &stubSkillsWorkspace{entries: entries, states: states, cfg: cfg},
		Styles:    &s,
	}
	return NewSkills(com, com.Workspace)
}

func TestSkillsDialog_EscCloses(t *testing.T) {
	t.Parallel()

	d := newTestSkillsDialog(
		t,
		[]skills.CatalogEntry{{ID: "a", Name: "Alpha", Source: skills.SourceSystem}},
		nil,
		nil,
	)

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	_, ok := action.(ActionClose)
	require.True(t, ok, "Esc should close the dialog")
}

func TestSkillsDialog_ReloadAction(t *testing.T) {
	t.Parallel()

	d := newTestSkillsDialog(
		t,
		[]skills.CatalogEntry{{ID: "a", Name: "Alpha", Source: skills.SourceSystem}},
		nil,
		nil,
	)

	action := d.HandleMsg(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	_, ok := action.(ActionSkillsReload)
	require.True(t, ok, "Ctrl+R should fire ActionSkillsReload")
}

func TestSkillsDialog_ToggleAction(t *testing.T) {
	t.Parallel()

	d := newTestSkillsDialog(
		t,
		[]skills.CatalogEntry{{ID: "alpha", Name: "Alpha", Source: skills.SourceSystem}},
		nil,
		nil,
	)

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	toggle, ok := action.(ActionSkillToggle)
	require.True(t, ok, "Enter should fire ActionSkillToggle")
	require.Equal(t, "Alpha", toggle.SkillName)
}

func TestSkillsDialog_NavigationWraps(t *testing.T) {
	t.Parallel()

	d := newTestSkillsDialog(
		t,
		[]skills.CatalogEntry{
			{ID: "a", Name: "Alpha", Source: skills.SourceSystem},
			{ID: "b", Name: "Beta", Source: skills.SourceUser},
		},
		nil,
		nil,
	)

	first := d.selectedSkill()
	require.NotNil(t, first)

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	second := d.selectedSkill()
	require.NotNil(t, second)
	require.NotEqual(t, first.entry.Name, second.entry.Name)

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	wrapped := d.selectedSkill()
	require.NotNil(t, wrapped)
	require.Equal(t, first.entry.Name, wrapped.entry.Name)

	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	upWrapped := d.selectedSkill()
	require.NotNil(t, upWrapped)
	require.Equal(t, second.entry.Name, upWrapped.entry.Name)
}

func TestSkillsDialog_FilterNarrowsList(t *testing.T) {
	t.Parallel()

	d := newTestSkillsDialog(
		t,
		[]skills.CatalogEntry{
			{ID: "alpha", Name: "Alpha", Source: skills.SourceSystem},
			{ID: "beta", Name: "Beta", Source: skills.SourceUser},
		},
		nil,
		nil,
	)

	d.HandleMsg(keyMsg('a'))
	d.HandleMsg(keyMsg('l'))

	filtered := d.list.FilteredItems()
	require.Len(t, filtered, 1)
	item := filtered[0].(*SkillItem)
	require.Equal(t, "Alpha", item.entry.Name)
}

func TestSkillsDialog_EmptyNoPanic(t *testing.T) {
	t.Parallel()

	d := newTestSkillsDialog(t, nil, nil, nil)

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, action, "no action when no skill selected")
}

func TestSkillItem_RenderDoesNotPanic(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	entry := skills.CatalogEntry{
		ID:          "test",
		Name:        "Test Skill",
		Description: "A test skill",
		Source:      skills.SourceSystem,
	}
	state := &skills.SkillState{Name: "Test Skill", State: skills.StateNormal}
	item := NewSkillItem(&s, entry, state, false)

	rendered := item.Render(60)
	require.NotEmpty(t, rendered)
}

func TestSkillItem_FilterMatchesNameAndDescription(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	entry := skills.CatalogEntry{
		ID:          "jq",
		Name:        "jq",
		Description: "Process JSON data",
		Source:      skills.SourceSystem,
	}
	item := NewSkillItem(&s, entry, nil, false)
	require.Contains(t, item.Filter(), "jq")
	require.Contains(t, item.Filter(), "JSON")
}

func TestSkillItem_DisabledRendering(t *testing.T) {
	t.Parallel()

	s := styles.CharmtonePantera()
	entry := skills.CatalogEntry{
		ID:     "disabled-skill",
		Name:   "Disabled",
		Source: skills.SourceUser,
	}
	item := NewSkillItem(&s, entry, nil, true)
	rendered := item.Render(60)
	require.NotEmpty(t, rendered)
}

func TestSkillsDialog_ShowsDisabledSkillsFromStates(t *testing.T) {
	t.Parallel()

	// Catalog returns only active skills. A disabled skill should still
	// appear via the states list.
	d := newTestSkillsDialog(
		t,
		[]skills.CatalogEntry{{ID: "active", Name: "Active", Source: skills.SourceSystem}},
		[]*skills.SkillState{
			{Name: "Active", State: skills.StateNormal},
			{Name: "DisabledOne", State: skills.StateNormal},
		},
		[]string{"DisabledOne"},
	)

	items := d.list.FilteredItems()
	// Should have both the active skill and the disabled one from states.
	require.GreaterOrEqual(t, len(items), 2)

	names := make(map[string]bool, len(items))
	for _, item := range items {
		if si, ok := item.(*SkillItem); ok {
			names[si.entry.Name] = true
		}
	}
	require.True(t, names["Active"])
	require.True(t, names["DisabledOne"])
}

func TestSkillsDialog_ListSkillsErrorRendersEmpty(t *testing.T) {
	t.Parallel()

	// When ListSkills fails, the dialog should render empty rather than
	// panicking or showing stale data, and key actions must be safe no-ops.
	s := styles.CharmtonePantera()
	com := &common.Common{
		Workspace: &stubSkillsWorkspace{
			listErr: errors.New("boom"),
			cfg:     &config.Config{Options: &config.Options{}},
		},
		Styles: &s,
	}
	d := NewSkills(com, com.Workspace)

	require.Empty(t, d.list.FilteredItems(), "no items when ListSkills errors")
	require.Nil(t, d.selectedSkill(), "no selection when the list is empty")

	// Enter (toggle) and Ctrl+R (reload) must not panic with no selection.
	require.Nil(t, d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}), "toggle is a no-op with no selection")
	reload := d.HandleMsg(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	_, ok := reload.(ActionSkillsReload)
	require.True(t, ok, "reload should still fire even when the list is empty")
}

func TestSkillsDialog_ActiveSkillNotDuplicatedByState(t *testing.T) {
	t.Parallel()

	// In production CatalogEntry.ID is the skill's file path (not its name),
	// while SkillState.Name is the name, and the states list includes active
	// skills. The dedup must key on name, or an active skill present in both
	// the catalog and the states list gets listed twice (once "on", once
	// "off").
	d := newTestSkillsDialog(
		t,
		[]skills.CatalogEntry{
			{ID: "/skills/alpha/SKILL.md", Name: "Alpha", Source: skills.SourceUser},
		},
		[]*skills.SkillState{
			{Name: "Alpha", State: skills.StateNormal},
		},
		nil,
	)

	items := d.list.FilteredItems()
	require.Len(t, items, 1, "an active skill must not be duplicated by its states entry")
	require.Equal(t, "Alpha", items[0].(*SkillItem).entry.Name)
	require.False(t, items[0].(*SkillItem).disabled, "the single entry should be the active one")
}
