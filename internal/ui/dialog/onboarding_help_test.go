package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestOnboardingHelpStartsOnEnter(t *testing.T) {
	t.Parallel()

	m := NewOnboardingHelp(testCommon(nil), []KeyHint{{Key: "enter", Desc: "send"}})
	require.Equal(t, OnboardingHelpID, m.ID())

	require.IsType(t, ActionOnboardingDone{}, m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
}

func TestOnboardingHelpIgnoresEscape(t *testing.T) {
	t.Parallel()

	m := NewOnboardingHelp(testCommon(nil), []KeyHint{{Key: "enter", Desc: "send"}})
	require.Nil(t, m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
}

func TestRenderKeyHintsAlignsKeys(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	view := RenderKeyHints(&sty, []KeyHint{
		{Key: "enter", Desc: "send"},
		{Key: "ctrl+p", Desc: "palette"},
	}, 12)

	lines := strings.Split(view, "\n")
	require.Len(t, lines, 2)
	require.Contains(t, lines[0], "enter")
	require.Contains(t, lines[0], "send")
	require.Contains(t, lines[1], "ctrl+p")
	require.Contains(t, lines[1], "palette")

	// Descriptions start on the same column regardless of key length.
	require.Equal(
		t,
		strings.Index(lines[0], "send"),
		strings.Index(lines[1], "palette"),
	)
}

func TestOnboardingProgress(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	progress := onboardingProgress(&sty, OnboardingStepProvider)
	require.Contains(t, progress, "1/3")

	progress = onboardingProgress(&sty, OnboardingStepHelp)
	require.Contains(t, progress, "3/3")

	require.Empty(t, onboardingProgress(&sty, OnboardingStep(99)))
}
