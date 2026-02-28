package muse

import (
	"fmt"
	"time"
)

// FormatCountdown formats a duration as MM:SS or SS.
func FormatCountdown(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if minutes > 0 {
		return fmt.Sprintf("%d:%02d", minutes, seconds)
	}
	return fmt.Sprintf("%d", seconds)
}

// PlaceholderText generates the placeholder text for the textarea.
// It shows countdown to next Muse trigger and Yolo status.
func (m *Muse) PlaceholderText(elapsed time.Duration, yoloEnabled bool, defaultPlaceholder string) string {
	var placeholder string

	if yoloEnabled {
		placeholder = "Yolo mode!"
	}

	if m.enabled {
		var remaining time.Duration
		if m.lastTrigger.IsZero() {
			// Before first trigger: countdown to timeout
			remaining = m.timeout - elapsed
		} else {
			// After first trigger: countdown to next interval
			if m.interval > 0 {
				remaining = m.interval - time.Since(m.lastTrigger)
				if remaining < 0 {
					remaining = 0
				}
			}
		}
		if remaining > 0 {
			if placeholder != "" {
				placeholder += " "
			}
			placeholder += "Muse in " + FormatCountdown(remaining)
		}
	}

	if placeholder == "" {
		return defaultPlaceholder
	}
	return placeholder
}
