package shellconfig

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProviderOAuthFlags(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add example \
  --type openai-compat \
  --base-url "https://api.example.com/v1" \
  --api-key "$EXAMPLE_KEY" \
  --oauth-issuer "https://auth.example.com" \
  --oauth-client-id "crush-example" \
  --oauth-client-secret "$EXAMPLE_SECRET" \
  --oauth-flow device \
  --oauth-scope openid \
  --oauth-scope offline_access \
  --oauth-redirect-uri "http://127.0.0.1:8979/callback" \
  --oauth-callback-port 8979 \
  --oauth-param audience "https://api.example.com" \
  --oauth-secret-basic true`)

	p := result["providers"].(map[string]any)["example"].(map[string]any)
	require.Equal(t, "https://api.example.com/v1", p["base_url"])

	auth := p["auth"].(map[string]any)
	require.Equal(t, "https://auth.example.com", auth["issuer"])
	require.Equal(t, "crush-example", auth["client_id"])
	require.Equal(t, "device", auth["flow"])
	require.Equal(t, "http://127.0.0.1:8979/callback", auth["redirect_uri"])
	require.Equal(t, float64(8979), auth["callback_port"])
	require.Equal(t, true, auth["client_secret_basic"])
	require.Equal(
		t,
		map[string]any{"audience": "https://api.example.com"},
		auth["extra_params"],
	)

	scopes := auth["scopes"].([]any)
	require.Equal(t, []any{"openid", "offline_access"}, scopes)
}

// A provider whose token endpoint only answers a request identifying the
// client declares the headers it demands; the declaration is what keeps
// Crush from hardcoding any provider's identity.
func TestProviderOAuthTokenHeaders(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add example \
  --oauth-token-url "https://auth.example.com/token" \
  --oauth-token-header User-Agent "example-cli/2.1.0" \
  --oauth-token-header X-Client "crush"`)

	auth := result["providers"].(map[string]any)["example"].(map[string]any)["auth"].(map[string]any)
	require.Equal(t, map[string]any{
		"User-Agent": "example-cli/2.1.0",
		"X-Client":   "crush",
	}, auth["token_headers"].(map[string]any))
}

func TestProviderOAuthFlagsAccumulate(t *testing.T) {
	t.Parallel()

	// Repeated calls merge, so a plugin can widen scopes after the fact.
	result := loadScript(t, `provider add example --oauth-issuer "https://auth.example.com" --oauth-scope read
provider add example --oauth-scope write`)

	auth := result["providers"].(map[string]any)["example"].(map[string]any)["auth"].(map[string]any)
	require.Equal(t, []any{"read", "write"}, auth["scopes"].([]any))
}

func TestProviderOAuthFlowValidation(t *testing.T) {
	t.Parallel()

	err := loadScriptErr(t, `provider add example --oauth-flwo device`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown flag")

	err = loadScriptErr(t, `provider add example --oauth-flow carrier-pigeon`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--oauth-flow must be one of: auto, browser, device")
}

func TestProviderOAuthParamRequiresTwoArgs(t *testing.T) {
	t.Parallel()

	err := loadScriptErr(t, `provider add example --oauth-param only-a-key`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires a key and value")
}

func TestProviderUsageFlags(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add example \
  --usage-url "https://api.example.com/v1internal:retrieveUserQuotaSummary" \
  --usage-method POST \
  --usage-groups groups \
  --usage-meters buckets \
  --usage-remaining remainingFraction \
  --usage-spent-percent percent \
  --usage-reset resetTime \
  --usage-title Credits \
  --usage-window window \
  --usage-model-group gemini "Gemini" \
  --usage-model-group cld "Claude"`)

	report := result["providers"].(map[string]any)["example"].(map[string]any)["usage"].(map[string]any)
	require.Equal(t, "https://api.example.com/v1internal:retrieveUserQuotaSummary", report["url"])
	require.Equal(t, "POST", report["method"])
	require.Equal(t, "groups", report["groups"])
	require.Equal(t, "buckets", report["meters"])
	require.Equal(t, "remainingFraction", report["remaining"])
	require.Equal(t, "percent", report["spent_percent"])
	require.Equal(t, "resetTime", report["reset"])
	require.Equal(t, "Credits", report["title"])
	require.Equal(t, "window", report["window"])
	require.Equal(t, map[string]any{"gemini": "Gemini", "cld": "Claude"}, report["model_groups"])
}

func TestProviderGatewayFlags(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add example   --gateway-request '.url as $u | {url: ($u + "?rewritten"), drop_headers: ["x-goog-api-key"]}'   --gateway-response '(.body.response // .body)'   --gateway-http1 true`)

	gateway := result["providers"].(map[string]any)["example"].(map[string]any)["gateway"].(map[string]any)
	require.Contains(t, gateway["request"], "drop_headers")
	require.Equal(t, "(.body.response // .body)", gateway["response"])
	require.Equal(t, true, gateway["http1"])
}

func TestProviderCatalogFlags(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add example \
  --catalog-url "https://api.example.com/v1/models" \
  --catalog-method POST \
  --catalog-body '{"pageSize":200}' \
  --catalog-header anthropic-version "2023-06-01" \
  --catalog-program '.data | map({id})'`)

	catalog := result["providers"].(map[string]any)["example"].(map[string]any)["catalog"].(map[string]any)
	require.Equal(t, "https://api.example.com/v1/models", catalog["url"])
	require.Equal(t, ".data | map({id})", catalog["program"])
	require.Equal(t, "POST", catalog["method"])
	require.Equal(t, `{"pageSize":200}`, catalog["body"])
	require.Equal(t, map[string]any{"anthropic-version": "2023-06-01"}, catalog["headers"])
}

// loadScriptErr runs a script that is expected to fail at load time.
func loadScriptErr(t *testing.T, script string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(script))
	return err
}
