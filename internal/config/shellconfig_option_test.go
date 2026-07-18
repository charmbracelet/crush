package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShellConfigOptionBooleans(t *testing.T) {
	store := loadCrushSh(t, `option debug true
option progress false
option auto-lsp false`)

	opts := store.Config().Options
	require.True(t, opts.Debug, "debug should be on")
	require.NotNil(t, opts.Progress)
	require.False(t, *opts.Progress, "progress should be off")
	require.NotNil(t, opts.AutoLSP)
	require.False(t, *opts.AutoLSP, "auto-lsp should be off")
}

// Config phrases this field negatively (disable_metrics) but the command
// exposes it positively. "metrics false" must land as disable_metrics = true.
func TestShellConfigOptionPositiveMetricsFalse(t *testing.T) {
	store := loadCrushSh(t, `option metrics false`)
	require.True(t, store.Config().Options.DisableMetrics, "metrics off => disable_metrics true")
}

// The bare positive form defaults to true, which inverts to disable = false.
func TestShellConfigOptionPositiveMetricsBare(t *testing.T) {
	store := loadCrushSh(t, `option metrics`)
	require.False(t, store.Config().Options.DisableMetrics, "metrics on => disable_metrics false")
}

func TestShellConfigOptionListAppends(t *testing.T) {
	store := loadCrushSh(t, `option disable-tool bash
option disable-tool sourcegraph`)

	require.Subset(t, store.Config().Options.DisabledTools, []string{"bash", "sourcegraph"})
}

// reset wipes values added earlier (or via source) while keeping anything
// added after it — observable in the effective config.
func TestShellConfigOptionReset(t *testing.T) {
	store := loadCrushSh(t, `option skill-path ./inherited-a
option skill-path ./inherited-b
option reset skill-path
option skill-path ./mine`)

	paths := store.Config().Options.SkillsPaths
	require.Contains(t, paths, "./mine")
	require.NotContains(t, paths, "./inherited-a")
	require.NotContains(t, paths, "./inherited-b")
}

func TestShellConfigOptionUnknownKeyFails(t *testing.T) {
	_, err := loadCrushShErr(t, `option bogus-key value`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown key")
}

func TestShellConfigOptionResetRejectsNonList(t *testing.T) {
	_, err := loadCrushShErr(t, `option reset debug`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not one")
}
