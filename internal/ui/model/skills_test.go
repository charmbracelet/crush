package model

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/pubsub"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	uistyles "github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// TestSkillStatusItemsIncludesBuiltinSkills verifies sidebar skills include
// both runtime-discovered skill states and builtin skills that may not have
// emitted a SkillState event yet.
func TestSkillStatusItemsIncludesBuiltinSkills(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	ui := &UI{
		com: &common.Common{Styles: &st},
		skillStates: []*skills.SkillState{
			{Name: "go-doc", Path: "/tmp/go-doc/SKILL.md", State: skills.StateNormal},
		},
	}

	items := ui.skillStatusItems()
	require.NotEmpty(t, items)

	var hasGoDoc bool
	for _, item := range items {
		if item.title == st.Resource.Name.Render("go-doc") {
			hasGoDoc = true
			break
		}
	}
	require.True(t, hasGoDoc)

	builtinSkills := skills.DiscoverBuiltin()
	require.NotEmpty(t, builtinSkills)

	var hasBuiltin bool
	for _, skill := range builtinSkills {
		if skill.Name == "go-doc" {
			continue
		}
		expected := st.Resource.Name.Render(skill.Name)
		for _, item := range items {
			if item.title == expected {
				hasBuiltin = true
				break
			}
		}
		if hasBuiltin {
			break
		}
	}
	require.True(t, hasBuiltin)
}

func TestSkillStatusItemsExcludesDisabledSkills(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	ui := &UI{
		com: &common.Common{
			Styles:    &st,
			Workspace: &testWorkspace{cfg: &config.Config{Options: &config.Options{DisabledSkills: []string{"go-doc", "crush-config"}}}},
		},
		skillStates: []*skills.SkillState{
			{Name: "go-doc", Path: "/tmp/go-doc/SKILL.md", State: skills.StateNormal},
		},
	}

	items := ui.skillStatusItems()

	for _, item := range items {
		require.NotEqual(t, "go-doc", item.name)
		require.NotEqual(t, "crush-config", item.name)
	}
}

func TestSkillStatusItemsShowsErroredSkillWithRedDot(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	ui := &UI{
		com: &common.Common{Styles: &st},
		skillStates: []*skills.SkillState{
			{Name: "go-doc", Path: "/tmp/go-doc/SKILL.md", State: skills.StateNormal},
			// Parse failures carry no name; the sidebar derives it from
			// the skill directory.
			{Path: "/tmp/broken/SKILL.md", State: skills.StateError, Err: fmt.Errorf("yaml: cannot unmarshal")},
			{Name: "invalid", Path: "/tmp/invalid/SKILL.md", State: skills.StateError, Err: fmt.Errorf("missing description")},
		},
	}

	items := ui.skillStatusItems()

	byName := make(map[string]skillStatusItem, len(items))
	for _, item := range items {
		byName[item.name] = item
	}

	broken, ok := byName["broken"]
	require.True(t, ok, "a parse-failed skill must still appear in the sidebar")
	require.Contains(t, broken.icon, st.Resource.ErrorIcon.String(), "errored skills must show the red dot")

	invalid, ok := byName["invalid"]
	require.True(t, ok, "a validation-failed skill must appear in the sidebar")
	require.Contains(t, invalid.icon, st.Resource.ErrorIcon.String())
}

// TestSkillStatusItemsFollowsInputs checks that each change the UI applies
// to the skill list's inputs rebuilds it, including an in-place theme swap.
func TestSkillStatusItemsFollowsInputs(t *testing.T) {
	t.Parallel()

	ui := newHostThemeTestUI(t, nil)
	ws := &ui.com.Workspace.(*themeTestWorkspace).testWorkspace
	find := func(name string) (skillStatusItem, bool) {
		for _, item := range ui.skillStatusItems() {
			if item.name == name {
				return item, true
			}
		}
		return skillStatusItem{}, false
	}

	first := ui.skillStatusItems()
	require.Equal(t, &first[0], &ui.skillStatusItems()[0], "an unchanged frame reuses the list")

	ui.Update(pubsub.Event[skills.Event]{Payload: skills.Event{States: []*skills.SkillState{{Name: "fresh-skill", Path: "/tmp/fresh-skill/SKILL.md"}}}})
	_, ok := find("fresh-skill")
	require.True(t, ok, "new skill states rebuild the list")

	ui.Update(skillsOverridesLoadedMsg{disabled: map[string]bool{"fresh-skill": true}})
	_, ok = find("fresh-skill")
	require.False(t, ok, "a repository override rebuilds the list")

	ui.Update(skillsOverridesLoadedMsg{})
	ws.cfg = &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}, DisabledSkills: []string{"fresh-skill"}}}
	_, ok = find("fresh-skill")
	require.False(t, ok, "a new config rebuilds the list")

	ws.cfg = &config.Config{Options: &config.Options{TUI: &config.TUIOptions{}}}
	before, _ := find("fresh-skill")
	theme := uistyles.CharmtonePantera()
	theme.Resource.Name = theme.Resource.Name.Bold(true).Underline(true)
	ui.applyTheme(theme)
	after, _ := find("fresh-skill")
	require.NotEqual(t, before.title, after.title, "a theme applied in place rebuilds the list")

	ui.skillStates = nil
	ui.Update(agentModelChangedMsg{})
	_, ok = find("fresh-skill")
	require.False(t, ok, "rebuilding the agent rebuilds the list")
}
