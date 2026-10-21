package grok

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestFlow builds a flow around a stub token endpoint without binding
// the loopback callback listener, so tests stay hermetic. It mutates the
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
		nonce:       "test-nonce",
		redirectURI: "http://127.0.0.1:45131/callback",
		result:      make(chan *http.Request, 1),
	}
}

func TestBrowserFlow_CallbackSuccess(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{
		AccessToken: "at-flow",
		ExpiresIn:   300,
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?code=abc&state=test-state", nil)
	flow.handleCallback(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "You’re all set")
	require.Equal(t, accountsOrigin, rec.Header().Get("Access-Control-Allow-Origin"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	token, err := flow.Wait(ctx)
	require.NoError(t, err)
	require.Equal(t, "at-flow", token.AccessToken)
}

func TestBrowserFlow_CallbackPreflight(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/callback", nil)
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	flow.handleCallback(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, accountsOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Private-Network"))
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")

	// A preflight must not be mistaken for the real callback: the flow
	// still has no result to hand to Wait.
	select {
	case <-flow.result:
		t.Fatal("preflight delivered a result to the flow")
	default:
	}
}

func TestBrowserFlow_StateMismatch(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?code=abc&state=evil", nil)
	flow.handleCallback(rec, req)

	_, err := flow.Wait(context.Background())
	require.ErrorContains(t, err, "state mismatch")
}

func TestBrowserFlow_ProviderError(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?error=access_denied&error_description=nope", nil)
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
		nonce:       "test-nonce",
		redirectURI: "http://127.0.0.1:45131/callback",
		authURL:     "https://auth.x.ai/oauth2/authorize?client_id=x",
		startURL:    "http://127.0.0.1:45131/auth/start",
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/start", nil)
	flow.handleStart(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// The handoff page opens the real authorization URL in a new tab.
	require.Contains(t, body, "One more click")
	require.Contains(t, body, `href="https://auth.x.ai/oauth2/authorize?client_id=x"`)
	require.Contains(t, body, `target="_blank"`)
	require.Contains(t, body, `id="continue"`)
	require.Contains(t, body, "Grok")
}

func TestBrowserFlow_MissingCode(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/callback?state=test-state", nil)
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

func TestCompleteWithCode(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{
		AccessToken:  "at-pasted",
		RefreshToken: "rt-pasted",
		ExpiresIn:    3600,
	})

	token, err := flow.CompleteWithCode(context.Background(), "http://127.0.0.1:45131/callback?code=abc&state=test-state")
	require.NoError(t, err)
	require.Equal(t, "at-pasted", token.AccessToken)
}

func TestCompleteWithCode_BareCodeSkipsState(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "at-bare", ExpiresIn: 60})

	// A bare code carries no state to validate, exactly like the Grok
	// CLI's paste fallback.
	token, err := flow.CompleteWithCode(context.Background(), "  bare-code  ")
	require.NoError(t, err)
	require.Equal(t, "at-bare", token.AccessToken)
}

func TestCompleteWithCode_StateMismatch(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	_, err := flow.CompleteWithCode(context.Background(), "http://127.0.0.1:45131/callback?code=abc&state=evil")
	require.ErrorContains(t, err, "state mismatch")
}

func TestCompleteWithCode_URLError(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	_, err := flow.CompleteWithCode(context.Background(), "http://127.0.0.1:45131/callback?error=access_denied&error_description=nope")
	require.ErrorContains(t, err, "access_denied")
}

func TestCompleteWithCode_InvalidInputs(t *testing.T) {
	flow := newTestFlow(t, tokenResponse{AccessToken: "x", ExpiresIn: 60})

	for _, input := range []string{"", "   ", "http://127.0.0.1:45131/callback?state=test-state"} {
		_, err := flow.CompleteWithCode(context.Background(), input)
		require.Error(t, err, "input %q must be rejected", input)
	}
}

func TestStartBrowserFlow(t *testing.T) {
	flow, err := StartBrowserFlow()
	require.NoError(t, err)
	t.Cleanup(flow.Close)

	require.NotEmpty(t, flow.URL())
	require.Contains(t, flow.URL(), Issuer+"/oauth2/authorize?")
	// The redirect URI names the loopback listener the flow bound.
	require.Contains(t, flow.URL(), url.QueryEscape(flow.RedirectURI()))
	// The OS assigns the port, so it is neither fixed nor privileged.
	require.NotEqual(t, 0, flow.RedirectURI())
}
