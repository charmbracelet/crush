package driver

import (
	"bytes"
	"time"
)

// Convert a string in [time.RFC3339Nano] format into a [time.Time]
// if it roundtrips back to the same string.
// This way times can be persisted to, and recovered from, the database,
// but if a string is needed, [database/sql] will recover the same string.
func maybeTime(text []byte) (_ time.Time, _ bool) {
	// Weed out (some) values that can't possibly be
	// [time.RFC3339Nano] timestamps.
	if len(text) < len("2006-01-02T15:04:05Z") {
		return
	}
	if len(text) > len(time.RFC3339Nano) {
		return
	}
	if text[4] != '-' || text[10] != 'T' || text[16] != ':' {
		return
	}

	// Slow path.
	var buf [len(time.RFC3339Nano)]byte
	date, err := time.Parse(time.RFC3339Nano, string(text))
	if err == nil && bytes.Equal(text, date.AppendFormat(buf[:0], time.RFC3339Nano)) {
		return date, true
	}
	return
}
