package dialog

import (
	"strings"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func newTestAPIKeyInput() *APIKeyInput {
	s := styles.CharmtonePantera()
	m, _ := NewAPIKeyInput(&common.Common{Styles: &s}, false,
		catwalk.Provider{ID: catwalk.InferenceProviderOpenAI, Name: "OpenAI"},
		config.SelectedModel{}, config.SelectedModelTypeLarge)
	return m
}

func uvRect(w, h int) uv.Rectangle { return uv.Rect(0, 0, w, h) }

// TestAPIKeyInputLongValueStaysOnOneLine verifies a long key scrolls
// within a single input row in every state, including the error state
// whose prompt icon is wider than the default prompt.
func TestAPIKeyInputLongValueStaysOnOneLine(t *testing.T) {
	t.Parallel()

	for _, st := range []APIKeyInputState{APIKeyInputStateInitial, APIKeyInputStateError, APIKeyInputStateVerified} {
		for _, w := range []int{100, 40} {
			m := newTestAPIKeyInput()
			m.state = st
			m.input.SetValue(strings.Repeat("sk-abcdefgh", 12))
			lines := drawFits(t, m, w, 30)
			// Any fragment of the key counts, so a wrapped tail that starts
			// mid-word still shows up as a second row.
			rows := 0
			for _, l := range lines {
				if strings.Contains(l, "sk-") || strings.Contains(l, "abcd") || strings.Contains(l, "efgh") {
					rows++
				}
			}
			require.Equal(t, 1, rows, "key must sit on exactly one row (state %d, width %d)", st, w)
			cur := m.Draw(uv.NewScreenBuffer(w, 30), uvRect(w, 30))
			if cur != nil {
				require.Contains(t, lines[cur.Y], "sk-abcdefgh")
			}
		}
	}
}

// TestAPIKeyInputConfigPathWraps verifies the config path note wraps
// inside the content width with its padding on every line, both in the
// framed dialog and in onboarding (no frame).
func TestAPIKeyInputConfigPathWraps(t *testing.T) {
	t.Parallel()

	for _, onboarding := range []bool{false, true} {
		for _, w := range []int{100, 40} {
			m := newTestAPIKeyInput()
			m.isOnboarding = onboarding
			scr := uv.NewScreenBuffer(w, 30)
			m.Draw(scr, scr.Bounds())
			lines := strings.Split(scr.String(), "\n")
			start := -1
			for i, l := range lines {
				if strings.Contains(l, "global") {
					start = i
					break
				}
			}
			require.NotEqual(t, -1, start)
			for _, l := range lines[start:] {
				if strings.Contains(l, "enter submit") {
					break
				}
				text := strings.Trim(l, " │")
				if text == "" {
					continue
				}
				// Every line keeps its one-cell inset.
				inset := strings.Index(l, text)
				if !onboarding {
					inset -= strings.Index(l, "│") + len("│")
				}
				require.Equal(t, 1, inset, "line lost its padding (onboarding=%v, width %d): %q", onboarding, w, l)
				require.LessOrEqual(t, len([]rune(strings.TrimRight(l, " "))), w)
			}
		}
	}
}

// TestAPIKeyInputHelpMatchesState verifies the help bar only offers keys
// that do something in the current state.
func TestAPIKeyInputHelpMatchesState(t *testing.T) {
	t.Parallel()

	cases := map[APIKeyInputState]struct{ want, notWant []string }{
		APIKeyInputStateInitial:   {want: []string{"enter submit", "esc exit"}},
		APIKeyInputStateError:     {want: []string{"enter submit", "esc exit"}},
		APIKeyInputStateVerifying: {notWant: []string{"enter submit", "esc exit"}},
		APIKeyInputStateVerified:  {want: []string{"enter save & continue"}, notWant: []string{"esc exit"}},
	}
	for st, c := range cases {
		m := newTestAPIKeyInput()
		m.state = st
		out := strings.Join(drawFits(t, m, 100, 30), "\n")
		for _, w := range c.want {
			require.Contains(t, out, w, "state %d", st)
		}
		for _, w := range c.notWant {
			require.NotContains(t, out, w, "state %d", st)
		}
	}
}

// TestAPIKeyInputPlaceholderNotRedundant verifies the placeholder does
// not repeat the title.
func TestAPIKeyInputPlaceholderNotRedundant(t *testing.T) {
	t.Parallel()

	out := strings.Join(drawFits(t, newTestAPIKeyInput(), 100, 30), "\n")
	require.Contains(t, out, "> Paste your key...")
	require.NotContains(t, out, "Enter your API key")
}
