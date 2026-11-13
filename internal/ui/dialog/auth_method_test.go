package dialog

import (
	"fmt"
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
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

// TestAuthCardClickAreasMatchPaintedCards checks that each card takes clicks
// from its top border to its bottom border and no further, so a click area
// shifted by one row fails.
func TestAuthCardClickAreasMatchPaintedCards(t *testing.T) {
	t.Parallel()
	for _, onboarding := range []bool{false, true} {
		// The narrow layout stacks the cards.
		for _, width := range []int{36, 100} {
			t.Run(fmt.Sprintf("onboarding=%t/width=%d", onboarding, width), func(t *testing.T) {
				t.Parallel()
				s := styles.CharmtonePantera()
				m := NewAuthMethod(
					&common.Common{Styles: &s},
					onboarding,
					catwalk.Provider{ID: catwalk.InferenceProviderOpenAI, Name: "OpenAI"},
					config.SelectedModel{Provider: "openai", Model: "gpt-5.1"},
					config.SelectedModelTypeLarge,
				)
				area := image.Rect(2, 3, width-2, 58)
				scr := uv.NewScreenBuffer(area.Max.X, area.Max.Y)
				m.Draw(scr, area)
				lines := strings.Split(ansi.Strip(scr.Render()), "\n")
				labelAt := func(text string) image.Point {
					for y, line := range lines {
						if before, _, ok := strings.Cut(line, text); ok {
							return image.Pt(ansi.StringWidth(before), y)
						}
					}
					t.Fatalf("%q is not painted in:\n%s", text, scr.Render())
					return image.Point{}
				}
				// border walks from a label to the card's painted top or
				// bottom border in the label's column.
				border := func(from image.Point, step int) int {
					for y := from.Y; y >= 0 && y < scr.Height(); y += step {
						if scr.CellAt(from.X, y).Content == "─" {
							return y
						}
					}
					t.Fatalf("No card border above or below %v in:\n%s", from, scr.Render())
					return 0
				}
				// A first click on a card selects it.
				selectsAt := func(p image.Point, card int) bool {
					m.selected, m.lastClickCard = 1-card, -1
					m.HandleMsg(tea.MouseClickMsg{X: p.X, Y: p.Y, Button: tea.MouseLeft})
					return m.selected == card
				}
				for _, card := range []struct {
					label string
					index int
				}{{"API Key", 1}, {"with Subscription", 0}} {
					label := labelAt(card.label)
					top, bottom := border(label, -1), border(label, 1)
					for _, y := range []int{top, bottom} {
						require.True(t, selectsAt(image.Pt(label.X, y), card.index), "row %d is inside the %q card", y, card.label)
					}
					for _, y := range []int{top - 1, bottom + 1} {
						require.False(t, selectsAt(image.Pt(label.X, y), card.index), "row %d is outside the %q card", y, card.label)
					}
				}
			})
		}
	}
}
