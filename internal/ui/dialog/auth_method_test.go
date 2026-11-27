package dialog

import (
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func newTestAuthMethod() *AuthMethod {
	s := styles.CharmtonePantera()
	return NewAuthMethod(
		&common.Common{Styles: &s},
		false,
		catwalk.Provider{ID: catwalk.InferenceProviderOpenAI, Name: "OpenAI"},
		config.SelectedModel{Provider: "openai", Model: "gpt-5.1"},
		config.SelectedModelTypeLarge,
	)
}

func TestAuthMethodDefaultsToOAuth(t *testing.T) {
	t.Parallel()

	m := newTestAuthMethod()
	require.Equal(t, AuthMethodID, m.ID())

	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectAuthMethod)
	require.True(t, ok)
	require.True(t, selected.UseOAuth)
	require.Equal(t, "openai", string(selected.Provider.ID))
	require.Equal(t, "gpt-5.1", selected.Model.Model)
	require.Equal(t, config.SelectedModelTypeLarge, selected.ModelType)
}

func TestAuthMethodTogglesToAPIKey(t *testing.T) {
	t.Parallel()

	m := newTestAuthMethod()
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})

	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectAuthMethod)
	require.True(t, ok)
	require.False(t, selected.UseOAuth)

	// Up toggles back to OAuth.
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok = action.(ActionSelectAuthMethod)
	require.True(t, ok)
	require.True(t, selected.UseOAuth)

	// Left/right toggle as well, since the cards sit side by side.
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyRight})
	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok = action.(ActionSelectAuthMethod)
	require.True(t, ok)
	require.False(t, selected.UseOAuth)

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyLeft})
	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok = action.(ActionSelectAuthMethod)
	require.True(t, ok)
	require.True(t, selected.UseOAuth)
}

func TestAuthMethodClose(t *testing.T) {
	t.Parallel()

	m := newTestAuthMethod()
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	_, ok := action.(ActionClose)
	require.True(t, ok)
}

func TestAuthMethodVimKeysToggle(t *testing.T) {
	t.Parallel()

	m := newTestAuthMethod()
	m.HandleMsg(tea.KeyPressMsg{Code: 'l', Text: "l"})
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectAuthMethod)
	require.True(t, ok)
	require.False(t, selected.UseOAuth)

	m = newTestAuthMethod()
	m.HandleMsg(tea.KeyPressMsg{Code: 'h', Text: "h"})
	action = m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok = action.(ActionSelectAuthMethod)
	require.True(t, ok)
	require.False(t, selected.UseOAuth)
}

func TestAuthMethodMouseClickFocusesCard(t *testing.T) {
	t.Parallel()

	m := newTestAuthMethod()
	scr := uv.NewScreenBuffer(100, 30)
	m.Draw(scr, image.Rect(0, 0, 100, 30))
	require.False(t, m.oauthCardArea.Empty())
	require.False(t, m.apiKeyCardArea.Empty())

	action := m.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X:      m.apiKeyCardArea.Min.X + 1,
		Y:      m.apiKeyCardArea.Min.Y + 1,
		Button: tea.MouseLeft,
	}))
	require.Nil(t, action)
	require.Equal(t, 1, m.selected)

	// A click outside the cards changes nothing.
	action = m.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X:      0,
		Y:      0,
		Button: tea.MouseLeft,
	}))
	require.Nil(t, action)
	require.Equal(t, 1, m.selected)
}

func TestAuthMethodMouseDoubleClickConfirms(t *testing.T) {
	t.Parallel()

	m := newTestAuthMethod()
	scr := uv.NewScreenBuffer(100, 30)
	m.Draw(scr, image.Rect(0, 0, 100, 30))

	click := tea.MouseClickMsg(tea.Mouse{
		X:      m.apiKeyCardArea.Min.X + 1,
		Y:      m.apiKeyCardArea.Min.Y + 1,
		Button: tea.MouseLeft,
	})
	require.Nil(t, m.HandleMsg(click))
	action := m.HandleMsg(click)
	selected, ok := action.(ActionSelectAuthMethod)
	require.True(t, ok)
	require.False(t, selected.UseOAuth)
}

// TestAuthMethodStackedCardsFitShortScreen verifies that stacked cards
// shrink to fit the screen and that the mouse hit areas line up with the
// drawn cards. Previously each stacked card kept its full 13 rows, pushing
// the API Key card off-screen at 40x24, and the hit areas assumed a
// one-line prompt although it wraps to two lines at 40 columns.
func TestAuthMethodStackedCardsFitShortScreen(t *testing.T) {
	t.Parallel()

	for _, sz := range [][2]int{{40, 24}, {40, 18}, {100, 30}} {
		m := newTestAuthMethod()
		drawFits(t, m, sz[0], sz[1])
		for _, onboarding := range []bool{false, true} {
			requireAuthCardAreasMatch(t, onboarding, sz[0], sz[1])
		}
	}
}

// requireAuthCardAreasMatch asserts both card hit areas sit exactly on the
// drawn cards' corners.
func requireAuthCardAreasMatch(t *testing.T, onboarding bool, w, h int) {
	t.Helper()
	m := newTestAuthMethod()
	m.isOnboarding = onboarding
	scr := uv.NewScreenBuffer(w, h)
	m.Draw(scr, image.Rect(0, 0, w, h))
	for name, r := range map[string]image.Rectangle{"oauth": m.oauthCardArea, "api key": m.apiKeyCardArea} {
		require.False(t, r.Empty(), "%s card area at %dx%d", name, w, h)
		require.LessOrEqual(t, r.Max.Y, h, "%s card off-screen at %dx%d", name, w, h)
		require.Equal(t, "╭", scr.CellAt(r.Min.X, r.Min.Y).Content, "%s hit area top-left at %dx%d (onboarding=%v)", name, w, h, onboarding)
		require.Equal(t, "╯", scr.CellAt(r.Max.X-1, r.Max.Y-1).Content, "%s hit area bottom-right at %dx%d (onboarding=%v)", name, w, h, onboarding)
	}
}
