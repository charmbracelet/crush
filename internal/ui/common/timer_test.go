package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestElapsedSince(t *testing.T) {
	t.Parallel()

	base := time.Unix(1_800_000_000, 0)
	at := func(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }

	tests := []struct {
		name    string
		started time.Time
		now     time.Time
		want    time.Duration
	}{
		{"same second", at(100), at(900), 0},
		{"crossing a second boundary", at(900), at(1100), time.Second},
		{"late start crosses with early start", at(137), at(3000), 3 * time.Second},
		{"holds until the next boundary", at(905), at(1999), time.Second},
		{"clock stepped backwards", at(5000), at(1000), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, ElapsedSince(tt.started, tt.now))
		})
	}
}

// Timers with different sub-second start phases must change on the same
// frame, which is the whole point of snapping both ends.
func TestElapsedSinceTicksTogether(t *testing.T) {
	t.Parallel()

	base := time.Unix(1_800_000_000, 0)
	starts := []time.Time{
		base.Add(137 * time.Millisecond),
		base.Add(612 * time.Millisecond),
		base.Add(905 * time.Millisecond),
	}
	for now := base.Add(time.Second); now.Before(base.Add(4 * time.Second)); now = now.Add(53 * time.Millisecond) {
		next := now.Add(53 * time.Millisecond)
		changed := 0
		for _, s := range starts {
			if ElapsedSince(s, now) != ElapsedSince(s, next) {
				changed++
			}
		}
		require.Contains(t, []int{0, len(starts)}, changed, "timers diverged between %v and %v", now, next)
	}
}
