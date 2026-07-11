package xai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequestDeviceCodeAndPoll(t *testing.T) {
	// Not parallel: mutates package-level discovery/client vars.

	var polls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": "https://auth.x.ai/oauth/device/code",
			"token_endpoint":                "https://auth.x.ai/oauth/token",
		})
	})
	// We rewrite discovery endpoints below via a custom server host that
	// still ends with .x.ai — for tests we temporarily relax trust by
	// using the real path pattern on httptest and injecting hosts.
	// Instead, serve everything on one httptest and override discoveryURL
	// + trust by returning endpoints on the test server after patching
	// requireTrusted via discovery that uses x.ai hosts rewritten through
	// a reverse approach: use httptest and set discovery to return absolute
	// paths on the same host by temporarily allowing via discoveryURL and
	// a test-only host check.

	// Simpler: host the discovery doc and intercept with rewritten vars.
	// We patch discoveryURL and use endpoints that pass trust by pointing
	// hostnames to auth.x.ai while routing via httptest Transport? Too heavy.
	// Use unexported-friendly approach: set discoveryURL to test server and
	// monkey-patch requireTrusted by using endpoints with host x.ai that
	// we never actually call — instead rewrite httpClient Transport.

	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	// Replace with a server that uses hostname we can trust after fixing
	// requireTrusted to accept the test server URL scheme http from
	// httptest — change test to use a full flow with package vars and a
	// temporary override of discovery that returns srv.URL endpoints,
	// and override requireTrusted by testing at a higher level with
	// httpClient only.

	// Re-implement mux with absolute srv URLs and temporarily replace
	// requireTrustedXAIEndpoint by testing only functions that don't need
	// it after discovery is injected.

	// Final approach: override discoveryURL and make requireTrusted accept
	// the test server by using host "auth.x.ai" in discovery while routing
	// requests through a custom RoundTripper that maps auth.x.ai -> srv.
	transport := rewriteTransport{base: srv.Client().Transport, target: srv.URL}
	oldClient := httpClient
	oldDiscovery := discoveryURL
	httpClient = &http.Client{Transport: transport, Timeout: 10 * time.Second}
	discoveryURL = "https://auth.x.ai/.well-known/openid-configuration"
	t.Cleanup(func() {
		httpClient = oldClient
		discoveryURL = oldDiscovery
	})

	mux.HandleFunc("/oauth/device/code", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code":       "dev-1",
			"user_code":         "ABCD-1234",
			"verification_uri":  "https://x.ai/device",
			"expires_in":        120,
			"interval":          1,
		})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		n := polls.Add(1)
		if n < 2 {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-xyz",
			"refresh_token": "refresh-xyz",
			"expires_in":    3600,
		})
	})

	ctx := context.Background()
	dc, err := RequestDeviceCode(ctx)
	require.NoError(t, err)
	require.Equal(t, "ABCD-1234", dc.UserCode)
	require.Equal(t, "https://x.ai/device", dc.VerificationURI)
	require.Equal(t, "https://auth.x.ai/oauth/token", dc.TokenEndpoint)

	tok, err := PollForToken(ctx, dc)
	require.NoError(t, err)
	require.Equal(t, "access-xyz", tok.AccessToken)
	require.Equal(t, "refresh-xyz", tok.RefreshToken)
	require.True(t, tok.ExpiresAt > 0)
}

func TestRefreshToken(t *testing.T) {
	// Not parallel: mutates package-level discovery/client vars.

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"device_authorization_endpoint": "https://auth.x.ai/oauth/device/code",
			"token_endpoint":                "https://auth.x.ai/oauth/token",
		})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		require.Equal(t, "old-refresh", r.Form.Get("refresh_token"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    1800,
		})
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	transport := rewriteTransport{base: srv.Client().Transport, target: srv.URL}
	oldClient := httpClient
	oldDiscovery := discoveryURL
	httpClient = &http.Client{Transport: transport, Timeout: 10 * time.Second}
	discoveryURL = "https://auth.x.ai/.well-known/openid-configuration"
	t.Cleanup(func() {
		httpClient = oldClient
		discoveryURL = oldDiscovery
	})

	tok, err := RefreshToken(context.Background(), "old-refresh")
	require.NoError(t, err)
	require.Equal(t, "new-access", tok.AccessToken)
	require.Equal(t, "new-refresh", tok.RefreshToken)
}

func TestRequireTrustedEndpoint(t *testing.T) {
	t.Parallel()
	require.NoError(t, requireTrustedXAIEndpoint("https://auth.x.ai/oauth/token"))
	require.NoError(t, requireTrustedXAIEndpoint("https://x.ai/device"))
	require.Error(t, requireTrustedXAIEndpoint("http://auth.x.ai/oauth/token"))
	require.Error(t, requireTrustedXAIEndpoint("https://evil.example/oauth/token"))
}

// rewriteTransport rewrites requests for auth.x.ai / x.ai hosts to the test server.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if host == "auth.x.ai" || host == "x.ai" {
		u, err := http.NewRequestWithContext(req.Context(), req.Method, t.target+req.URL.Path, req.Body)
		if err != nil {
			return nil, err
		}
		u.Header = req.Header.Clone()
		u.ContentLength = req.ContentLength
		req = u
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
