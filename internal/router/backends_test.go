package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListOpenRouterDecisionModels(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[{"id":"typesafe/jev-1.13","name":"TypeSafe: Jev 1.13"},{"id":"jaredpalmer/kev-4b","name":"Jared Palmer: Kev 4B"}]}`))
	}))
	defer server.Close()

	orig := openRouterModelsURL
	openRouterModelsURL = server.URL + "?output_modalities=decisions"
	defer func() { openRouterModelsURL = orig }()

	presets, err := ListOpenRouterDecisionModels(t.Context(), server.Client())
	require.NoError(t, err)
	require.Equal(t, "output_modalities=decisions", gotQuery)
	require.Equal(t, []Preset{
		{Label: "Jared Palmer: Kev 4B", Model: "jaredpalmer/kev-4b"},
		{Label: "TypeSafe: Jev 1.13", Model: "typesafe/jev-1.13"},
	}, presets)
}

func TestProviderNamesStartWithOpenRouter(t *testing.T) {
	t.Parallel()
	names := ProviderNames()
	require.Equal(t, "openrouter", names[0])
	require.ElementsMatch(t,
		[]string{"openrouter", "opencode-zen", "typesafe", "vercel", "cloudflare", "local"},
		names,
	)
}
