package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestCreateAuthorizationFlow(t *testing.T) {
	t.Parallel()
	flow, err := CreateAuthorizationFlow()
	require.NoError(t, err)
	require.NotEmpty(t, flow.URL)
	require.NotEmpty(t, flow.State)
	require.NotEmpty(t, flow.Verifier)

	parsed, err := url.Parse(flow.URL)
	require.NoError(t, err)
	require.Equal(t, "https", parsed.Scheme)
	require.Equal(t, "auth.openai.com", parsed.Host)
	require.Equal(t, "/oauth/authorize", parsed.Path)

	query := parsed.Query()
	require.Equal(t, "code", query.Get("response_type"))
	require.Equal(t, oauthClientID, query.Get("client_id"))
	require.Equal(t, RedirectURL, query.Get("redirect_uri"))
	require.Equal(t, Scope, query.Get("scope"))
	require.NotEmpty(t, query.Get("code_challenge"))
	require.Equal(t, "S256", query.Get("code_challenge_method"))
	require.Equal(t, flow.State, query.Get("state"))
	require.Equal(t, "true", query.Get("id_token_add_organizations"))
	require.Equal(t, "true", query.Get("codex_cli_simplified_flow"))
	require.Equal(t, HeaderOriginatorVal, query.Get("originator"))
}

func TestExchangeAuthorizationCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		err := r.ParseForm()
		require.NoError(t, err)
		require.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		require.Equal(t, oauthClientID, r.Form.Get("client_id"))
		require.Equal(t, "test-code", r.Form.Get("code"))
		require.Equal(t, "test-verifier", r.Form.Get("code_verifier"))
		require.Equal(t, RedirectURL, r.Form.Get("redirect_uri"))

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"expires_in":    3600,
		})
		require.NoError(t, err)
	}))
	defer server.Close()

	originalTokenURL := tokenURL
	defer func() { tokenURL = originalTokenURL }()
	tokenURL = server.URL

	token, err := ExchangeAuthorizationCode(context.Background(), "test-code", "test-verifier")
	require.NoError(t, err)
	require.NotNil(t, token)
	require.Equal(t, "test-access-token", token.AccessToken)
	require.Equal(t, "test-refresh-token", token.RefreshToken)
	require.Equal(t, 3600, token.ExpiresIn)
}

func TestRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		err := r.ParseForm()
		require.NoError(t, err)
		require.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		require.Equal(t, oauthClientID, r.Form.Get("client_id"))
		require.Equal(t, "test-refresh-token", r.Form.Get("refresh_token"))

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    3600,
		})
		require.NoError(t, err)
	}))
	defer server.Close()

	originalTokenURL := tokenURL
	defer func() { tokenURL = originalTokenURL }()
	tokenURL = server.URL

	token, err := RefreshToken(context.Background(), "test-refresh-token")
	require.NoError(t, err)
	require.NotNil(t, token)
	require.Equal(t, "new-access-token", token.AccessToken)
	require.Equal(t, "new-refresh-token", token.RefreshToken)
}

func TestExtractAccountID(t *testing.T) {
	t.Parallel()
	// This is a simplified, decoded JWT payload for testing purposes.
	// In a real scenario, this would be part of a full JWT.
	payload := `{"https://api.openai.com/auth": {"chatgpt_account_id": "test-account-id"}}`
	encodedPayload := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	accountID, err := ExtractAccountID(encodedPayload)
	require.NoError(t, err)
	require.Equal(t, "test-account-id", accountID)

	_, err = ExtractAccountID("invalid-jwt")
	require.Error(t, err)
}

func TestSetExpiresAt(t *testing.T) {
	t.Parallel()
	token := &oauth.Token{
		ExpiresIn: 3600,
	}
	token.SetExpiresAt()
	require.WithinDuration(t, time.Now().Add(time.Hour), time.Unix(token.ExpiresAt, 0), time.Second)
}
