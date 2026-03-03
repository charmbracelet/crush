package muse

import (
	"time"

	"github.com/charmbracelet/crush/internal/ui/util"
)

// PlaceholderText generates the placeholder text for the textarea.
// It shows countdown to next Muse trigger and Yolo status.
// hasSession is required - countdown is only shown when there's an active session.
// isBusy prevents countdown from showing when agent is active.
func (m *Muse) PlaceholderText(elapsed time.Duration, yoloEnabled, hasSession, isBusy bool, defaultPlaceholder string) string {
	var placeholder string

	if yoloEnabled {
		placeholder = "Yolo mode!"
	}

	// Only show Muse countdown if there's an active session and agent is not busy
	if m.enabled && hasSession && !isBusy {
		// Countdown to interval
		remaining := m.interval - elapsed
		if remaining > 0 {
			if placeholder != "" {
				placeholder += " "
			}
			placeholder += "Muse in " + util.FormatDuration(int(remaining.Seconds()))
		}
	}

	if placeholder == "" {
		return defaultPlaceholder
	}
	return placeholder
}
