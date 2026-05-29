package agent

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/subagents"
	"github.com/stretchr/testify/require"
)

// newTestSkill constructs a minimal *skills.Skill for use in unit tests.
// The name must pass skills.Validate, so it must be alphanumeric with hyphens.
func newTestSkill(name string, disableModelInvocation bool) *skills.Skill {
	return &skills.Skill{
		Name:                   name,
		Description:            "test skill: " + name,
		DisableModelInvocation: disableModelInvocation,
		Instructions:           "do the " + name + " thing",
	}
}

// newTestSubagent constructs a minimal *subagents.Subagent whose fields satisfy
// any validation that subagentPrompt may perform internally.
func newTestSubagent(name string, skillNames []string, body string) *subagents.Subagent {
	return &subagents.Subagent{
		Name:        name,
		Description: "test subagent " + name,
		Skills:      skillNames,
		Body:        body,
	}
}

// ---------------------------------------------------------------------------
// resolvePreloadedSkillsXML
// ---------------------------------------------------------------------------

func TestResolvePreloadedSkillsXML(t *testing.T) {
	t.Parallel()

	alpha := newTestSkill("alpha", false)
	beta := newTestSkill("beta", false)
	gamma := newTestSkill("gamma", true) // DisableModelInvocation = true

	tests := []struct {
		name         string
		skillNames   []string
		activeSkills []*skills.Skill
		// wantContains is a slice of strings that must appear in the result.
		wantContains []string
		// wantAbsent is a slice of strings that must NOT appear in the result.
		wantAbsent []string
		// wantEmpty asserts the result is the empty string.
		wantEmpty bool
	}{
		{
			name:       "empty_skill_names",
			skillNames: nil,
			activeSkills: []*skills.Skill{
				alpha,
			},
			wantEmpty: true,
		},
		{
			name:         "empty_active_skills",
			skillNames:   []string{"alpha"},
			activeSkills: nil,
			wantEmpty:    true,
		},
		{
			name:         "single_skill_found",
			skillNames:   []string{"alpha"},
			activeSkills: []*skills.Skill{alpha},
			wantContains: []string{"alpha"},
		},
		{
			name:         "skill_not_found",
			skillNames:   []string{"missing"},
			activeSkills: []*skills.Skill{alpha, beta},
			wantEmpty:    true,
		},
		{
			name:         "disable_model_invocation_skipped",
			skillNames:   []string{"gamma"},
			activeSkills: []*skills.Skill{gamma},
			wantEmpty:    true,
		},
		{
			name:         "multiple_skills_some_found",
			skillNames:   []string{"alpha", "missing", "beta"},
			activeSkills: []*skills.Skill{alpha, beta},
			wantContains: []string{"alpha", "beta"},
			wantAbsent:   []string{"missing"},
		},
		{
			name:         "preserves_order",
			skillNames:   []string{"beta", "alpha"},
			activeSkills: []*skills.Skill{alpha, beta},
			// beta's FormatInvocation output must appear before alpha's in the result
			wantContains: []string{"beta", "alpha"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolvePreloadedSkillsXML(tc.skillNames, tc.activeSkills)

			if tc.wantEmpty {
				require.Empty(t, got)
				return
			}

			for _, want := range tc.wantContains {
				require.Contains(t, got, want)
			}

			for _, absent := range tc.wantAbsent {
				require.NotContains(t, got, absent)
			}
		})
	}
}

// TestResolvePreloadedSkillsXML_PreservesOrder verifies that when multiple
// skills are requested the output XML segments appear in skillNames order, not
// in activeSkills order.
func TestResolvePreloadedSkillsXML_PreservesOrder(t *testing.T) {
	t.Parallel()

	alpha := newTestSkill("alpha", false)
	beta := newTestSkill("beta", false)

	// Request beta before alpha even though activeSkills has alpha first.
	got := resolvePreloadedSkillsXML([]string{"beta", "alpha"}, []*skills.Skill{alpha, beta})

	betaIdx := strings.Index(got, "beta")
	alphaIdx := strings.Index(got, "alpha")

	require.NotEqual(t, -1, betaIdx, "beta should appear in result")
	require.NotEqual(t, -1, alphaIdx, "alpha should appear in result")
	require.Less(t, betaIdx, alphaIdx, "beta should appear before alpha in output")
}

// TestResolvePreloadedSkillsXML_FormatInvocationUsed verifies that the output
// for a found skill is derived from FormatInvocation() and therefore contains
// the <loaded_skill> wrapper element.
func TestResolvePreloadedSkillsXML_FormatInvocationUsed(t *testing.T) {
	t.Parallel()

	sk := newTestSkill("my-skill", false)
	got := resolvePreloadedSkillsXML([]string{"my-skill"}, []*skills.Skill{sk})

	// FormatInvocation always wraps output in <loaded_skill>…</loaded_skill>.
	require.Contains(t, got, "<loaded_skill>")
	require.Contains(t, got, "my-skill")
}

// ---------------------------------------------------------------------------
// subagentPrompt
// ---------------------------------------------------------------------------

func TestSubagentPrompt_NoSkills(t *testing.T) {
	t.Parallel()

	sa := newTestSubagent("no-skills-agent", nil, "You do things.")
	p, err := subagentPrompt(sa, nil)

	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestSubagentPrompt_WithKnownSkill(t *testing.T) {
	t.Parallel()

	sk := newTestSkill("helper-skill", false)
	sa := newTestSubagent("skilled-agent", []string{"helper-skill"}, "You use the helper skill.")

	p, err := subagentPrompt(sa, []*skills.Skill{sk})

	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestSubagentPrompt_WithUnknownSkill(t *testing.T) {
	t.Parallel()

	// Subagent requests a skill that is not in activeSkills — must not error.
	sa := newTestSubagent("unknown-skill-agent", []string{"nonexistent"}, "Body text.")

	p, err := subagentPrompt(sa, nil)

	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestSubagentPrompt_NilSubagentSkills(t *testing.T) {
	t.Parallel()

	sa := newTestSubagent("nil-skills-agent", nil, "")
	activeSkills := []*skills.Skill{newTestSkill("some-skill", false)}

	p, err := subagentPrompt(sa, activeSkills)

	require.NoError(t, err)
	require.NotNil(t, p)
}
