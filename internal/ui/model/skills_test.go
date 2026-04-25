package model

import (
	"testing"

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

	st := uistyles.DefaultStyles()
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

	st := uistyles.DefaultStyles()
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
