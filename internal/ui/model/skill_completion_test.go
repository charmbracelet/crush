package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/textarea"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/completions"
	"github.com/stretchr/testify/require"
)

func newSkillCompletionUI() *UI {
	com := common.DefaultCommon(&slashCommandWorkspace{ready: true})
	m := &UI{
		com:      com,
		status:   NewStatus(com, nil),
		chat:     NewChat(com, config.ScrollbarDefault),
		textarea: textarea.New(),
		state:    uiChat,
		focus:    uiFocusEditor,
		width:    140,
		height:   45,
	}
	m.attachments = attachments.New(nil, attachments.Keymap{})
	m.completions = completions.New(
		com.Styles.Completions.Normal,
		com.Styles.Completions.Focused,
		com.Styles.Completions.Match,
	)
	return m
}

// TestOpenSkillCompletionsPopulatesFromSkillStates verifies the popup opens
// in skill mode with items built from healthy skill states only.
func TestOpenSkillCompletionsPopulatesFromSkillStates(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.skillStates = []*skills.SkillState{
		{Name: "commit", Path: "/skills/commit/SKILL.md", State: skills.StateNormal},
		{Name: "code-review", Path: "/skills/code-review/SKILL.md", State: skills.StateNormal},
		{Name: "broken", Path: "/skills/broken/SKILL.md", State: skills.StateError},
		nil,
	}

	m.openSkillCompletions(5)

	require.True(t, m.completionsOpen)
	require.Equal(t, completions.TriggerSkill, m.completionsTrigger)
	require.Equal(t, 5, m.completionsStartIndex)
	require.True(t, m.completions.IsOpen())
	require.True(t, m.completions.HasItems())
}

// TestSkillCompletionValuesPrefersCatalog verifies the popup is built from
// the effective skill catalog once it has loaded. The catalog is what
// carries descriptions, includes builtin skills, and has already resolved
// user-over-builtin overrides and the disabled-skills list — the raw
// discovery states do none of that.
func TestSkillCompletionValuesPrefersCatalog(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.skillStates = []*skills.SkillState{
		{Name: "only-a-state", Path: "/skills/only-a-state/SKILL.md", State: skills.StateNormal},
	}
	m.skillCatalog = []skills.CatalogEntry{
		{ID: "crush://skills/commit/SKILL.md", Name: "commit", Description: "Write a conventional commit."},
		{ID: "/skills/alpha/SKILL.md", Name: "alpha", Description: "Go first."},
		{Name: ""}, // Nameless entries are not selectable; drop them.
	}

	values := m.skillCompletionValues()

	require.Equal(t, []completions.SkillCompletionValue{
		{Name: "alpha", Description: "Go first.", Path: "/skills/alpha/SKILL.md"},
		{Name: "commit", Description: "Write a conventional commit.", Path: "crush://skills/commit/SKILL.md"},
	}, values)
}

// TestSkillCompletionValuesFallsBackToStates covers the window between
// startup and the first catalog load: a name-only list still beats an empty
// popup. Duplicate names (a user skill shadowing a builtin) collapse to one.
func TestSkillCompletionValuesFallsBackToStates(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.skillStates = []*skills.SkillState{
		{Name: "commit", Path: "/user/skills/commit/SKILL.md", State: skills.StateNormal},
		{Name: "commit", Path: "crush://skills/commit/SKILL.md", State: skills.StateNormal},
		{Name: "", Path: "/skills/unnamed/SKILL.md", State: skills.StateNormal},
		{Name: "broken", State: skills.StateError},
		nil,
	}

	values := m.skillCompletionValues()

	require.Equal(t, []completions.SkillCompletionValue{
		{Name: "commit", Path: "/user/skills/commit/SKILL.md"},
	}, values)
}

// TestOpenSkillCompletionsNoopWithoutSkills guards the empty-popup trap: an
// open completions popup consumes enter and the arrow keys, so opening one
// with nothing in it would leave a user with no skills unable to submit a
// prompt containing a '/'.
func TestOpenSkillCompletionsNoopWithoutSkills(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()

	m.openSkillCompletions(3)

	require.False(t, m.completionsOpen)
	require.Equal(t, completions.TriggerNone, m.completionsTrigger)
	require.False(t, m.completions.IsOpen())
}

// TestOpenSkillCompletionsSortsByName pins alphabetical ordering of the
// skill popup items: with no query the first selected item should be the
// alphabetically-first skill.
func TestOpenSkillCompletionsSortsByName(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.skillStates = []*skills.SkillState{
		{Name: "zebra", State: skills.StateNormal},
		{Name: "alpha", State: skills.StateNormal},
	}

	m.openSkillCompletions(0)

	// Select the first item without filtering: it should be "alpha".
	msg, handled := m.completions.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, handled)
	sel, ok := msg.(completions.SelectionMsg[completions.SkillCompletionValue])
	require.True(t, ok, "expected skill selection, got %T", msg)
	require.Equal(t, "alpha", sel.Value.Name)
}

// TestInsertSkillCompletion verifies selecting a skill replaces the /query
// with "/skill-name" plus a trailing space.
func TestInsertSkillCompletion(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.textarea.SetValue("hello can you /cod")
	m.completionsOpen = true
	m.completionsTrigger = completions.TriggerSkill
	m.completionsStartIndex = len("hello can you ")

	m.insertSkillCompletion(completions.SkillCompletionValue{Name: "code-review"})

	require.Equal(t, "hello can you /code-review ", m.textarea.Value())
}

// TestInsertSkillCompletionKeepsSlashRegression guards against the
// double-slash bug: the replacement span includes the trigger '/', so the
// inserted text must include exactly one.
func TestInsertSkillCompletionKeepsSlashRegression(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.textarea.SetValue("/com")
	m.completionsOpen = true
	m.completionsTrigger = completions.TriggerSkill
	m.completionsStartIndex = 0

	m.insertSkillCompletion(completions.SkillCompletionValue{Name: "commit"})

	require.Equal(t, "/commit ", m.textarea.Value())
}

// TestCloseCompletionsResetsTrigger verifies closing the popup clears the
// trigger mode so a stale async file load can't clobber a later skill popup.
func TestCloseCompletionsResetsTrigger(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.completionsOpen = true
	m.completionsTrigger = completions.TriggerSkill
	m.completionsQuery = "cod"
	m.completionsStartIndex = 3

	m.closeCompletions()

	require.False(t, m.completionsOpen)
	require.Equal(t, completions.TriggerNone, m.completionsTrigger)
	require.Empty(t, m.completionsQuery)
	require.Zero(t, m.completionsStartIndex)
}

// TestCompletionItemsLoadedIgnoredInSkillMode is the regression test for the
// stale async load: a file-completion load that resolves while the popup is
// in skill mode must not replace the skill items.
func TestCompletionItemsLoadedIgnoredInSkillMode(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.skillStates = []*skills.SkillState{{Name: "commit", State: skills.StateNormal}}
	m.openSkillCompletions(0)
	require.True(t, m.completions.HasItems())

	// A stale async file load completing while in skill mode must be ignored.
	m.Update(completions.CompletionItemsLoadedMsg{
		Files: []completions.FileCompletionValue{{Path: "foo.go"}},
	})

	// Skill items survive: filtering by skill name still matches.
	m.completions.Filter("commit")
	require.True(t, m.completions.HasItems())
}

// TestCompletionItemsLoadedAppliesInFileMode is the counterpart regression:
// the async file load must still populate the popup when in file mode.
func TestCompletionItemsLoadedAppliesInFileMode(t *testing.T) {
	t.Parallel()

	m := newSkillCompletionUI()
	m.completionsOpen = true
	m.completionsTrigger = completions.TriggerFile

	m.Update(completions.CompletionItemsLoadedMsg{
		Files: []completions.FileCompletionValue{{Path: "foo.go"}},
	})

	m.completions.Filter("foo")
	require.True(t, m.completions.HasItems())
}
