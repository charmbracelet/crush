package common

import (
	"fmt"
	"sync"
	"time"
)

// turnClock tracks when each session's current turn started. Turns in
// different sessions run concurrently, so one clock per session: a turn
// finishing in one session must not reset the elapsed time shown in
// another.
var turnClock = struct {
	mu      sync.Mutex
	current string
	started map[string]time.Time
}{started: map[string]time.Time{}}

// StartTurn starts sessionID's turn clock from now, replacing any earlier
// start.
func StartTurn(sessionID string) {
	if sessionID == "" {
		return
	}
	turnClock.mu.Lock()
	defer turnClock.mu.Unlock()
	turnClock.started[sessionID] = time.Now()
	turnClock.current = sessionID
}

// StopTurn stops tracking sessionID's turn.
func StopTurn(sessionID string) {
	turnClock.mu.Lock()
	defer turnClock.mu.Unlock()
	delete(turnClock.started, sessionID)
	if turnClock.current == sessionID {
		turnClock.current = ""
	}
}

// CurrentElapsed returns the formatted elapsed time for the most recently
// started turn still running, or an empty string when no turn is running.
// Render paths that show a live timer but do not know which session they
// belong to, such as in-flight tool calls, use this instead of Elapsed.
func CurrentElapsed() string {
	turnClock.mu.Lock()
	sessionID := turnClock.current
	turnClock.mu.Unlock()
	if sessionID == "" {
		return ""
	}
	return Elapsed(sessionID)
}

// Elapsed returns the formatted elapsed time for sessionID's turn, or an
// empty string when that session is not timing one.
func Elapsed(sessionID string) string {
	turnClock.mu.Lock()
	startTime, timing := turnClock.started[sessionID]
	turnClock.mu.Unlock()
	if !timing {
		return ""
	}

	return FormatDuration(time.Since(startTime))
}

// FormatDuration formats a duration as a human friendly string.
func FormatDuration(elapsed time.Duration) string {
	totalSeconds := int(elapsed.Seconds())
	minutes := int(elapsed.Minutes())
	hours := int(elapsed.Hours())

	switch {
	case hours >= 1:
		return fmt.Sprintf("%dh %dm", hours, minutes%60)
	case minutes >= 1:
		return fmt.Sprintf("%dm %ds", minutes, totalSeconds%60)
	default:
		return fmt.Sprintf("%ds", totalSeconds)
	}
}
