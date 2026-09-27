package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/router"
	"github.com/stretchr/testify/require"
)

// TestRouteDecision_SelectsEffort exercises resolveRouterDecision directly
// against a fake System One server, without spinning up a full coordinator
// + provider stack.
func TestRouteDecision_SelectsEffort(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": 2, "confidence": 0.95},
			},
		})
	}))
	defer server.Close()

	routerCfg := &config.RouterOptions{
		Enabled:  true,
		Provider: "local",
		BaseURL:  server.URL,
		Model:    "test-model",
	}

	decision, ok, err := resolveRouterDecision(t.Context(), routerCfg, "", "run the tests", nil)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "high", decision.ReasoningEffort)
	require.False(t, decision.LowConfidence)
}

// TestRouteDecision_ReportsQueryingAroundTheCall proves the querying
// callback fires exactly twice around the HTTP call: once with active=true
// before the request, and once with active=false after it returns —
// regardless of whether the call succeeded — so a live "consulting"
// indicator in the UI never gets stuck on.
func TestRouteDecision_ReportsQueryingAroundTheCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": 2, "confidence": 0.95},
			},
		})
	}))
	defer server.Close()

	routerCfg := &config.RouterOptions{
		Enabled:  true,
		Provider: "local",
		BaseURL:  server.URL,
		Model:    "test-model",
	}

	var models []string
	var actives []bool
	_, ok, err := resolveRouterDecision(t.Context(), routerCfg, "", "run the tests", func(model string, active bool) {
		models = append(models, model)
		actives = append(actives, active)
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []string{"test-model", "test-model"}, models)
	require.Equal(t, []bool{true, false}, actives)
}

// TestRouteDecision_ReportsQueryingEvenOnFailure proves the querying
// callback's active=false call still fires when the router call fails,
// so a failed router call never leaves the UI's "consulting" indicator on.
func TestRouteDecision_ReportsQueryingEvenOnFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	routerCfg := &config.RouterOptions{Enabled: true, Provider: "local", BaseURL: server.URL, Model: "test-model"}

	var actives []bool
	_, ok, err := resolveRouterDecision(t.Context(), routerCfg, "", "prompt", func(_ string, active bool) {
		actives = append(actives, active)
	})
	require.Error(t, err)
	require.False(t, ok)
	require.Equal(t, []bool{true, false}, actives)
}

func TestRouteDecision_DisabledIsNoop(t *testing.T) {
	t.Parallel()

	_, ok, err := resolveRouterDecision(t.Context(), &config.RouterOptions{Enabled: false}, "", "prompt", nil)
	require.NoError(t, err, "disabled is a no-op, not a failure")
	require.False(t, ok)
}

func TestRouteDecision_ServerErrorFailsOpen(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	routerCfg := &config.RouterOptions{Enabled: true, Provider: "local", BaseURL: server.URL, Model: "test-model"}
	_, ok, err := resolveRouterDecision(t.Context(), routerCfg, "", "prompt", nil)
	require.Error(t, err)
	require.False(t, ok)
}

// TestRouteDecision_OpenRouterProviderDefaultsModelAndFallsBackToAPIKey
// checks the openrouter endpoint resolution in isolation. The openrouter
// path always dials the real OpenRouter base URL, so a round-trip test would
// need network access (and would send data to openrouter.ai); instead the
// provider switch is tested through resolveRouterEndpoint, which is the
// exact code resolveRouterDecision uses to build its client.
func TestRouteDecision_OpenRouterProviderDefaultsModelAndFallsBackToAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       config.RouterOptions
		wantModel string
		wantKey   string
	}{
		{
			name:      "empty model and key fall back to defaults",
			cfg:       config.RouterOptions{Enabled: true, Provider: "openrouter"},
			wantModel: router.Backends["openrouter"].DefaultModel,
			wantKey:   "test-fallback-key",
		},
		{
			name:      "empty provider behaves like openrouter",
			cfg:       config.RouterOptions{Enabled: true},
			wantModel: router.Backends["openrouter"].DefaultModel,
			wantKey:   "test-fallback-key",
		},
		{
			name:      "explicit model and key win over defaults",
			cfg:       config.RouterOptions{Enabled: true, Provider: "openrouter", Model: "custom/model", APIKey: "router-key"},
			wantModel: "custom/model",
			wantKey:   "router-key",
		},
		{
			name:      "configured base URL is ignored for openrouter",
			cfg:       config.RouterOptions{Enabled: true, Provider: "openrouter", BaseURL: "http://example.invalid"},
			wantModel: router.Backends["openrouter"].DefaultModel,
			wantKey:   "test-fallback-key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			endpoint, err := resolveRouterEndpoint(&tc.cfg, "test-fallback-key")
			require.NoError(t, err)
			require.Equal(t, router.Backends["openrouter"].BaseURL, endpoint.baseURL)
			require.Equal(t, tc.wantModel, endpoint.model)
			require.Equal(t, tc.wantKey, endpoint.apiKey)
		})
	}
}

// TestRouteDecision_OpenRouterUsesDecisionsPathNotSystemOne pins the actual
// wire path OpenRouter's alpha Decisions API expects
// (POST https://openrouter.ai/api/alpha/decisions, per @openrouter/sdk's
// generated client) against the local-provider System One path
// (POST {base_url}/v1/systemone) — a prior version of this endpoint
// resolution sent every provider to /systemone, which 404s against real
// OpenRouter and made the router silently fail open on every message.
func TestRouteDecision_OpenRouterUsesDecisionsPathNotSystemOne(t *testing.T) {
	t.Parallel()

	openrouterEndpoint, err := resolveRouterEndpoint(&config.RouterOptions{Enabled: true, Provider: "openrouter"}, "test-fallback-key")
	require.NoError(t, err)
	require.Equal(t, "/api/alpha/decisions", openrouterEndpoint.path)

	localEndpoint, err := resolveRouterEndpoint(&config.RouterOptions{Enabled: true, Provider: "local", BaseURL: "http://localhost:9999"}, "test-fallback-key")
	require.NoError(t, err)
	require.Equal(t, "/v1/systemone", localEndpoint.path)
}

// TestRouteDecision_SystemOneProviders checks that every backend serving
// the System One contract at {base_url}/v1/systemone uses its documented
// default URL and model, reuses Crush's key for the same-named provider,
// and lets a configured base URL override the default. OpenRouter (its
// own decisions path), Vercel (a nested path) and Cloudflare (an
// account-scoped URL with a nested request body) are covered separately.
func TestRouteDecision_SystemOneProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       config.RouterOptions
		wantURL   string
		wantModel string
		wantKey   string
	}{
		{
			name:      "opencode-zen defaults and reuses crush key",
			cfg:       config.RouterOptions{Provider: "opencode-zen"},
			wantURL:   "https://opencode.ai/zen",
			wantModel: "jev-1.13",
			wantKey:   "crush-key",
		},
		{
			name:      "typesafe defaults",
			cfg:       config.RouterOptions{Provider: "typesafe", APIKey: "ts-key"},
			wantURL:   "https://api.typesafe.ai",
			wantModel: "jev-latest",
			wantKey:   "ts-key",
		},
		{
			name:      "configured base URL overrides default",
			cfg:       config.RouterOptions{Provider: "typesafe", BaseURL: "http://proxy.invalid"},
			wantURL:   "http://proxy.invalid",
			wantModel: "jev-latest",
			wantKey:   "crush-key",
		},
		{
			name:      "local keeps configured url and model",
			cfg:       config.RouterOptions{Provider: "local", BaseURL: "http://127.0.0.1:8008", Model: "kev-latest"},
			wantURL:   "http://127.0.0.1:8008",
			wantModel: "kev-latest",
			wantKey:   "crush-key",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			endpoint, err := resolveRouterEndpoint(&tc.cfg, "crush-key")
			require.NoError(t, err)
			require.Equal(t, router.SystemOnePath, endpoint.path)
			require.Equal(t, tc.wantURL, endpoint.baseURL)
			require.Equal(t, tc.wantModel, endpoint.model)
			require.Equal(t, tc.wantKey, endpoint.apiKey)
		})
	}

	_, err := resolveRouterEndpoint(&config.RouterOptions{Provider: "local"}, "crush-key")
	require.Error(t, err, "local without a base URL must fail open")
}

// TestRouteDecision_VercelProvider checks the Vercel AI Gateway backend:
// the TypeSafe-compatible base URL and its own path, with the gateway's
// model default, while still speaking the flat System One request shape.
func TestRouteDecision_VercelProvider(t *testing.T) {
	t.Parallel()

	endpoint, err := resolveRouterEndpoint(&config.RouterOptions{Provider: "vercel"}, "crush-key")
	require.NoError(t, err)
	require.Equal(t, "https://ai-gateway.vercel.sh", endpoint.baseURL)
	require.Equal(t, "/typesafe/v1/systemone", endpoint.path)
	require.Equal(t, "typesafe-ai/jev", endpoint.model)
	require.Equal(t, "crush-key", endpoint.apiKey)
	require.False(t, endpoint.nestedInput, "Vercel uses the flat System One shape")
}

// TestRouteDecision_CloudflareProvider checks the Cloudflare Workers AI
// backend: its run URL is account-scoped, so a configured base_url is
// mandatory, the request body nests state and questions under input, and
// the model default is Cloudflare's own Jev id.
func TestRouteDecision_CloudflareProvider(t *testing.T) {
	t.Parallel()

	_, err := resolveRouterEndpoint(&config.RouterOptions{Provider: "cloudflare"}, "crush-key")
	require.Error(t, err, "cloudflare without a base URL must fail open")

	endpoint, err := resolveRouterEndpoint(&config.RouterOptions{
		Provider: "cloudflare",
		BaseURL:  "https://api.cloudflare.com/client/v4/accounts/acct/ai/run",
	}, "crush-key")
	require.NoError(t, err)
	require.Equal(t, "https://api.cloudflare.com/client/v4/accounts/acct/ai/run", endpoint.baseURL)
	require.Empty(t, endpoint.path, "the account-scoped run URL is the whole address")
	require.Equal(t, "typesafe/jev", endpoint.model)
	require.True(t, endpoint.nestedInput, "Cloudflare nests state/questions under input")
}

func TestRouteDecision_UnrecognizedProviderFailsOpen(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"reasoning_effort": map[string]any{"type": "score", "score": 1, "confidence": 1},
			},
		})
	}))
	defer server.Close()

	for _, provider := range []string{"Local", "ollama", "OpenRouter"} {
		routerCfg := &config.RouterOptions{Enabled: true, Provider: provider, BaseURL: server.URL, Model: "test-model"}

		_, endpointErr := resolveRouterEndpoint(routerCfg, "test-fallback-key")
		require.Error(t, endpointErr, "provider %q must not resolve to any endpoint", provider)

		_, ok, decisionErr := resolveRouterDecision(t.Context(), routerCfg, "test-fallback-key", "prompt", nil)
		require.Error(t, decisionErr, "provider %q must fail open", provider)
		require.False(t, ok, "provider %q must fail open", provider)
	}
	require.Zero(t, hits.Load(), "an unrecognized provider must not make any router request")
}
