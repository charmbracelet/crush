package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config/trust"
	"github.com/stretchr/testify/require"
)

// TestFilterProjectConfigsByTrust covers how project configs are filtered
// by trust decisions: unknown and rejected configs are excluded, trusted
// configs are kept, and a changed config (even a previously rejected one)
// becomes unknown again.
func TestFilterProjectConfigsByTrust(t *testing.T) {
	t.Setenv("CRUSH_TRUST_ALL", "")

	dataDir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_DATA", dataDir)
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())

	workDir := t.TempDir()
	projectConfig := filepath.Join(workDir, "crush.json")
	require.NoError(t, os.WriteFile(projectConfig, []byte(`{"options":{}}`), 0o644))

	trustStore := trust.New()
	require.NoError(t, trustStore.Load())

	configPaths := lookupConfigs(workDir)

	// Unknown config: filtered out and flagged for a trust prompt.
	filtered, untrusted := filterProjectConfigsByTrust(trustStore, workDir, configPaths)
	require.Equal(t, []string{projectConfig}, untrusted)
	require.NotContains(t, filtered, projectConfig)

	// Trusted config: kept, no prompt needed.
	require.NoError(t, trustStore.Trust(projectConfig))
	filtered, untrusted = filterProjectConfigsByTrust(trustStore, workDir, configPaths)
	require.Empty(t, untrusted)
	require.Contains(t, filtered, projectConfig)

	// Rejected config: filtered out without a prompt.
	require.NoError(t, trustStore.Reject(projectConfig))
	filtered, untrusted = filterProjectConfigsByTrust(trustStore, workDir, configPaths)
	require.Empty(t, untrusted)
	require.NotContains(t, filtered, projectConfig)

	// Changed rejected config: unknown again, prompts once more.
	require.NoError(t, os.WriteFile(projectConfig, []byte(`{"options":{"progress":true}}`), 0o644))
	filtered, untrusted = filterProjectConfigsByTrust(trustStore, workDir, configPaths)
	require.Equal(t, []string{projectConfig}, untrusted)
	require.NotContains(t, filtered, projectConfig)
}

// TestProjectTrustDecisions exercises the ConfigStore trust API end to
// end: rejecting records a "no" that suppresses future prompts,
// re-prompting clears the previous decision, and accepting persists a
// "yes" and reloads the config so the trusted file takes effect.
func TestProjectTrustDecisions(t *testing.T) {
	t.Setenv("CRUSH_TRUST_ALL", "")

	dataDir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_DATA", dataDir)
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	resetProviderState()
	t.Cleanup(resetProviderState)

	workDir := t.TempDir()
	projectConfig := filepath.Join(workDir, "crush.json")
	require.NoError(t, os.WriteFile(projectConfig, []byte(twoProviderConfig("openai", "gpt-4")), 0o600))

	store, err := Load(workDir, dataDir, false)
	require.NoError(t, err)

	// The unknown project config is awaiting a trust decision.
	require.True(t, store.HasUntrustedProjectConfigs())
	require.Equal(t, []string{projectConfig}, store.UntrustedProjectPaths())

	// Rejecting records a "no" for the current content and stops the
	// prompts.
	require.NoError(t, store.RejectProjectTrust())
	require.False(t, store.HasUntrustedProjectConfigs())
	require.Equal(t, []string{projectConfig}, store.RejectedProjectPaths())
	require.Empty(t, store.TrustedProjectPaths())

	// Re-prompting clears the stored "no" and marks the config as
	// awaiting a decision again.
	require.NoError(t, store.PromptProjectTrust(context.Background(), []string{projectConfig}))
	require.True(t, store.HasUntrustedProjectConfigs())
	require.Empty(t, store.RejectedProjectPaths())

	// Accepting records a "yes" and reloads the config so the trusted
	// file is applied.
	require.NoError(t, store.AcceptProjectTrust(context.Background()))
	require.False(t, store.HasUntrustedProjectConfigs())
	require.Equal(t, []string{projectConfig}, store.TrustedProjectPaths())
	require.Equal(t, "openai", store.Config().Models[SelectedModelTypeLarge].Provider)
}
