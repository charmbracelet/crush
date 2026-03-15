package config

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/stretchr/testify/require"
)

func TestProviderConfigTestConnection_Vercel(t *testing.T) {
	t.Parallel()

	const apiKey = "vercel-test-key"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/models", r.URL.Path)
		require.Equal(t, "Bearer "+apiKey, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := ProviderConfig{
		ID:      string(catwalk.InferenceProviderVercel),
		Type:    catwalk.TypeVercel,
		BaseURL: server.URL,
		APIKey:  apiKey,
	}

	err := cfg.TestConnection(NewEnvironmentVariableResolver(env.NewFromMap(nil)))
	require.NoError(t, err)
}
