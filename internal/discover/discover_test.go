package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestDiscoverModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [
				{"id": "model-a", "object": "model", "owned_by": "org"},
				{"id": "model-b", "object": "model", "owned_by": "org"}
			]
		}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "model-a", models[0].ID)
	require.Equal(t, "model-b", models[1].ID)
}

func TestDiscoverModels_ExistingModelsWin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "model-a", "object": "model"},
				{"id": "model-b", "object": "model"}
			]
		}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		ExistingModels: []catwalk.Model{
			{ID: "model-a", Name: "My Custom Name", ContextWindow: 200000, CanReason: true},
		},
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 2)

	require.Equal(t, "model-a", models[0].ID)
	require.Equal(t, "My Custom Name", models[0].Name)
	require.Equal(t, int64(200000), models[0].ContextWindow)
	require.True(t, models[0].CanReason)

	require.Equal(t, "model-b", models[1].ID)
	require.Equal(t, "model-b", models[1].Name)
}

func TestDiscoverModels_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.Error(t, err)
	require.Nil(t, models)
}

func TestDiscoverModels_ExtraHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "custom-value", r.Header.Get("X-Custom"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "m1", "object": "model"}]}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		ExtraHeaders: map[string]string{
			"X-Custom": "custom-value",
		},
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
}

type mockResolver struct{}

func (m *mockResolver) ResolveValue(val string) (string, error) { return val, nil }

type envResolver struct {
	env map[string]string
}

func (e *envResolver) ResolveValue(val string) (string, error) {
	if v, ok := e.env[val]; ok {
		return v, nil
	}
	return val, nil
}

func TestDiscoverModels_ResolvesShellVariables(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer resolved-key", r.Header.Get("Authorization"))
		require.Equal(t, "resolved-header", r.Header.Get("X-Custom"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "m1", "object": "model"}]}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "$MY_API_KEY",
		ExtraHeaders: map[string]string{
			"X-Custom": "$MY_HEADER",
		},
	}

	resolver := &envResolver{env: map[string]string{
		"$MY_API_KEY": "resolved-key",
		"$MY_HEADER":  "resolved-header",
	}}

	models, err := DiscoverModels(context.Background(), cfg, resolver)
	require.NoError(t, err)
	require.Len(t, models, 1)
}

func TestDiscoverModels_SkipsEmptyExtraHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("X-Empty"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "m1", "object": "model"}]}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
		ExtraHeaders: map[string]string{
			"X-Empty": "$UNSET_VAR",
		},
	}

	resolver := &envResolver{env: map[string]string{
		"$UNSET_VAR": "",
	}}

	models, err := DiscoverModels(context.Background(), cfg, resolver)
	require.NoError(t, err)
	require.Len(t, models, 1)
}

func TestDiscoverModels_ContextLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [
				{"id": "glm-4.7", "object": "model", "context_length": 131072},
				{"id": "gemma-4", "object": "model", "context_length": 262144},
				{"id": "no-context", "object": "model"}
			]
		}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 3)

	require.Equal(t, "glm-4.7", models[0].ID)
	require.Equal(t, int64(131072), models[0].ContextWindow)

	require.Equal(t, "gemma-4", models[1].ID)
	require.Equal(t, int64(262144), models[1].ContextWindow)

	require.Equal(t, "no-context", models[2].ID)
	require.Equal(t, int64(0), models[2].ContextWindow)
}

func TestDiscoverModels_MetadataFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "vllm-model", "object": "model", "metadata": {"max_model_len": 65536}},
				{"id": "ctx-model", "object": "model", "metadata": {"max_context_length": 32768}},
				{"id": "both-model", "object": "model", "context_length": 100000, "metadata": {"max_model_len": 65536}}
			]
		}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "test-key",
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 3)

	// metadata.max_model_len fallback
	require.Equal(t, int64(65536), models[0].ContextWindow)
	// metadata.max_context_length fallback
	require.Equal(t, int64(32768), models[1].ContextWindow)
	// context_length takes precedence over metadata
	require.Equal(t, int64(100000), models[2].ContextWindow)
}

func TestDiscoverModels_NoAuthWhenNoAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": "m1", "object": "model"}]}`))
	}))
	defer server.Close()

	cfg := Config{
		ID:      "test",
		BaseURL: server.URL + "/v1",
		APIKey:  "",
	}

	models, err := DiscoverModels(context.Background(), cfg, &mockResolver{})
	require.NoError(t, err)
	require.Len(t, models, 1)
}

func TestStripV1Suffix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:8000/v1", "http://localhost:8000"},
		{"http://localhost:8000/v1/", "http://localhost:8000"},
		{"http://localhost:8000", "http://localhost:8000"},
		{"http://localhost:8000/", "http://localhost:8000"},
		{"http://localhost:8000/api/v1", "http://localhost:8000/api"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripV1Suffix(tt.input)
		require.Equal(t, tt.want, got, "stripV1Suffix(%q)", tt.input)
	}
}
