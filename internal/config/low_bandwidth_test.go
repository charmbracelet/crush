package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLowBandwidthEnabled verifies the accessor handles every
// nil-pointer permutation gracefully \u2014 callers reach for it before any
// runtime guarantees that the TUI struct exists.
func TestLowBandwidthEnabled(t *testing.T) {
	t.Parallel()

	tt := func(v bool) *bool { return &v }

	tests := []struct {
		name string
		cfg  *Config
		want bool
	}{
		{name: "nil config", cfg: nil, want: false},
		{name: "nil options", cfg: &Config{}, want: false},
		{name: "nil tui", cfg: &Config{Options: &Options{}}, want: false},
		{name: "nil low_bandwidth pointer", cfg: &Config{Options: &Options{TUI: &TUIOptions{}}}, want: false},
		{name: "explicit false", cfg: &Config{Options: &Options{TUI: &TUIOptions{LowBandwidth: tt(false)}}}, want: false},
		{name: "explicit true", cfg: &Config{Options: &Options{TUI: &TUIOptions{LowBandwidth: tt(true)}}}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.cfg.LowBandwidthEnabled())
		})
	}
}

// TestLowBandwidthEnvVar exercises the CRUSH_LOW_BANDWIDTH override
// path. setDefaults() should respect a truthy env value regardless of
// what the persisted config says.
func TestLowBandwidthEnvVar(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{name: "1 enables", env: "1", want: true},
		{name: "true enables", env: "true", want: true},
		{name: "0 disables", env: "0", want: false},
		{name: "false disables", env: "false", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Cannot t.Parallel \u2014 mutates env.
			t.Setenv("CRUSH_LOW_BANDWIDTH", tc.env)

			cfg := &Config{}
			cfg.setDefaults(t.TempDir(), t.TempDir())
			require.Equal(t, tc.want, cfg.LowBandwidthEnabled())
		})
	}
}

// TestLowBandwidthEnvVar_Unset_FallsBackToConfig confirms the env var
// only acts as an override; without it the persisted config wins.
func TestLowBandwidthEnvVar_Unset_FallsBackToConfig(t *testing.T) {
	t.Setenv("CRUSH_LOW_BANDWIDTH", "")

	tt := true
	cfg := &Config{Options: &Options{TUI: &TUIOptions{LowBandwidth: &tt}}}
	cfg.setDefaults(t.TempDir(), t.TempDir())
	require.True(t, cfg.LowBandwidthEnabled())
}
