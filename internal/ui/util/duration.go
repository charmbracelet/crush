package util

import (
	"fmt"
	"strings"
)

// FormatDuration formats a duration in seconds to a human-readable string.
// Supports days, hours, minutes, and seconds.
// Examples:
//
//	30s
//	5m
//	1h30m
//	2d12h45m30s
//	7d
func FormatDuration(seconds int) string {
	if seconds == 0 {
		return "0s"
	}

	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60
	seconds %= 60

	parts := []string{}
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, "")
}
