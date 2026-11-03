package model

import (
	"image"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOnboardingKeyHintsTrackBindings(t *testing.T) {
	t.Parallel()

	ui := newTestUIWithConfig(t, &config.Config{})
	ui.keyMap = DefaultKeyMap()

	hints := ui.onboardingKeyHints()
	require.NotEmpty(t, hints)

	enter := ui.keyMap.Editor.SendMessage.Help()
	require.Equal(t, enter.Key, hints[0].Key)
	require.Equal(t, "send", hints[0].Desc)

	for _, hint := range hints {
		require.NotEmpty(t, hint.Key, "every hint names a key")
		require.NotEmpty(t, hint.Desc, "every hint explains the key")
	}
}

func TestShowLandingTips(t *testing.T) {
	t.Parallel()

	newUI := func(t *testing.T, tips *bool) *UI {
		t.Helper()
		ui := newTestUIWithConfig(t, &config.Config{
			Options: &config.Options{TUI: &config.TUIOptions{Tips: tips}},
		})
		ui.state = uiLanding
		ui.layout.main = image.Rect(0, 0, 80, 24)
		return ui
	}

	t.Run("shown on first run", func(t *testing.T) {
		t.Parallel()
		require.True(t, newUI(t, nil).showLandingTips())
	})

	t.Run("hidden when disabled", func(t *testing.T) {
		t.Parallel()
		enabled := false
		require.False(t, newUI(t, &enabled).showLandingTips())
	})

	t.Run("hidden once dismissed", func(t *testing.T) {
		t.Parallel()
		ui := newUI(t, nil)
		ui.tipsDismissed = true
		require.False(t, ui.showLandingTips())
	})

	t.Run("hidden outside the landing view", func(t *testing.T) {
		t.Parallel()
		ui := newUI(t, nil)
		ui.state = uiChat
		require.False(t, ui.showLandingTips())
	})

	t.Run("hidden on small terminals", func(t *testing.T) {
		t.Parallel()
		ui := newUI(t, nil)
		ui.layout.main = image.Rect(0, 0, 40, 24)
		require.False(t, ui.showLandingTips())

		ui = newUI(t, nil)
		ui.layout.main = image.Rect(0, 0, 80, 10)
		require.False(t, ui.showLandingTips())
	})
}
