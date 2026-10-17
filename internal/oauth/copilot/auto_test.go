package copilot

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestFetchAutoSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer at-auto", r.Header.Get("Authorization"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"auto_mode":{"model_hints":["auto"]}}`, string(body))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"available_models": ["gpt-4.1", "claude-sonnet-4.5"],
			"session_token": "sess-1",
			"expires_at": 4102444800
		}`))
	}))
	t.Cleanup(server.Close)

	orig := autoSessionURL
	autoSessionURL = server.URL
	t.Cleanup(func() { autoSessionURL = orig })

	session, err := fetchAutoSession(context.Background(), &oauth.Token{AccessToken: "at-auto"})
	require.NoError(t, err)
	require.Equal(t, "sess-1", session.SessionToken)
	require.Equal(t, []string{"gpt-4.1", "claude-sonnet-4.5"}, session.AvailableModels)
}

func TestFetchAutoSession_Errors(t *testing.T) {
	t.Run("nil token", func(t *testing.T) {
		_, err := fetchAutoSession(context.Background(), nil)
		require.ErrorContains(t, err, "OAuth token")
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		t.Cleanup(server.Close)

		orig := autoSessionURL
		autoSessionURL = server.URL
		t.Cleanup(func() { autoSessionURL = orig })

		_, err := fetchAutoSession(context.Background(), &oauth.Token{AccessToken: "at"})
		require.Error(t, err)
	})

	t.Run("no usable model", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"available_models": [], "session_token": "sess-1"}`))
		}))
		t.Cleanup(server.Close)

		orig := autoSessionURL
		autoSessionURL = server.URL
		t.Cleanup(func() { autoSessionURL = orig })

		_, err := fetchAutoSession(context.Background(), &oauth.Token{AccessToken: "at"})
		require.ErrorContains(t, err, "no usable model")
	})
}

func TestAutoResolverCachesSession(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{
			"available_models": ["gpt-4.1"],
			"session_token": "sess-1",
			"expires_at": 4102444800
		}`))
	}))
	t.Cleanup(server.Close)

	orig := autoSessionURL
	autoSessionURL = server.URL
	t.Cleanup(func() { autoSessionURL = orig })
	stubCatalog(t, `{
		"data": [
			{"id": "gpt-4.1", "name": "GPT-4.1", "supported_endpoints": ["/chat/completions"]}
		]
	}`)

	resolver := NewAutoResolver(func() *oauth.Token { return &oauth.Token{AccessToken: "at"} }, nil)

	selection, err := resolver.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "gpt-4.1", selection.model)
	require.Equal(t, "sess-1", selection.sessionToken)

	selection, err = resolver.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "gpt-4.1", selection.model)
	require.Equal(t, "sess-1", selection.sessionToken)
	require.Equal(t, int32(1), calls.Load(), "a fresh session is reused")

	resolver.session.ExpiresAt = time.Now().Unix() - 60
	_, err = resolver.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load(), "an expired session is refreshed")
}

// stubCatalog points the models endpoint at a test server serving the
// given catalog payload.
func stubCatalog(t *testing.T, payload string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)

	orig := modelsEndpoint
	modelsEndpoint = server.URL
	t.Cleanup(func() { modelsEndpoint = orig })
}

func TestAutoResolverRoutesResponsesOnlyModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"available_models": ["gpt-5.4-mini", "gpt-4.1"],
			"session_token": "sess-1",
			"expires_at": 4102444800
		}`))
	}))
	t.Cleanup(server.Close)

	orig := autoSessionURL
	autoSessionURL = server.URL
	t.Cleanup(func() { autoSessionURL = orig })
	stubCatalog(t, `{
		"data": [
			{"id": "gpt-5.4-mini", "name": "GPT-5.4 mini", "supported_endpoints": ["/responses"]},
			{"id": "gpt-4.1", "name": "GPT-4.1", "supported_endpoints": ["/chat/completions", "/responses"]}
		]
	}`)

	resolver := NewAutoResolver(func() *oauth.Token { return &oauth.Token{AccessToken: "at"} }, nil)
	selection, err := resolver.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "gpt-5.4-mini", selection.model)
	require.Equal(t, autoEndpointResponses, selection.endpoint)
	require.True(t, resolver.UsesResponsesAPI("gpt-5.4-mini"))
}

func TestAutoResolverFallsBackWhenCatalogFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"available_models": ["gpt-5.4-mini"],
			"session_token": "sess-1",
			"expires_at": 4102444800
		}`))
	}))
	t.Cleanup(server.Close)

	orig := autoSessionURL
	autoSessionURL = server.URL
	t.Cleanup(func() { autoSessionURL = orig })

	catalogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(catalogServer.Close)

	origModels := modelsEndpoint
	modelsEndpoint = catalogServer.URL
	t.Cleanup(func() { modelsEndpoint = origModels })

	resolver := NewAutoResolver(func() *oauth.Token { return &oauth.Token{AccessToken: "at"} }, func(modelID string) bool {
		return modelID == "gpt-5.4-mini"
	})
	selection, err := resolver.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "gpt-5.4-mini", selection.model, "the first session model is used when the catalog is unreachable")
	require.Equal(t, autoEndpointUnknown, selection.endpoint)
	require.True(t, resolver.UsesResponsesAPI("gpt-5.4-mini"))
}

func TestAutoResolverNoSupportedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"available_models": ["gpt-5.4-mini"],
			"session_token": "sess-1",
			"expires_at": 4102444800
		}`))
	}))
	t.Cleanup(server.Close)

	orig := autoSessionURL
	autoSessionURL = server.URL
	t.Cleanup(func() { autoSessionURL = orig })
	stubCatalog(t, `{
		"data": [
			{"id": "gpt-5.4-mini", "name": "GPT-5.4 mini", "supported_endpoints": ["/messages"]}
		]
	}`)

	resolver := NewAutoResolver(func() *oauth.Token { return &oauth.Token{AccessToken: "at"} }, nil)
	_, err := resolver.resolve(context.Background())
	require.ErrorContains(t, err, "supported endpoint")
}
