package config

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestOptionsGetRequestTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		options  *Options
		expected time.Duration
	}{
		{
			name:     "nil options",
			options:  nil,
			expected: DefaultRequestTimeout,
		},
		{
			name:     "unset field",
			options:  &Options{},
			expected: DefaultRequestTimeout,
		},
		{
			name:     "disabled",
			options:  &Options{RequestTimeout: ptr(0)},
			expected: 0,
		},
		{
			name:     "negative disables too",
			options:  &Options{RequestTimeout: ptr(-1)},
			expected: 0,
		},
		{
			name:     "seconds to duration",
			options:  &Options{RequestTimeout: ptr(300)},
			expected: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, tt.options.GetRequestTimeout())
		})
	}
}

func TestOptionsRequestTimeoutFromJSON(t *testing.T) {
	t.Parallel()

	var cfg Config
	require.NoError(t, json.Unmarshal([]byte(`{"options":{"request_timeout":300}}`), &cfg))
	require.Equal(t, 300, *cfg.Options.RequestTimeout)
	require.Equal(t, 5*time.Minute, cfg.Options.GetRequestTimeout())
}
