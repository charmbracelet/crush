package orcarouter

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthorizationUsesS256AndExchangesAtAuthOrigin(t *testing.T) {
	var exchanged struct {
		Code                string `json:"code"`
		Verifier            string `json:"code_verifier"`
		CodeChallengeMethod string `json:"code_challenge_method"`
	}
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/auth/keys", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&exchanged))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"key":"sk-orca-test","scope":"api"}`))
	}))
	defer authServer.Close()
	t.Setenv("ORCA_AUTH_BASE_URL", authServer.URL)
	t.Setenv("ORCA_BASE_URL", "")

	authorization, err := StartAuthorization()
	require.NoError(t, err)
	defer authorization.Cancel()
	authorizeURL, err := url.Parse(authorization.URL)
	require.NoError(t, err)
	require.Equal(t, authServer.URL+"/auth", authorizeURL.Scheme+"://"+authorizeURL.Host+authorizeURL.Path)
	require.Equal(t, "S256", authorizeURL.Query().Get("code_challenge_method"))
	require.Equal(t, "Crush", authorizeURL.Query().Get("app_name"))
	require.Equal(t, "api", authorizeURL.Query().Get("scope"))
	require.NotEmpty(t, authorizeURL.Query().Get("state"))
	require.NotEmpty(t, authorizeURL.Query().Get("code_challenge"))
	require.NotContains(t, authorization.URL, "code_verifier")

	callbackURL, err := url.Parse(authorizeURL.Query().Get("callback_url"))
	require.NoError(t, err)
	q := callbackURL.Query()
	q.Set("code", "one-time-code")
	q.Set("state", authorizeURL.Query().Get("state"))
	callbackURL.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, callbackURL.String(), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	callbackBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "no-referrer", resp.Header.Get("Referrer-Policy"))
	require.Contains(t, string(callbackBody), "history.replaceState")
	require.Contains(t, string(callbackBody), "window.close()")

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	key, scope, err := authorization.Wait(ctx)
	require.NoError(t, err)
	require.Equal(t, "sk-orca-test", key)
	require.Equal(t, "api", scope)
	require.Equal(t, "one-time-code", exchanged.Code)
	require.Equal(t, "S256", exchanged.CodeChallengeMethod)
	require.NotEmpty(t, exchanged.Verifier)
	require.NotContains(t, authorization.URL, exchanged.Verifier)
	digest := sha256.Sum256([]byte(exchanged.Verifier))
	require.Equal(t, authorizeURL.Query().Get("code_challenge"), base64.RawURLEncoding.EncodeToString(digest[:]))
}

func TestAuthorizationRejectsMismatchedStateWithoutExchange(t *testing.T) {
	exchanges := 0
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		exchanges++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer authServer.Close()
	t.Setenv("ORCA_AUTH_BASE_URL", authServer.URL)
	t.Setenv("ORCA_BASE_URL", "")

	authorization, err := StartAuthorization()
	require.NoError(t, err)
	defer authorization.Cancel()
	authorizeURL, err := url.Parse(authorization.URL)
	require.NoError(t, err)
	callbackURL, err := url.Parse(authorizeURL.Query().Get("callback_url"))
	require.NoError(t, err)
	q := callbackURL.Query()
	q.Set("code", "must-not-be-exchanged")
	q.Set("state", "wrong-state")
	callbackURL.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, callbackURL.String(), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	callbackBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, string(callbackBody), "history.replaceState")
	require.NotContains(t, string(callbackBody), "window.close()")

	_, _, err = authorization.Wait(t.Context())
	require.ErrorContains(t, err, "state mismatch")
	require.Zero(t, exchanges)
}

func TestAuthorizationCancelReleasesWaiter(t *testing.T) {
	authServer := httptest.NewServer(http.NotFoundHandler())
	defer authServer.Close()
	t.Setenv("ORCA_AUTH_BASE_URL", authServer.URL)
	t.Setenv("ORCA_BASE_URL", "")

	authorization, err := StartAuthorization()
	require.NoError(t, err)
	authorization.Cancel()
	_, _, err = authorization.Wait(t.Context())
	require.ErrorIs(t, err, context.Canceled)
}

func TestSuccessfulCallbackPageAttemptsToCloseBrowser(t *testing.T) {
	page := callbackPage("Authorization received. Return to Crush to continue.", true)
	require.Contains(t, page, "history.replaceState")
	require.Contains(t, page, "window.close()")
}

func TestFailedCallbackPageStaysOpenAndEscapesMessage(t *testing.T) {
	page := callbackPage(`<script>alert("unsafe")</script>`, false)
	require.Contains(t, page, "history.replaceState")
	require.NotContains(t, page, "window.close()")
	require.NotContains(t, page, `<script>alert("unsafe")</script>`)
	require.Contains(t, page, `&lt;script&gt;alert(&#34;unsafe&#34;)&lt;/script&gt;`)
}
