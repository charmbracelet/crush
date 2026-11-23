package config

import (
	"context"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

// These tests replace the package-level detector, so none of them runs in
// parallel.

func withAntLogin(t *testing.T, signedIn bool) {
	t.Helper()

	previous := detectAntLogin
	detectAntLogin = func(func(string) string) bool { return signedIn }
	t.Cleanup(func() { detectAntLogin = previous })
}

func configureWithEnv(t *testing.T, vars map[string]string, providers ...catwalk.Provider) *Config {
	t.Helper()

	cfg := &Config{}
	cfg.setDefaults("/tmp", "")
	e := env.NewFromMap(vars)
	require.NoError(t, cfg.configureProviders(context.Background(), testStore(cfg), e, NewShellVariableResolver(e), providers))
	return cfg
}

var (
	anthropicProvider = catwalk.Provider{
		ID:          catwalk.InferenceProviderAnthropic,
		Name:        "Anthropic",
		Type:        catwalk.TypeMessages,
		APIKey:      "$ANTHROPIC_API_KEY",
		APIEndpoint: "https://api.anthropic.com",
		Models:      []catwalk.Model{{ID: "claude"}},
	}
	openaiProvider = catwalk.Provider{
		ID:          catwalk.InferenceProviderOpenAI,
		APIKey:      "$OPENAI_API_KEY",
		APIEndpoint: "https://api.openai.com/v1",
		Models:      []catwalk.Model{{ID: "gpt"}},
	}
)

func TestAntLoginConfiguresAnthropicWithoutAKey(t *testing.T) {
	withAntLogin(t, true)

	cfg := configureWithEnv(t, map[string]string{}, anthropicProvider)

	require.True(t, cfg.IsConfigured(), "a signed-in ant counts as a provider")
	pc, ok := cfg.Providers.Get("anthropic")
	require.True(t, ok)
	require.Equal(t, ExternalAuthAnt, pc.ExternalAuth)
	require.Equal(t, "$ANTHROPIC_API_KEY", pc.APIKey, "no key is invented")
}

func TestAnAnthropicKeyWinsOverTheAntLogin(t *testing.T) {
	withAntLogin(t, true)

	cfg := configureWithEnv(t, map[string]string{"ANTHROPIC_API_KEY": "sk-ant-key"}, anthropicProvider)

	pc, ok := cfg.Providers.Get("anthropic")
	require.True(t, ok)
	require.Empty(t, pc.ExternalAuth)
}

func TestNoKeyAndNoAntLoginLeavesAnthropicUnconfigured(t *testing.T) {
	withAntLogin(t, false)

	cfg := configureWithEnv(t, map[string]string{}, anthropicProvider)

	require.False(t, cfg.IsConfigured())
}

func TestAntLoginOnlyAppliesToAnthropic(t *testing.T) {
	withAntLogin(t, true)

	cfg := configureWithEnv(t, map[string]string{}, openaiProvider)

	require.False(t, cfg.IsConfigured(), "an ant login is no credential for other providers")
}
