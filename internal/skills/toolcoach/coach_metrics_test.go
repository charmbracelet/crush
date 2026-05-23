package toolcoach

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoachMetrics_Basics(t *testing.T) {
	t.Parallel()

	m := newCoachMetrics()

	require.Equal(t, uint64(0), m.toolCallsCoached.Load())
	require.Equal(t, int64(0), m.avgDelayMicros())
	require.Equal(t, 0.0, m.totalCoachTimeMs())

	m.recordToolCall()
	m.recordToolCall()
	require.Equal(t, uint64(2), m.toolCallsCoached.Load())

	m.recordDelay(50)
	m.recordDelay(150)
	require.Equal(t, int64(100), m.avgDelayMicros())

	m.recordCoachTime(1.5)
	require.InDelta(t, 1.5, m.totalCoachTimeMs(), 0.01)
}

func TestCoachMetrics_PatternFireAndResolve(t *testing.T) {
	t.Parallel()

	m := newCoachMetrics()

	// Fire edit_without_view for foo.go.
	m.recordPatternFire("edit_without_view", "edit", `{"file_path":"foo.go"}`)
	require.Equal(t, uint64(1), m.patternFireCount["edit_without_view"])
	require.Equal(t, uint64(0), m.patternActedCount["edit_without_view"])

	// Agent views foo.go → should resolve as acted.
	m.checkPendingTips("view", `{"file_path":"foo.go"}`, nil)
	require.Equal(t, uint64(1), m.patternActedCount["edit_without_view"])
	require.Equal(t, uint64(0), m.patternIgnoredCount["edit_without_view"])
}

func TestCoachMetrics_PatternFireAndIgnore(t *testing.T) {
	t.Parallel()

	m := newCoachMetrics()

	// Fire edit_without_view.
	m.recordPatternFire("edit_without_view", "edit", `{"file_path":"foo.go"}`)

	// Agent does unrelated things 3 times.
	m.checkPendingTips("bash", `{"command":"ls"}`, nil)
	m.checkPendingTips("grep", `{"pattern":"bar"}`, nil)
	m.checkPendingTips("view", `{"file_path":"other.go"}`, nil)

	require.Equal(t, uint64(0), m.patternActedCount["edit_without_view"])
	require.Equal(t, uint64(1), m.patternIgnoredCount["edit_without_view"])
}

func TestCoachMetrics_ExportClearsPending(t *testing.T) {
	t.Parallel()

	m := newCoachMetrics()
	m.recordPatternFire("edit_without_view", "edit", `{"file_path":"foo.go"}`)

	// Export without resolution should count as ignored.
	m.export("session-1")
	require.Equal(t, uint64(0), m.patternActedCount["edit_without_view"])
	require.Equal(t, uint64(1), m.patternIgnoredCount["edit_without_view"])
	require.Empty(t, m.pendingTips)
}

func TestCoachMetrics_MultiplePending(t *testing.T) {
	t.Parallel()

	m := newCoachMetrics()

	m.recordPatternFire("edit_without_view", "edit", `{"file_path":"a.go"}`)
	m.recordPatternFire("broad_grep", "grep", `{"pattern":".*"}`)
	m.recordPatternFire("write_over_existing", "write", `{"file_path":"b.go"}`)

	// Agent acts on edit_without_view and write_over_existing but not broad_grep.
	m.checkPendingTips("view", `{"file_path":"a.go"}`, nil)
	m.checkPendingTips("edit", `{"file_path":"b.go"}`, nil)
	m.checkPendingTips("bash", `{"command":"echo hi"}`, nil)

	require.Equal(t, uint64(1), m.patternActedCount["edit_without_view"])
	require.Equal(t, uint64(1), m.patternActedCount["write_over_existing"])
	require.Equal(t, uint64(0), m.patternActedCount["broad_grep"])
	require.Equal(t, uint64(1), m.patternIgnoredCount["broad_grep"])
}

// BenchmarkCoachLatencyThreshold is a performance regression gate. If
// runCoach() p95 exceeds 10µs over 1000 iterations, the benchmark fails.
//
// To run: go test -bench=BenchmarkCoachLatencyThreshold -count=5
func BenchmarkCoachLatencyThreshold(b *testing.B) {
	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}
	input := `{"file_path":"foo.go","old_string":"x","new_string":"y"}`

	// Warm-up.
	for i := 0; i < 1000; i++ {
		state.runCoach(cfg, CoachingTutor, "edit", input)
	}

	// Collect samples.
	delays := make([]int64, 0, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		state.runCoach(cfg, CoachingTutor, "edit", input)
		delays = append(delays, time.Since(start).Microseconds())
	}

	if b.N < 1000 {
		return // Not enough samples for a reliable p95.
	}

	sort.Slice(delays, func(i, j int) bool { return delays[i] < delays[j] })
	p95Idx := int(float64(len(delays)) * 0.95)
	p95 := delays[p95Idx]

	const p95ThresholdMicros = int64(10)
	if p95 > p95ThresholdMicros {
		b.Fatalf("p95 latency %dµs exceeds threshold %dµs", p95, p95ThresholdMicros)
	}
	b.Logf("p95 latency: %dµs (threshold: %dµs)", p95, p95ThresholdMicros)
}
