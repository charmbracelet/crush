package model

import (
	"strings"
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
	now := time.Now()
	row := u.subAgentRow(chat.DispatchStatus{
		Label:   "freddy-ysam-redteam-audit-with-a-very-long-tail",
		Running: true,
		Started: now.Add(-42 * time.Second),
	}, width, now)

	plain := ansi.Strip(row)
	require.LessOrEqual(t, lipgloss.Width(row), width)
	require.Contains(t, plain, "Freddy")
	require.Contains(t, plain, "…")
	require.Regexp(t, `\d+s$`, plain)
}

func TestSubAgentRowLeavesShortLabelWhole(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	plain := ansi.Strip(u.subAgentRow(chat.DispatchStatus{Label: "xochitl-porky"}, 40, time.Now()))
	require.Equal(t, "porky", lastWord(plain))
	require.NotContains(t, plain, "done")
	require.NotContains(t, plain, "…")
}

func lastWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
