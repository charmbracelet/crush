package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestCompletionsEnricher_PopulatesContextWindowAndMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		// Mix of field shapes seen across OpenAI-compatible providers:
		// llama-swap and 9Router use top-level context_length/context_window,
		// 9Router nests contextWindow under capabilities, and max_completion_tokens
		// carries the provider's own max output tokens.
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "m1", "object": "model", "context_length": 131072, "context_window": 131072},
				{"id": "m2", "object": "model", "context_length": 1048576, "max_completion_tokens": 65536},
				{"id": "m3", "object": "model", "capabilities": {"contextWindow": 200000}},
				{"id": "m4", "object": "model", "context_length": 128000}
			]
		}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}
	resolver := &mockResolver{}

	models := []catwalk.Model{
		{ID: "m1", Name: "m1"},
		{ID: "m2", Name: "m2"},
		{ID: "m3", Name: "m3"},
		{ID: "m4", Name: "m4"},
	}

	enriched, err := (&completionsEnricher{}).EnrichModels(context.Background(), cfg, resolver, models)
	require.NoError(t, err)
	require.Len(t, enriched, 4)

	require.Equal(t, int64(131072), enriched[0].ContextWindow)

	require.Equal(t, int64(1048576), enriched[1].ContextWindow)
	require.Equal(t, int64(65536), enriched[1].DefaultMaxTokens)

	require.Equal(t, int64(200000), enriched[2].ContextWindow)
	// No max_completion_tokens: fall back to 25% of the context window.
	require.Equal(t, int64(50000), enriched[2].DefaultMaxTokens)

	// No max_completion_tokens: fall back to 25% of the context window.
	require.Equal(t, int64(128000), enriched[3].ContextWindow)
	require.Equal(t, int64(32000), enriched[3].DefaultMaxTokens)
}

func TestCompletionsEnricher_PreservesExistingValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "m1", "context_length": 131072, "max_completion_tokens": 65536}
			]
		}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}
	resolver := &mockResolver{}

	// User overrides (or a local-server enricher) win: non-zero fields are
	// left untouched.
	models := []catwalk.Model{
		{ID: "m1", Name: "m1", ContextWindow: 200000, DefaultMaxTokens: 8192},
	}

	enriched, err := (&completionsEnricher{}).EnrichModels(context.Background(), cfg, resolver, models)
	require.NoError(t, err)
	require.Len(t, enriched, 1)

	require.Equal(t, int64(200000), enriched[0].ContextWindow)
	require.Equal(t, int64(8192), enriched[0].DefaultMaxTokens)
}

func TestCompletionsEnricher_NoModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}
	resolver := &mockResolver{}

	models := []catwalk.Model{{ID: "m1", Name: "m1", ContextWindow: 4096}}

	enriched, err := (&completionsEnricher{}).EnrichModels(context.Background(), cfg, resolver, models)
	require.NoError(t, err)
	require.Len(t, enriched, 1)
	require.Equal(t, int64(4096), enriched[0].ContextWindow)
}
