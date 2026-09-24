package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestFlow builds a flow around a stub token endpoint without binding
// the fixed callback ports, so tests stay hermetic. It mutates the
// package-level tokenEndpoint, so tests using it must not run in
// parallel with each other.
func newTestFlow(t *testing.T, tokenResp any) *BrowserFlow {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResp)
	}))
	t.Cleanup(server.Close)

	orig := tokenEndpoint
	tokenEndpoint = server.URL
	t.Cleanup(func() { tokenEndpoint = orig })

	return &BrowserFlow{
		pkce:        PKCE{Verifier: "test-verifier", Challenge: "test-challenge"},
		state:       "test-state",
		redirectURI: "http://localhost:1455/auth/callback",
		result:      make(chan *http.Request, 1),
	}
}

func TestBrowserFlow_CallbackSuccess(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{
		AccessToken: "at-flow",
		ExpiresIn:   300,
		IDToken:     signClaims(t, map[string]any{"chatgpt_account_id": "acct-flow"}),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/callback?code=abc&state=test-state", nil)
	flow.handleCallback(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "You’re all set")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token, err := flow.Wait(ctx)
	require.NoError(t, err)
	require.Equal(t, "at-flow", token.AccessToken)
	require.Equal(t, "acct-flow", token.AccountID)
}

func TestBrowserFlow_StateMismatch(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/callback?code=abc&state=evil", nil)
	flow.handleCallback(rec, req)

	_, err := flow.Wait(context.Background())
	require.ErrorContains(t, err, "state mismatch")
}

func TestBrowserFlow_ProviderError(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/callback?error=access_denied&error_description=nope", nil)
	flow.handleCallback(rec, req)

	// The failure page keeps a 400 so the browser reflects the outcome.
	require.Equal(t, http.StatusBadRequest, rec.Code)

	_, err := flow.Wait(context.Background())
	require.ErrorContains(t, err, "access_denied")
}

func TestBrowserFlow_StartHandoffPage(t *testing.T) {
	flow := &BrowserFlow{
		pkce:        PKCE{Verifier: "test-verifier", Challenge: "test-challenge"},
		state:       "test-state",
		redirectURI: "http://localhost:1455/auth/callback",
		authURL:     "https://auth.openai.com/oauth/authorize?client_id=x",
		startURL:    "http://localhost:1455/auth/start",
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/start", nil)
	flow.handleStart(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// The handoff page opens the real authorization URL in a new tab.
	require.Contains(t, body, "One more click")
	require.Contains(t, body, `href="https://auth.openai.com/oauth/authorize?client_id=x"`)
	require.Contains(t, body, `target="_blank"`)
	require.Contains(t, body, `id="continue"`)
}

func TestBrowserFlow_MissingCode(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/callback?state=test-state", nil)
	flow.handleCallback(rec, req)

	_, err := flow.Wait(context.Background())
	require.ErrorContains(t, err, "no code")
}

func TestBrowserFlow_WaitCancel(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := flow.Wait(ctx)
	require.ErrorIs(t, err, context.Canceled)
}
