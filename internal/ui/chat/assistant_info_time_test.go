package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestFormatTurnInfo(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("test", 7*60*60)
	stamp := time.Date(2026, time.July, 11, 7, 55, 12, 0, location)
	finish := message.Finish{InputTokens: 15_240, OutputTokens: 420}
	expected := "15k ctx · 420 out · " + stamp.Local().Format("15:04:05 · Mon, Jan 02, 2006")
	require.Equal(t, expected, formatTurnInfo(finish, stamp))
}

func TestFormatTurnInfoMarksEstimatedUsage(t *testing.T) {
	t.Parallel()

	stamp := time.Date(2026, time.July, 11, 7, 55, 12, 0, time.Local)
	finish := message.Finish{InputTokens: 1_250, OutputTokens: 25, EstimatedTokens: true}
	require.Contains(t, formatTurnInfo(finish, stamp), "~1.2k ctx · 25 out")
}
