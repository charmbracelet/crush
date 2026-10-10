package config

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/shell"

	"github.com/stretchr/testify/require"
)

func TestOptionsGetMaxBackgroundJobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		options  *Options
		expected int
	}{
		{"nil options", nil, shell.DefaultMaxBackgroundJobs},
		{"unset", &Options{}, shell.DefaultMaxBackgroundJobs},
		{"zero means default", &Options{MaxBackgroundJobs: ptr(0)}, shell.DefaultMaxBackgroundJobs},
		{"negative means default", &Options{MaxBackgroundJobs: ptr(-5)}, shell.DefaultMaxBackgroundJobs},
		{"configured", &Options{MaxBackgroundJobs: ptr(200)}, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, tt.options.GetMaxBackgroundJobs())
		})
	}
}

func TestOptionsMaxBackgroundJobsFromJSON(t *testing.T) {
	t.Parallel()

	var cfg Config
	require.NoError(t, json.Unmarshal([]byte(`{"options":{"max_background_jobs":10}}`), &cfg))
	require.Equal(t, 10, cfg.Options.GetMaxBackgroundJobs())
}
