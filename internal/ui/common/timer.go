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
	started map[string]time.Time
}{started: map[string]time.Time{}}

// StartTurn begins tracking elapsed time for sessionID's turn. A session
// that is already timing keeps its original start, so a prompt queued
// behind a running turn continues to count from the turn in flight rather
// than restarting mid-turn.
func StartTurn(sessionID string) {
	if sessionID == "" {
		return
	}
	turnClock.mu.Lock()
	defer turnClock.mu.Unlock()
	if _, timing := turnClock.started[sessionID]; timing {
		return
	}
	turnClock.started[sessionID] = time.Now()
}

// StopTurn stops tracking sessionID's turn.
func StopTurn(sessionID string) {
	turnClock.mu.Lock()
	defer turnClock.mu.Unlock()
	delete(turnClock.started, sessionID)
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

	elapsed := time.Since(startTime)
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
