package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeviceCodeFlow(t *testing.T) {
	// Not parallel: mutates package-level endpoint vars.

	var polls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/accounts/deviceauth/usercode", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, ClientID, body["client_id"])
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_auth_id": "auth-1",
			"user_code":      "WXYZ-9876",
			"interval":       "1",
		})
	})
	mux.HandleFunc("/api/accounts/deviceauth/token", func(w http.ResponseWriter, r *http.Request) {
		n := polls.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"authorization_code": "authcode",
			"code_challenge":     "challenge",
			"code_verifier":      "verifier",
		})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		require.Equal(t, ClientID, r.Form.Get("client_id"))
		require.Equal(t, "authcode", r.Form.Get("code"))
		require.Equal(t, "verifier", r.Form.Get("code_verifier"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-1",
			"refresh_token": "refresh-1",
			"expires_in":    3600,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	patchEndpoints(t, srv.URL)

	ctx := context.Background()
	dc, err := RequestDeviceCode(ctx)
	require.NoError(t, err)
	require.Equal(t, "WXYZ-9876", dc.UserCode)
	require.Equal(t, "auth-1", dc.DeviceAuthID)

	tok, err := PollForToken(ctx, dc)
	require.NoError(t, err)
	require.Equal(t, "access-1", tok.AccessToken)
	require.Equal(t, "refresh-1", tok.RefreshToken)
	require.True(t, tok.ExpiresAt > time.Now().Unix())
}

func TestRefreshToken(t *testing.T) {
	// Not parallel: mutates package-level endpoint vars.

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		require.Equal(t, "old-rt", r.Form.Get("refresh_token"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-at",
			"refresh_token": "new-rt",
			"expires_in":    7200,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	patchEndpoints(t, srv.URL)

	tok, err := RefreshToken(context.Background(), "old-rt")
	require.NoError(t, err)
	require.Equal(t, "new-at", tok.AccessToken)
	require.Equal(t, "new-rt", tok.RefreshToken)
}

func TestRefreshTokenPreservesRefreshWhenOmitted(t *testing.T) {
	// Not parallel: mutates package-level endpoint vars.

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-at",
			"expires_in":   60,
			// refresh_token intentionally omitted
		})
	}))
	t.Cleanup(srv.Close)
	patchEndpoints(t, srv.URL)

	tok, err := RefreshToken(context.Background(), "keep-me")
	require.NoError(t, err)
	require.Equal(t, "new-at", tok.AccessToken)
	require.Equal(t, "keep-me", tok.RefreshToken)
}

func TestChatGPTAccountID(t *testing.T) {
	t.Parallel()

	payload := base64.RawURLEncoding.EncodeToString([]byte(`{
		"https://api.openai.com/auth": {"chatgpt_account_id": "acct_123"}
	}`))
	jwt := "hdr." + payload + ".sig"
	require.Equal(t, "acct_123", ChatGPTAccountID(jwt))
	require.Equal(t, "", ChatGPTAccountID("not-a-jwt"))
}

func patchEndpoints(t *testing.T, base string) {
	t.Helper()
	oldUser, oldPoll, oldToken, oldClient := userCodeURL, tokenPollURL, oauthTokenURL, httpClient
	userCodeURL = base + "/api/accounts/deviceauth/usercode"
	tokenPollURL = base + "/api/accounts/deviceauth/token"
	oauthTokenURL = base + "/oauth/token"
	httpClient = &http.Client{Timeout: 10 * time.Second}
	t.Cleanup(func() {
		userCodeURL, tokenPollURL, oauthTokenURL, httpClient = oldUser, oldPoll, oldToken, oldClient
	})
}
