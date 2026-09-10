package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/keyring"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// TestMakeAuthRefreshCallback pins which provider configurations fantasy
// receives an OnAuthRefresh callback for. The keychain case matters because
// a keychain:// reference contains no "$", so the historical shell-template
// check alone would leave providers whose keys live in the OS keychain
// without any 401 recovery path.
func TestMakeAuthRefreshCallback(t *testing.T) {
	tests := []struct {
		name     string
		provider config.ProviderConfig
		wantNil  bool
	}{
		{
			name:     "keychain reference",
			provider: config.ProviderConfig{APIKeyTemplate: keyring.Ref("mock")},
		},
		{
			name:     "shell template",
			provider: config.ProviderConfig{APIKeyTemplate: "$OPENAI_API_KEY"},
		},
		{
			name:     "oauth token",
			provider: config.ProviderConfig{OAuthToken: &oauth.Token{AccessToken: "tok"}},
		},
		{
			name:     "aws auth refresh",
			provider: config.ProviderConfig{AWSAuthRefresh: "aws sso login"},
		},
		{
			name:     "static key",
			provider: config.ProviderConfig{APIKey: "sk-static", APIKeyTemplate: "sk-static"},
			wantNil:  true,
		},
	}

	coord := &coordinator{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callback := coord.makeAuthRefreshCallback(tt.provider)
			if tt.wantNil {
				require.Nil(t, callback)
			} else {
				require.NotNil(t, callback)
			}
		})
	}
}

// newKeychainTestCoordinator builds a hermetic coordinator whose only
// provider stores its API key in the (mocked) OS keychain and references it
// from the config file with a keychain:// URI. It uses a known catalog
// provider (openai) because only the known-provider load path keeps the raw
// key template in APIKeyTemplate for 401 re-resolution; the catalog is
// static and model discovery never runs for known providers, so no network
// access happens.
//
// The tests using this helper must not run in parallel: MockInit swaps a
// process-global keyring backend.
func newKeychainTestCoordinator(t *testing.T, secret string) (*coordinator, *config.ConfigStore) {
	t.Helper()

	keyring.MockInit()
	require.NoError(t, keyring.Set("openai", secret))

	env := testEnv(t)

	crushJSON := `{
  "options": {"disable_provider_auto_update": true},
  "providers": {"openai": {"id": "openai", "base_url": "http://127.0.0.1:9/v1",
    "api_key": "keychain://openai",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "openai", "model": "mock-model"},
             "small": {"provider": "openai", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	cfg.SetupAgents()

	coord := &coordinator{
		cfg:         cfg,
		sessions:    env.sessions,
		messages:    env.messages,
		permissions: env.permissions,
		history:     env.history,
		filetracker: *env.filetracker,
		agents:      make(map[string]SessionAgent),
		interactive: true,
	}

	p, err := coderPrompt(prompt.WithWorkingDir(env.workingDir))
	require.NoError(t, err)
	agentCfg := cfg.Config().Agents[config.AgentCoder]

	agent, err := coord.buildAgent(context.Background(), p, agentCfg, false)
	require.NoError(t, err)
	coord.currentAgent = agent
	coord.agents[config.AgentCoder] = agent

	// buildAgent resolves the keychain secret on background readiness
	// goroutines. The mock keyring backend is unsynchronized, so those
	// reads must finish before the tests mutate the mock store.
	require.NoError(t, coord.readyWg.Wait())

	return coord, cfg
}

// TestRetryAfterUnauthorizedReResolvesKeychainRef covers the 401 recovery
// path for keys stored in the OS keychain: when the provider reports the key
// as invalid (typically because it was rotated), Crush must re-read the
// secret from the keychain and rebuild the provider with the fresh value
// instead of surfacing the auth error.
func TestRetryAfterUnauthorizedReResolvesKeychainRef(t *testing.T) {
	coord, cfg := newKeychainTestCoordinator(t, "key-v1")

	providerCfg, ok := cfg.Config().Providers.Get("openai")
	require.True(t, ok)
	require.True(t, keyring.IsRef(providerCfg.APIKeyTemplate),
		"config load must keep the keychain reference for re-resolution")

	require.NotNil(t, coord.makeAuthRefreshCallback(providerCfg),
		"keychain-backed providers need an auth refresh callback")

	// Simulate the user rotating the key in the OS keychain while Crush
	// is running.
	require.NoError(t, keyring.Set("openai", "key-v2"))

	require.NoError(t, coord.retryAfterUnauthorized(t.Context(), providerCfg))

	updated, ok := cfg.Config().Providers.Get("openai")
	require.True(t, ok)
	require.Equal(t, "key-v2", updated.APIKey,
		"the rotated keychain secret must be re-resolved into the provider")
	require.True(t, keyring.IsRef(updated.APIKeyTemplate),
		"the keychain reference must survive re-resolution")
}

// TestRetryAfterUnauthorizedKeychainEntryMissing covers the failure mode
// where the keychain entry disappeared after startup (user deleted it, or a
// different keyring is attached): the refresh must fail with an error so
// fantasy surfaces the original 401, and the provider keeps its last known
// configuration.
func TestRetryAfterUnauthorizedKeychainEntryMissing(t *testing.T) {
	coord, cfg := newKeychainTestCoordinator(t, "key-v1")

	providerCfg, ok := cfg.Config().Providers.Get("openai")
	require.True(t, ok)

	require.NoError(t, keyring.Delete("openai"))

	err := coord.retryAfterUnauthorized(t.Context(), providerCfg)
	require.Error(t, err)

	unchanged, ok := cfg.Config().Providers.Get("openai")
	require.True(t, ok)
	require.True(t, keyring.IsRef(unchanged.APIKeyTemplate))
}
