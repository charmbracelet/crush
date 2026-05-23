package toolcoach

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionState_FileTracking(t *testing.T) {
	t.Parallel()

	state := newSessionState()

	require.False(t, state.hasViewed("foo.go"))
	require.False(t, state.hasEdited("foo.go"))

	state.trackFileAccess("view", `{"file_path":"foo.go"}`)
	require.True(t, state.hasViewed("foo.go"))
	require.False(t, state.hasEdited("foo.go"))
	require.Equal(t, 1, state.viewCount("foo.go"))

	state.trackFileAccess("edit", `{"file_path":"foo.go"}`)
	require.True(t, state.hasEdited("foo.go"))
}

func TestSessionState_ConsecutiveEdits(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	require.Equal(t, 0, state.consecutiveEdits("foo.go"))

	state.recordToolCall("edit", `{"file_path":"foo.go"}`)
	require.Equal(t, 1, state.consecutiveEdits("foo.go"))

	state.recordToolCall("edit", `{"file_path":"foo.go"}`)
	require.Equal(t, 2, state.consecutiveEdits("foo.go"))

	state.recordToolCall("view", `{"file_path":"foo.go"}`)
	require.Equal(t, 0, state.consecutiveEdits("foo.go"))
}

func TestSessionState_Timing(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	require.Equal(t, time.Duration(0), state.totalTime())

	state.addCoachTime(150 * time.Microsecond)
	require.Equal(t, 150*time.Microsecond, state.totalTime())

	state.addCoachTime(50 * time.Microsecond)
	require.Equal(t, 200*time.Microsecond, state.totalTime())
}

func TestSessionState_TurnCounters(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	require.Equal(t, 0, state.patternsFiredThisTurn())

	state.incrementPatternsFired()
	state.incrementPatternsFired()
	require.Equal(t, 2, state.patternsFiredThisTurn())

	state.resetTurnCounters()
	require.Equal(t, 0, state.patternsFiredThisTurn())
}

func TestRunCoach_DelayMicros(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	result := state.runCoach(cfg, CoachingTutor, "edit", `{"file_path":"foo.go","old_string":"x","new_string":"y"}`)
	require.NotNil(t, result)
	require.GreaterOrEqual(t, result.DelayMicros, int64(0))
	// Heuristics should be well under 1ms (1000µs) in normal builds.
	// Under the race detector overhead can spike, so we only assert
	// a loose ceiling here; the benchmark verifies real performance.
	require.Less(t, result.DelayMicros, int64(5000))
}

func BenchmarkRunCoach(b *testing.B) {
	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}
	input := `{"file_path":"foo.go","old_string":"x","new_string":"y"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.runCoach(cfg, CoachingTutor, "edit", input)
	}
}
