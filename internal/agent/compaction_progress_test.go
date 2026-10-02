package agent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompactionProgressThrottles(t *testing.T) {
	p := &compactionProgress{replaced: 40, goal: 8192, last: time.Now().Add(-2 * progressInterval)}

	// The first delta reports at once, so the wait is explained immediately
	// rather than after a quarter second of unlabelled spinner.
	require.Equal(t, "Compacting 40 messages · checkpoint 25/8.2k tokens", p.advance(100))
	// Silence is about the report, not the count: deltas that arrive inside
	// the interval still add up.
	require.Empty(t, p.advance(100), "a delta inside the interval must stay quiet")

	p.last = time.Now().Add(-2 * progressInterval)
	require.Equal(t, "Compacting 40 messages · checkpoint 75/8.2k tokens", p.advance(100))
}

// TestCompactionProgressNamesTheThinkingPhase matters most on reasoning
// models, which can spend longer thinking than writing.
func TestCompactionProgressNamesTheThinkingPhase(t *testing.T) {
	p := &compactionProgress{replaced: 7, goal: 1024, last: time.Now().Add(-2 * progressInterval)}
	require.Equal(t, "Compacting 7 messages · thinking…", p.advance(0))
}

func TestHumanTokens(t *testing.T) {
	require.Equal(t, "999", humanTokens(999))
	require.Equal(t, "1.0k", humanTokens(1000))
	require.Equal(t, "8.2k", humanTokens(8192))
	require.Equal(t, "0", humanTokens(0))
}
