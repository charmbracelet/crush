package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAutoSummarizeThreshold(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		contextWindow int64
		ratio         float64
		buffer        int64
		want          int64
	}{
		{"large window keeps the flat buffer", 400_000, 0, 0, largeContextWindowBuffer},
		{"small window reserves a fifth", 100_000, 0, 0, 20_000},
		{"200k is not large enough for the buffer", 200_000, 0, 0, 40_000},
		{"buffer replaces the flat default", 400_000, 0, 40_000, 40_000},
		{"buffer leaves small windows alone", 100_000, 0, 40_000, 20_000},
		{"ratio replaces the small window default", 100_000, 0.3, 0, 30_000},
		{"ratio leaves large windows alone", 400_000, 0.3, 0, largeContextWindowBuffer},
		{"each setting covers its own regime", 400_000, 0.3, 40_000, 40_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, autoSummarizeThreshold(tc.contextWindow, tc.ratio, tc.buffer))
		})
	}
}
