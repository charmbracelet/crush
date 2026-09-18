package copilot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

	resolver := newAutoResolver(func() *oauth.Token { return &oauth.Token{AccessToken: "at"} })

	model, sessionToken, err := resolver.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "gpt-4.1", model)
	require.Equal(t, "sess-1", sessionToken)

	model, sessionToken, err = resolver.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "gpt-4.1", model)
	require.Equal(t, "sess-1", sessionToken)
	require.Equal(t, int32(1), calls.Load(), "a fresh session is reused")

	resolver.session.ExpiresAt = time.Now().Unix() - 60
	_, _, err = resolver.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load(), "an expired session is refreshed")
}

func TestInitiatorTransportAutoModel(t *testing.T) {
	sessionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"available_models": ["gpt-4.1"],
			"session_token": "sess-1",
			"expires_at": 4102444800
		}`))
	}))
	t.Cleanup(sessionServer.Close)

	orig := autoSessionURL
	autoSessionURL = sessionServer.URL
	t.Cleanup(func() { autoSessionURL = orig })

	var gotModel, gotSessionToken string
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSessionToken = r.Header.Get("Copilot-Session-Token")
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &payload)
		gotModel = payload.Model
	}))
	t.Cleanup(chatServer.Close)

	transport := &initiatorTransport{
		auto: newAutoResolver(func() *oauth.Token { return &oauth.Token{AccessToken: "at"} }),
	}
	client := &http.Client{Transport: transport}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, chatServer.URL,
		strings.NewReader(`{"model":"auto","messages":[{"role":"user"}]}`))
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "gpt-4.1", gotModel, "auto is rewritten to the session-granted model")
	require.Equal(t, "sess-1", gotSessionToken)
}

func TestInitiatorTransportAutoModelPassthrough(t *testing.T) {
	var gotModel, gotSessionToken string
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSessionToken = r.Header.Get("Copilot-Session-Token")
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &payload)
		gotModel = payload.Model
	}))
	t.Cleanup(chatServer.Close)

	transport := &initiatorTransport{
		auto: newAutoResolver(func() *oauth.Token { return &oauth.Token{AccessToken: "at"} }),
	}
	client := &http.Client{Transport: transport}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, chatServer.URL,
		strings.NewReader(`{"model":"gpt-4.1","messages":[{"role":"user"}]}`))
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "gpt-4.1", gotModel, "a concrete model is untouched")
	require.Empty(t, gotSessionToken, "no session token is attached")
}
