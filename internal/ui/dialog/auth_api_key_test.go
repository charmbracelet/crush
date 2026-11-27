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
			count := 0
			for _, l := range lines {
				count += strings.Count(l, "sk-abcdefgh")
			}
			require.Less(t, count, 12, "key wrapped onto extra rows (state %d, width %d)", st, w)
			cur := m.Draw(uv.NewScreenBuffer(w, 30), uvRect(w, 30))
			if cur != nil {
				require.Contains(t, lines[cur.Y], "sk-abcdefgh")
			}
		}
	}
}

func uvRect(w, h int) uv.Rectangle { return uv.Rect(0, 0, w, h) }
