package model

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	uistyles "github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestParentBreadcrumbLine_EmptyTitle(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	got := parentBreadcrumbLine(&st, "", "", 40)
	require.Empty(t, got)
}

func TestParentBreadcrumbLine_WithTitle(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	got := parentBreadcrumbLine(&st, "blue", "Main Session", 40)
	require.Contains(t, stripANSI(got), "↑ parent: Main Session")
}

func TestParentBreadcrumbLine_WidthRespected(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	longTitle := "This Is An Extremely Long Session Title That Will Not Fit"
	got := parentBreadcrumbLine(&st, "", longTitle, 20)
	for line := range strings.SplitSeq(got, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 20, "line exceeds width: %q", line)
	}
}

func TestDefaultKeyMap_HasParentSessionBinding(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	require.True(t, km.ParentSession.Enabled(), "ParentSession binding should be enabled")
	require.Contains(t, km.ParentSession.Keys(), "ctrl+up")
}

// Verify that key.Binding is the type we're using (compile-time check).
var _ key.Binding = DefaultKeyMap().ParentSession
