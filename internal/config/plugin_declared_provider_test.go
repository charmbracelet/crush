package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestDeclaredOAuthProviderPlugin loads a plugin that declares a
// subscription-style provider: the OAuth flow, the quota report, the
// per-family limits, and the model list. Everything a provider needs has to
// be expressible this way, with no helper for any specific one.
func TestDeclaredOAuthProviderPlugin(t *testing.T) {
	// Isolate from the developer's real config so only the plugin under test
	// contributes. No t.Parallel(): these tests set env vars.
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))
	t.Setenv("EXAMPLE_CLIENT_SECRET", "shared-secret")

	workDir := t.TempDir()
	writePluginScript(t, workDir, "example.sh", `#!/usr/bin/env bash
provider add example-sub \
  --name "Example Subscription" \
  --type anthropic \
  --base-url "https://api.example.com" \
  --flat-rate true \
  --extra-header anthropic-beta "oauth-2025-04-20" \
  --usage-url "https://api.example.com/v1internal:retrieveUserQuotaSummary" \
  --usage-method POST \
  --usage-groups groups \
  --usage-model-group gemini "Gemini" \
  --usage-model-group cld "Claude" \
  --oauth-flow browser \
  --oauth-client-id "public-client-id" \
  --oauth-client-secret "${EXAMPLE_CLIENT_SECRET:-}" \
  --oauth-auth-url "https://auth.example.com/oauth2/authorize" \
  --oauth-token-url "https://auth.example.com/oauth2/token" \
  --oauth-redirect-uri "http://localhost:0/oauth2callback" \
  --oauth-param access_type offline \
  --oauth-scope user:inference \
  --oauth-scope user:profile \
  --gateway-request '(.url as $u | {url: ($u + "?rewritten"), drop_headers: ["x-goog-api-key"]})' \
  --gateway-response '(.body.response // .body)' \
  --discover-models false

model add example-sub/gemini-1-pro \
  --name "Gemini 1 Pro" \
  --context-window 1048576 --default-max-tokens 65536 \
  --can-reason true --supports-images true \
  --reasoning-level low --reasoning-level medium --reasoning-level high \
  --reasoning-effort medium

model add example-sub/cld-sonnet-9 \
  --name "Claude Sonnet 9" \
  --context-window 1000000 --default-max-tokens 128000 \
  --can-reason true --supports-images true \
  --reasoning-level low --reasoning-level max \
  --reasoning-effort max
`)

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)

	pc, ok := store.Config().Providers.Get("example-sub")
	require.True(t, ok, "the plugin should register the subscription provider")
	require.Equal(t, "Example Subscription", pc.Name)
	require.Equal(t, "anthropic", string(pc.Type))
	require.Equal(t, "https://api.example.com", pc.BaseURL)
	require.True(t, pc.FlatRate, "a flat monthly plan must not accumulate per-token cost")
	require.Equal(t, "oauth-2025-04-20", pc.ExtraHeaders["anthropic-beta"])

	require.True(t, pc.UsesOAuth(), "the OAuth block is what makes crush login work")
	require.Equal(t, "browser", pc.Auth.FlowMode())
	require.Equal(t, "public-client-id", pc.Auth.ClientID)
	require.Equal(t, "shared-secret", pc.Auth.ClientSecret,
		"an env-supplied secret expands into the spec, so no plugin has to carry one")
	require.Equal(t, "https://auth.example.com/oauth2/authorize", pc.Auth.AuthorizeURL)
	require.Equal(t, "http://localhost:0/oauth2callback", pc.Auth.RedirectURI)
	require.Equal(t, map[string]string{"access_type": "offline"}, pc.Auth.ExtraParams)
	require.Equal(t, []string{"user:inference", "user:profile"}, pc.Auth.Scopes)

	require.NotNil(t, pc.Usage, "the plugin declares where the plan's remaining quota is read")
	require.Equal(t, "POST", pc.Usage.Method)
	require.Equal(t, "groups", pc.Usage.Groups)
	require.Equal(t, "buckets", pc.Usage.MetersPath())
	require.Equal(t, map[string]string{"gemini": "Gemini", "cld": "Claude"}, pc.Usage.ModelGroups)

	// The gateway adapter carries the provider's wire format, so a provider
	// whose gateway differs from its SDK is configuration rather than code.
	require.NotNil(t, pc.Gateway)
	require.Contains(t, pc.Gateway.Request, "drop_headers")
	require.Equal(t, "(.body.response // .body)", pc.Gateway.Response)

	require.Len(t, pc.Models, 2)
	var pro *catwalk.Model
	for i := range pc.Models {
		if pc.Models[i].ID == "gemini-1-pro" {
			pro = &pc.Models[i]
		}
	}
	require.NotNil(t, pro)
	require.Equal(t, []string{"low", "medium", "high"}, pro.ReasoningLevels)
	require.Equal(t, "medium", pro.DefaultReasoningEffort)
	require.Equal(t, int64(1048576), pro.ContextWindow)

	// Unsigned-in, the provider is configured but cannot serve: its models
	// belong in the picker behind a sign-in prompt, while default selection
	// and agent setup skip it, because building a client with no credential
	// would fail startup.
	require.True(t, store.Config().IsConfigured(), "a signed-out provider still counts as configured")
	require.False(t, store.Config().HasUsableSelection())
	var usable bool
	for _, p := range store.Config().UsableProviders() {
		if p.ID == "example-sub" {
			usable = true
		}
	}
	require.False(t, usable, "a provider waiting on a sign-in must not serve a request")
}

// writePluginScript drops a plugin into the working directory's plugin
// folder, which is where a project ships providers.
func writePluginScript(t *testing.T, workDir, name, script string) {
	t.Helper()
	dir := filepath.Join(workDir, ".crush", "plugins")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
}
