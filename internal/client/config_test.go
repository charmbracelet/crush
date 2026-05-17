package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

// captureClient returns a Client that talks to the given test server,
// plus a channel receiving the parsed request body for each call.
func captureClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c, err := NewClient(t.TempDir(), "tcp", u.Host)
	require.NoError(t, err)
	return c
}

func TestSetProviderAPIKeyStringSendsKind(t *testing.T) {
	t.Parallel()

	var got proto.ConfigProviderKeyRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	require.NoError(t, c.SetProviderAPIKey(context.Background(), "ws1", config.ScopeGlobal, "openai", "sk-xyz"))

	require.Equal(t, proto.APIKeyKindString, got.Kind)
	require.Equal(t, "openai", got.ProviderID)
	require.Equal(t, config.ScopeGlobal, got.Scope)
	decoded, err := got.DecodeAPIKey()
	require.NoError(t, err)
	require.Equal(t, "sk-xyz", decoded)
}

func TestSetProviderAPIKeyOAuthSendsKind(t *testing.T) {
	t.Parallel()

	var got proto.ConfigProviderKeyRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tok := &oauth.Token{AccessToken: "a", RefreshToken: "r", ExpiresIn: 60, ExpiresAt: 1234567890}
	c := captureClient(t, srv)
	require.NoError(t, c.SetProviderAPIKey(context.Background(), "ws1", config.ScopeGlobal, "hyper", tok))

	require.Equal(t, proto.APIKeyKindOAuth, got.Kind)
	decoded, err := got.DecodeAPIKey()
	require.NoError(t, err)
	require.Equal(t, tok, decoded.(*oauth.Token))
}

func TestSetProviderAPIKeyUnsupportedTypeFailsLocally(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	err := c.SetProviderAPIKey(context.Background(), "ws1", config.ScopeGlobal, "x", 42)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported api key type")
	require.False(t, called, "server should not have been reached")
}

func TestSetProviderAPIKeyNilOAuthFailsLocally(t *testing.T) {
	t.Parallel()

	c := captureClient(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	var tok *oauth.Token
	err := c.SetProviderAPIKey(context.Background(), "ws1", config.ScopeGlobal, "x", tok)
	require.Error(t, err)
}
