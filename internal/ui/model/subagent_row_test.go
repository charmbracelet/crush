package model

import (
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestSubAgentRowKeepsElapsedTimeWhenLabelIsLong(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	const width = 32
	row := u.subAgentRow(chat.DispatchStatus{
		Label:   "freddy-ysam-redteam-audit-with-a-very-long-tail",
		Running: true,
		Started: time.Now().Add(-42 * time.Second),
	}, width)

	plain := ansi.Strip(row)
	require.LessOrEqual(t, lipgloss.Width(row), width)
	require.Contains(t, plain, "Freddy")
	require.Contains(t, plain, "…")
	require.Regexp(t, `\d+s$`, plain)
}

func TestSubAgentRowLeavesShortLabelWhole(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	plain := ansi.Strip(u.subAgentRow(chat.DispatchStatus{Label: "xochitl-porky"}, 40))
	require.Contains(t, plain, "porky done")
	require.NotContains(t, plain, "…")
}
