package orcarouter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProvidersExposeSeparateAuthenticationChoices(t *testing.T) {
	t.Setenv("ORCA_BASE_URL", "")
	t.Setenv("ORCA_API_BASE_URL", "")

	providers, err := Providers()
	require.NoError(t, err)
	require.Len(t, providers, 2)
	require.Equal(t, APIProviderID, string(providers[0].ID))
	require.Equal(t, "$ORCAROUTER_API_KEY", providers[0].APIKey)
	require.Equal(t, OAuthProviderID, string(providers[1].ID))
	require.Empty(t, providers[1].APIKey)
	require.Equal(t, defaultAPIBaseURL, providers[0].APIEndpoint)
	require.Equal(t, providers[0].APIEndpoint, providers[1].APIEndpoint)
	require.NotEmpty(t, providers[0].Models)
	require.Equal(t, providers[0].Models, providers[1].Models)
}

func TestFetchProvidersBoundsAndFiltersCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Empty(t, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "openai/gpt-5.5",
					"name": "GPT-5.5 live",
					"object": "model",
					"supported_endpoint_types": ["openai"],
					"architecture": {"input_modalities": ["text", "image"]},
					"top_provider": {"context_length": 1000, "max_completion_tokens": 200},
					"pricing": {"prompt": "0.000002", "completion_per_million": "7"}
				},
				{"id": "anthropic-only", "object": "model", "supported_endpoint_types": ["anthropic"]},
				{"id": "openai/gpt-5.5", "object": "model", "supported_endpoint_types": ["openai"]},
				{"id": "", "object": "model", "supported_endpoint_types": ["openai"]}
			]
		}`))
	}))
	defer server.Close()
	t.Setenv("ORCA_API_BASE_URL", server.URL)
	t.Setenv("ORCA_BASE_URL", "")

	providers, err := FetchProviders(t.Context())
	require.NoError(t, err)
	require.Len(t, providers, 2)
	require.Len(t, providers[0].Models, 1)
	model := providers[0].Models[0]
	require.Equal(t, "openai/gpt-5.5", model.ID)
	require.Equal(t, "GPT-5.5 live", model.Name)
	require.Equal(t, int64(1000), model.ContextWindow)
	require.Equal(t, int64(200), model.DefaultMaxTokens)
	require.Equal(t, 2.0, model.CostPer1MIn)
	require.Equal(t, 7.0, model.CostPer1MOut)
	require.True(t, model.CanReason, "verified fallback metadata is preserved")
	require.True(t, model.SupportsImages)
}

func TestBaseURLOverridesAreIndependentAndSecure(t *testing.T) {
	t.Setenv("ORCA_BASE_URL", "https://shared.example/prefix")
	t.Setenv("ORCA_AUTH_BASE_URL", "https://auth.example")
	t.Setenv("ORCA_API_BASE_URL", "https://api.example/custom/v1")

	authBase, err := AuthBaseURL()
	require.NoError(t, err)
	require.Equal(t, "https://auth.example", authBase)
	apiBase, err := APIBaseURL()
	require.NoError(t, err)
	require.Equal(t, "https://api.example/custom/v1", apiBase)

	t.Setenv("ORCA_AUTH_BASE_URL", "http://remote.example")
	_, err = AuthBaseURL()
	require.ErrorContains(t, err, "must use HTTPS")

	t.Setenv("ORCA_AUTH_BASE_URL", "http://127.0.0.1:8080")
	authBase, err = AuthBaseURL()
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:8080", authBase)
}
