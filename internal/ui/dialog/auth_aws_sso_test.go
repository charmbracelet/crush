package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

const testSSOURL = "https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH"

func newTestAWSSSO() *AWSSSO {
	s := styles.CharmtonePantera()
	m, _ := NewAWSSSO(&common.Common{Styles: &s}, "aws sso login")
	return m
}

// TestAWSSSOShowsWholeURL verifies the verification URL is wrapped
// rather than truncated, so the user code at its end stays visible, and
// that it can be copied.
func TestAWSSSOShowsWholeURL(t *testing.T) {
	t.Parallel()

	for _, w := range []int{100, 40} {
		m := newTestAWSSSO()
		m.SetURL(testSSOURL)
		lines := drawFits(t, m, w, 30)
		var joined strings.Builder
		for _, l := range lines {
			joined.WriteString(strings.TrimSpace(strings.Trim(strings.TrimSpace(l), "│")))
		}
		require.Contains(t, joined.String(), testSSOURL, "width %d", w)
		require.NotContains(t, strings.Join(lines, "\n"), "…│")
	}

	m := newTestAWSSSO()
	m.SetURL(testSSOURL)
	require.Contains(t, strings.Join(drawFits(t, m, 100, 30), "\n"), "c copy url")
	action := m.HandleMsg(tea.KeyPressMsg{Code: 'c', Text: "c"})
	_, ok := action.(ActionCmd)
	require.True(t, ok)
}

// TestAWSSSOSuccessMatchesOAuth verifies the success screen uses the
// same left-aligned, padded layout as the OAuth and MCP dialogs.
func TestAWSSSOSuccessMatchesOAuth(t *testing.T) {
	t.Parallel()

	m := newTestAWSSSO()
	m.Finish("")
	lines := drawFits(t, m, 100, 30)
	row := -1
	for i, l := range lines {
		if strings.Contains(l, "Authentication successful!") {
			row = i
		}
	}
	require.NotEqual(t, -1, row)
	require.NotContains(t, lines[row], "✓")
	require.Contains(t, lines[row], "│ Authentication successful!")
	require.Equal(t, "│", strings.Trim(strings.TrimSpace(lines[row-1]), " │")+"│", "blank line above")
	require.Equal(t, "│", strings.Trim(strings.TrimSpace(lines[row+1]), " │")+"│", "blank line below")
}
