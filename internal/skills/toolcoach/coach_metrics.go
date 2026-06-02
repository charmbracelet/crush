package toolcoach

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/event"
)

// pendingTip tracks a fired coaching tip and the expected remediation action
// so we can later determine whether the agent acted on the advice.
type pendingTip struct {
	patternID      string
	firedAt        time.Time
	expectedTool   string
	expectedFile   string
	callsRemaining int
	resolved       bool
}

// coachMetrics tracks per-session telemetry for the toolcoach.
type coachMetrics struct {
	toolCallsCoached     atomic.Uint64
	delaySumMicros       atomic.Int64
	delayCount           atomic.Uint64
	totalCoachTimeMicros atomic.Int64
	exported             atomic.Bool

	patternFireCount    map[string]uint64
	patternActedCount   map[string]uint64
	patternIgnoredCount map[string]uint64
	pendingTips         []pendingTip
	mu                  sync.Mutex
}

// newCoachMetrics creates a fresh metrics accumulator.
func newCoachMetrics() *coachMetrics {
	return &coachMetrics{
		patternFireCount:    make(map[string]uint64),
		patternActedCount:   make(map[string]uint64),
		patternIgnoredCount: make(map[string]uint64),
	}
}

// recordToolCall increments the total coached call counter.
func (m *coachMetrics) recordToolCall() {
	m.toolCallsCoached.Add(1)
}

// recordPatternFire increments the fire counter for a pattern and registers a
// pending tip so we can later determine if the agent acted on it.
func (m *coachMetrics) recordPatternFire(patternID, toolName, input string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.patternFireCount[patternID]++

	// Register a pending tip with a heuristic expectation.
	tip := pendingTip{
		patternID:      patternID,
		firedAt:        time.Now(),
		callsRemaining: 3,
	}
	switch patternID {
	case "edit_without_view":
		tip.expectedTool = "view"
		tip.expectedFile, _ = jsonpeek(input, "file_path")
	case "broad_grep":
		tip.expectedTool = "grep"
	case "missing_multiedit":
		tip.expectedTool = "multiedit"
		tip.expectedFile, _ = jsonpeek(input, "file_path")
	case "repeated_view":
		tip.expectedTool = "edit"
		tip.expectedFile, _ = jsonpeek(input, "file_path")
	case "write_over_existing":
		tip.expectedTool = "edit"
		tip.expectedFile, _ = jsonpeek(input, "file_path")
	case "destructive_bash":
		tip.expectedTool = "bash"
	}
	m.pendingTips = append(m.pendingTips, tip)
}

// recordDelay adds a single delay observation to the running average.
func (m *coachMetrics) recordDelay(micros int64) {
	m.delaySumMicros.Add(micros)
	m.delayCount.Add(1)
}

// recordCoachTime accumulates total coach overhead in milliseconds.
func (m *coachMetrics) recordCoachTime(ms float64) {
	m.totalCoachTimeMicros.Add(int64(ms * 1000)) // store as micros for atomic safety
}

// tipValidator is a function that checks whether a tool call resolves a
// pending tip. If nil, the default expected-tool heuristic is used.
type tipValidator func(toolName, input string, tip pendingTip) bool

// checkPendingTips evaluates whether any pending tips were resolved by the
// current tool call. It should be called after recording the tool call.
func (m *coachMetrics) checkPendingTips(toolName, input string, validator tipValidator) {
	m.mu.Lock()
	if len(m.pendingTips) == 0 {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// Parse JSON outside the lock to avoid holding it during I/O.
	var filePath string
	switch toolName {
	case "view", "edit", "write", "multiedit":
		filePath, _ = jsonpeek(input, "file_path")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	remaining := m.pendingTips[:0]
	for _, tip := range m.pendingTips {
		if tip.resolved {
			continue
		}
		tip.callsRemaining--

		resolved := false
		if validator != nil {
			resolved = validator(toolName, input, tip)
		} else if toolName == tip.expectedTool {
			if tip.expectedFile == "" || tip.expectedFile == filePath {
				resolved = true
			}
		}

		if resolved {
			m.patternActedCount[tip.patternID]++
			continue
		}

		if tip.callsRemaining <= 0 {
			m.patternIgnoredCount[tip.patternID]++
			continue
		}

		remaining = append(remaining, tip)
	}

	// If the backing array has grown much larger than the active slice,
	// copy to a new array to prevent memory retention of resolved tips.
	if cap(remaining) > len(remaining)*4 && cap(remaining) > 16 {
		trimmed := make([]pendingTip, len(remaining))
		copy(trimmed, remaining)
		remaining = trimmed
	}
	m.pendingTips = remaining
}

// avgDelayMicros returns the mean coach delay. Returns 0 if no samples.
func (m *coachMetrics) avgDelayMicros() int64 {
	count := m.delayCount.Load()
	if count == 0 {
		return 0
	}
	return m.delaySumMicros.Load() / int64(count)
}

// totalCoachTimeMs returns the cumulative coach time in milliseconds.
func (m *coachMetrics) totalCoachTimeMs() float64 {
	return float64(m.totalCoachTimeMicros.Load()) / 1000.0
}

// patternCounts returns a snapshot of fire/acted/ignored counts per pattern.
func (m *coachMetrics) patternCounts() map[string][3]uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Resolve any remaining pending tips as ignored before snapshotting.
	for _, tip := range m.pendingTips {
		if !tip.resolved {
			m.patternIgnoredCount[tip.patternID]++
		}
	}
	m.pendingTips = nil

	result := make(map[string][3]uint64, len(m.patternFireCount))
	for patternID, fired := range m.patternFireCount {
		result[patternID] = [3]uint64{
			fired,
			m.patternActedCount[patternID],
			m.patternIgnoredCount[patternID],
		}
	}
	return result
}

// export emits a summary event for the session and per-pattern detail events.
// It is safe to call multiple times; only the first call actually exports.
func (m *coachMetrics) export(sessionID string) {
	if sessionID == "" {
		return
	}
	if !m.exported.CompareAndSwap(false, true) {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Resolve any remaining pending tips as ignored.
	for _, tip := range m.pendingTips {
		if !tip.resolved {
			m.patternIgnoredCount[tip.patternID]++
		}
	}
	m.pendingTips = nil

	toolCalls := m.toolCallsCoached.Load()
	totalTime := m.totalCoachTimeMs()
	avgDelay := m.avgDelayMicros()

	slog.Debug("Toolcoach session summary",
		"session_id", sessionID,
		"tool_calls_coached", toolCalls,
		"total_coach_time_ms", totalTime,
		"avg_delay_micros", avgDelay,
		"patterns_fired", fmt.Sprintf("%+v", m.patternFireCount),
		"patterns_acted", fmt.Sprintf("%+v", m.patternActedCount),
		"patterns_ignored", fmt.Sprintf("%+v", m.patternIgnoredCount),
	)

	event.TrackToolcoachSessionSummary(sessionID, int64(toolCalls), totalTime, avgDelay)

	for patternID, count := range m.patternFireCount {
		acted := m.patternActedCount[patternID]
		ignored := m.patternIgnoredCount[patternID]
		event.TrackToolcoachPatternDetail(sessionID, patternID, int64(count), int64(acted), int64(ignored))
	}
}
