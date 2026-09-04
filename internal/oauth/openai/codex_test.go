package openai_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/stretchr/testify/require"
)

func makeTestJWT(payload map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	data, _ := json.Marshal(payload)
	claims := base64.RawURLEncoding.EncodeToString(data)
	return header + "." + claims + ".signature"
}

func TestParseJWTClaims(t *testing.T) {
	t.Parallel()

	jwt := makeTestJWT(map[string]any{
		"chatgpt_account_id":        "acc_12345",
		"chatgpt_compute_residency": "us",
	})

	claims, err := openai.ParseJWTClaims(jwt)
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.Equal(t, "acc_12345", openai.ExtractAccountID(claims))
	require.Equal(t, "us", openai.ExtractResidency(claims))
}

func TestParseJWTClaims_NestedAuth(t *testing.T) {
	t.Parallel()

	jwt := makeTestJWT(map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id":        "acc_nested",
			"chatgpt_compute_residency": "eu",
		},
	})

	claims, err := openai.ParseJWTClaims(jwt)
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.Equal(t, "acc_nested", openai.ExtractAccountID(claims))
	require.Equal(t, "eu", openai.ExtractResidency(claims))
}

func TestParseJWTClaims_OrganizationFallback(t *testing.T) {
	t.Parallel()

	jwt := makeTestJWT(map[string]any{
		"organizations": []map[string]any{
			{"id": "org_primary"},
		},
	})

	claims, err := openai.ParseJWTClaims(jwt)
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.Equal(t, "org_primary", openai.ExtractAccountID(claims))
}

func TestAuthorizeURL(t *testing.T) {
	t.Parallel()

	pkce := oauth.PKCE{
		Verifier:  "verifier-abc",
		Challenge: "challenge-xyz",
	}
	rawURL := openai.AuthorizeURL("http://localhost:1455/auth/callback", pkce, "state-123")
	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	q := u.Query()
	require.Equal(t, "code", q.Get("response_type"))
	require.Equal(t, openai.ClientID, q.Get("client_id"))
	require.Equal(t, "http://localhost:1455/auth/callback", q.Get("redirect_uri"))
	require.Equal(t, "challenge-xyz", q.Get("code_challenge"))
	require.Equal(t, "S256", q.Get("code_challenge_method"))
	require.Equal(t, "state-123", q.Get("state"))
}

func TestRefreshToken(t *testing.T) {
	origClient := openai.HTTPClient
	defer func() { openai.HTTPClient = origClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		require.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		require.Equal(t, "rt-123", r.Form.Get("refresh_token"))
		require.Equal(t, openai.ClientID, r.Form.Get("client_id"))

		jwt := makeTestJWT(map[string]any{
			"chatgpt_account_id": "acc-refreshed",
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at-new",
			"refresh_token": "rt-new",
			"id_token":      jwt,
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	// Redirect request to mock server via Transport
	openai.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			targetURL, _ := url.Parse(server.URL + req.URL.Path)
			req.URL = targetURL
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	tok, err := openai.RefreshToken(context.Background(), "rt-123")
	require.NoError(t, err)
	require.NotNil(t, tok)
	require.Equal(t, "at-new", tok.AccessToken)
	require.Equal(t, "rt-new", tok.RefreshToken)
	require.Equal(t, "acc-refreshed", tok.AccountID)
}

func TestRefreshToken_PreservesExistingRefreshToken(t *testing.T) {
	origClient := openai.HTTPClient
	defer func() { openai.HTTPClient = origClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "rt-existing", r.Form.Get("refresh_token"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at-new",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	openai.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			targetURL, _ := url.Parse(server.URL + req.URL.Path)
			req.URL = targetURL
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	tok, err := openai.RefreshToken(context.Background(), "rt-existing")
	require.NoError(t, err)
	require.Equal(t, "rt-existing", tok.RefreshToken)
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
