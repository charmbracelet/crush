package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		seconds  int
		expected string
	}{
		{
			name:     "zero",
			seconds:  0,
			expected: "0s",
		},
		{
			name:     "seconds only",
			seconds:  30,
			expected: "30s",
		},
		{
			name:     "one minute",
			seconds:  60,
			expected: "1m",
		},
		{
			name:     "minute and seconds",
			seconds:  90,
			expected: "1m30s",
		},
		{
			name:     "5 minutes",
			seconds:  300,
			expected: "5m",
		},
		{
			name:     "one hour",
			seconds:  3600,
			expected: "1h",
		},
		{
			name:     "hour and minutes",
			seconds:  3665,
			expected: "1h1m5s",
		},
		{
			name:     "one day",
			seconds:  86400,
			expected: "1d",
		},
		{
			name:     "day and time",
			seconds:  90061,
			expected: "1d1h1m1s",
		},
		{
			name:     "one week (max interval)",
			seconds:  604800,
			expected: "7d",
		},
		{
			name:     "mixed units",
			seconds:  93785,
			expected: "1d2h3m5s",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FormatDuration(tt.seconds)
			require.Equal(t, tt.expected, result)
		})
	}
}
