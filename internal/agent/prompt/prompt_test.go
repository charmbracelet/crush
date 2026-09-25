package prompt

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// gatedSkill is the builtin skill that declares `requires: honcho`, and
// so is the one that must appear or vanish with the feature.
const gatedSkill = "honcho-memory"

// buildSkillXML renders a prompt whose whole body is the skill block, so
// the assertions below read the exact text the model would receive.
func buildSkillXML(t *testing.T, opts ...Option) string {
	t.Helper()

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	p, err := NewPrompt("test", "{{.AvailSkillXML}}", opts...)
	require.NoError(t, err)

	out, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)
	return out
}

func TestSkillGatingHidesSkillWithoutFeature(t *testing.T) {
	t.Parallel()

	// An unset features func means no integrations are live, which is
	// the default every one-off prompt gets.
	xml := buildSkillXML(t)

	require.NotContains(t, xml, gatedSkill,
		"a skill requiring an unavailable integration must stay out of the prompt")
	// Guard against the assertion passing because skill discovery
	// produced nothing at all.
	require.Contains(t, xml, "crush-config",
		"ungated skills must still be present")
}

func TestSkillGatingShowsSkillWithFeature(t *testing.T) {
	t.Parallel()

	xml := buildSkillXML(t, WithFeatures(func() []string { return []string{"honcho"} }))

	require.Contains(t, xml, gatedSkill,
		"a skill whose required integration is live must be offered")
}

// TestSkillGatingIsReEvaluatedPerBuild is the regression guard for
// connecting memory mid-session. The features func is deliberately not a
// slice captured at construction: a backend connected after the prompt
// was built must show up on the next build without a restart.
func TestSkillGatingIsReEvaluatedPerBuild(t *testing.T) {
	t.Parallel()

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	// live stands in for the memory backend being connected partway
	// through the session.
	live := false
	p, err := NewPrompt("test", "{{.AvailSkillXML}}", WithFeatures(func() []string {
		if !live {
			return nil
		}
		return []string{"honcho"}
	}))
	require.NoError(t, err)

	before, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)
	require.NotContains(t, before, gatedSkill, "skill must be hidden before connecting")

	live = true

	after, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)
	require.Contains(t, after, gatedSkill, "skill must appear once connected, without a rebuild")
}

// TestSkillGatingDoesNotConsultConfig pins the behaviour that motivated
// the change: gating follows the live integration, not what the user
// asked for. An integration can be enabled in config and still fail to
// start, and a prompt that trusted config would promise the model tools
// it was never given.
func TestSkillGatingDoesNotConsultConfig(t *testing.T) {
	t.Parallel()

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)

	// Enable memory in config while leaving the features func unset,
	// which is what "configured but failed to start" looks like here.
	store.Config().Honcho = &config.Honcho{Enabled: true}

	p, err := NewPrompt("test", "{{.AvailSkillXML}}")
	require.NoError(t, err)

	xml, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)

	require.NotContains(t, xml, gatedSkill,
		"config alone must not unlock a skill when the integration is not actually live")
}

// TestSkillGatingLeavesDisabledSkillsAlone checks the two filters
// compose, since both run over the same slice in sequence.
func TestSkillGatingLeavesDisabledSkillsAlone(t *testing.T) {
	t.Parallel()

	store, err := config.Init(t.TempDir(), "", false)
	require.NoError(t, err)
	store.Config().Options.DisabledSkills = []string{"crush-config"}

	p, err := NewPrompt("test", "{{.AvailSkillXML}}",
		WithFeatures(func() []string { return []string{"honcho"} }))
	require.NoError(t, err)

	xml, err := p.Build(t.Context(), "", "", store)
	require.NoError(t, err)

	require.NotContains(t, xml, "crush-config", "explicitly disabled skills stay out")
	require.Contains(t, xml, gatedSkill, "gating must not disturb the disabled-skill filter")
	require.True(t, strings.Contains(xml, "jq"), "unrelated skills survive both filters")
}
