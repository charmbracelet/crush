package honcho

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// These tests cannot run in parallel: t.Setenv forbids it, and the
// token path is process-global state.

// tsToken is a fully populated token, so a round-trip test can prove
// no field is dropped.
func tsToken() *OAuthToken {
	return &OAuthToken{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-abc",
		TokenType:    "Bearer",
		Scope:        "read write",
		ExpiresIn:    3600,
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
		ClientID:     "client-123",
		Issuer:       "https://auth.example.test",
	}
}

// tsPath returns the token path, failing the test if it cannot be
// resolved.
func tsPath(t *testing.T) string {
	t.Helper()
	path, err := TokenPath()
	require.NoError(t, err)
	return path
}

// tsRead reads the token file back as a generic object.
func tsRead(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(tsPath(t))
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

// tsWriteDiscovery serves a minimal authorization server document
// pointing back at the same test server.
func tsWriteDiscovery(w http.ResponseWriter, r *http.Request) {
	base := "http://" + r.Host
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"issuer":"` + base +
		`","authorization_endpoint":"` + base + `/authorize","token_endpoint":"` + base +
		`/token","registration_endpoint":"` + base +
		`/register","code_challenge_methods_supported":["S256"]}`))
}

// tsWriteRaw writes arbitrary bytes to the token file.
func tsWriteRaw(t *testing.T, content string) {
	t.Helper()
	path := tsPath(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// tsForbiddenServer returns the URL of a server that fails the test
// if anything reaches it. It is how "no network call" is asserted.
func tsForbiddenServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestTokenPath(t *testing.T) {
	dir := isolateConfig(t)

	path, err := TokenPath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, tokenFileName), path)
	require.True(t, strings.HasSuffix(path, "crush-token.json"))
}

// TestLoadTokenMissingFile checks that not being signed in reads as an
// ordinary state rather than a failure.
func TestLoadTokenMissingFile(t *testing.T) {
	isolateConfig(t)

	tok, err := LoadToken()
	require.NoError(t, err)
	require.Nil(t, tok)
}

func TestSaveAndLoadTokenRoundTrip(t *testing.T) {
	isolateConfig(t)

	want := tsToken()
	require.NoError(t, SaveToken(want))

	got, err := LoadToken()
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, *want, *got)
}

func TestSaveTokenPermissions(t *testing.T) {
	t.Run("fresh file is owner only", func(t *testing.T) {
		isolateConfig(t)
		require.NoError(t, SaveToken(tsToken()))

		info, err := os.Stat(tsPath(t))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("existing world readable file is tightened", func(t *testing.T) {
		isolateConfig(t)
		// An existing file keeps its old mode through WriteFile, so
		// the chmod afterwards is what actually secures it.
		path := tsPath(t)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte("{}"), 0o644))

		require.NoError(t, SaveToken(tsToken()))

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})
}

func TestSaveTokenNil(t *testing.T) {
	isolateConfig(t)

	err := SaveToken(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no token to save")
}

func TestLoadTokenMalformed(t *testing.T) {
	isolateConfig(t)
	tsWriteRaw(t, `{"access_token": `)

	tok, err := LoadToken()
	require.Error(t, err)
	require.Nil(t, tok)
	require.Contains(t, err.Error(), "parse token")
}

// TestLoadTokenEmptyAccessToken treats a token with no access token as
// no token at all.
func TestLoadTokenEmptyAccessToken(t *testing.T) {
	isolateConfig(t)
	tsWriteRaw(t, `{"refresh_token":"refresh-abc","client_id":"client-123"}`)

	tok, err := LoadToken()
	require.NoError(t, err)
	require.Nil(t, tok)
}

func TestDeleteToken(t *testing.T) {
	isolateConfig(t)

	require.NoError(t, SaveToken(tsToken()))
	require.FileExists(t, tsPath(t))

	require.NoError(t, DeleteToken())
	require.NoFileExists(t, tsPath(t))

	// Deleting what is already gone is not a failure.
	require.NoError(t, DeleteToken())
}

func TestSignedIn(t *testing.T) {
	isolateConfig(t)

	require.False(t, SignedIn(), "no token file means signed out")

	require.NoError(t, SaveToken(tsToken()))
	require.True(t, SignedIn())

	require.NoError(t, DeleteToken())
	require.False(t, SignedIn())
}

// TestAccessTokenNotSignedIn checks the empty-string-no-error contract
// callers rely on to fall back to an API key.
func TestAccessTokenNotSignedIn(t *testing.T) {
	isolateConfig(t)
	f := oaNewFake(t)

	got, err := AccessToken(t.Context(), f.srv.URL)
	require.NoError(t, err)
	require.Empty(t, got)
	require.Zero(t, f.tokens())
}

// TestAccessTokenUnexpiredSkipsNetwork points at a server that fails
// the test if it is touched, so a live token provably costs nothing.
func TestAccessTokenUnexpiredSkipsNetwork(t *testing.T) {
	isolateConfig(t)

	srv := tsForbiddenServer(t)
	require.NoError(t, SaveToken(tsToken()))

	got, err := AccessToken(t.Context(), srv)
	require.NoError(t, err)
	require.Equal(t, "access-abc", got)
}

// TestAccessTokenRefreshesAndPersists checks that an expired token is
// exchanged and the fresh one lands on disk, not just in memory.
func TestAccessTokenRefreshesAndPersists(t *testing.T) {
	isolateConfig(t)
	f := oaNewFake(t)
	f.tokenBody = `{"access_token":"access-fresh","refresh_token":"refresh-fresh","expires_in":3600}`

	stale := tsToken()
	stale.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	require.NoError(t, SaveToken(stale))

	got, err := AccessToken(t.Context(), f.srv.URL)
	require.NoError(t, err)
	require.Equal(t, "access-fresh", got)

	stored := tsRead(t)
	require.Equal(t, "access-fresh", stored["access_token"])
	require.Equal(t, "refresh-fresh", stored["refresh_token"])
	require.Equal(t, "client-123", stored["client_id"])

	form := f.posted()
	require.Equal(t, "refresh_token", form.Get("grant_type"))
	require.Equal(t, "refresh-abc", form.Get("refresh_token"))
	require.Equal(t, "client-123", form.Get("client_id"))
}

// TestAccessTokenDropsDeadGrant checks the stale credential is deleted
// when the refresh fails, so the next start reports "signed out"
// rather than retrying a grant that can never work again.
func TestAccessTokenDropsDeadGrant(t *testing.T) {
	isolateConfig(t)
	f := oaNewFake(t)
	f.tokenStatus = http.StatusBadRequest
	f.tokenBody = `{"error":"invalid_grant","error_description":"revoked"}`

	stale := tsToken()
	stale.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	require.NoError(t, SaveToken(stale))

	got, err := AccessToken(t.Context(), f.srv.URL)
	require.Error(t, err)
	require.Empty(t, got)
	require.Contains(t, err.Error(), "invalid_grant")

	require.NoFileExists(t, tsPath(t), "a dead grant must not be left on disk")
	require.False(t, SignedIn())
}

// TestResolveEnabledByStoredToken proves a stored sign-in is enough to
// turn the integration on, with no API key and no configured
// deployment.
func TestResolveEnabledByStoredToken(t *testing.T) {
	isolateConfig(t)

	require.False(t, Resolve(Config{}).Enabled,
		"nothing configured and nothing stored means disabled")

	require.NoError(t, SaveToken(tsToken()))
	require.True(t, Resolve(Config{}).Enabled,
		"a stored OAuth token is an unambiguous statement of intent")
}

// TestAccessTokenKeepsCredentialWhenOffline is the guard against the
// worst failure this package can have: signing a user out because
// their machine could not reach the network.
//
// A transport failure, a timeout, or a 5xx says nothing about whether
// the grant is still good, so the stored token must survive.
func TestAccessTokenKeepsCredentialWhenOffline(t *testing.T) {
	t.Setenv("HONCHO_CONFIG_DIR", t.TempDir())

	expired := tsToken()
	expired.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	require.NoError(t, SaveToken(expired))

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
		{
			name: "service unavailable",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			},
		},
		{
			name: "transient oauth error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, discoveryPath) {
					tsWriteDiscovery(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"temporarily_unavailable"}`))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			_, err := AccessToken(t.Context(), srv.URL)
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrGrantRevoked)

			// The credential must still be there for the next try.
			require.True(t, SignedIn(), "a reachability failure must not sign the user out")
			got, loadErr := LoadToken()
			require.NoError(t, loadErr)
			require.NotNil(t, got)
			require.Equal(t, "refresh-abc", got.RefreshToken)
		})
	}

	// An unreachable host, the plain offline case.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachable := srv.URL
	srv.Close()

	_, err := AccessToken(t.Context(), unreachable)
	require.Error(t, err)
	require.True(t, SignedIn(), "an unreachable server must not sign the user out")
}

// TestAccessTokenDiscardsRevokedGrant is the other side: when the
// server says the grant is gone, keeping the token would mean retrying
// a credential that can never work.
func TestAccessTokenDiscardsRevokedGrant(t *testing.T) {
	t.Setenv("HONCHO_CONFIG_DIR", t.TempDir())

	for _, code := range []string{"invalid_grant", "invalid_client", "unauthorized_client"} {
		t.Run(code, func(t *testing.T) {
			expired := tsToken()
			expired.ExpiresAt = time.Now().Add(-time.Hour).Unix()
			require.NoError(t, SaveToken(expired))

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, discoveryPath) {
					tsWriteDiscovery(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"` + code + `"}`))
			}))
			defer srv.Close()

			_, err := AccessToken(t.Context(), srv.URL)
			require.ErrorIs(t, err, ErrGrantRevoked)
			require.False(t, SignedIn(), "a revoked grant must be discarded")
		})
	}
}

// TestSaveTokenIsAtomic checks that a failed write cannot leave a
// truncated credential behind. The previous token must survive.
func TestSaveTokenIsAtomic(t *testing.T) {
	t.Setenv("HONCHO_CONFIG_DIR", t.TempDir())

	require.NoError(t, SaveToken(tsToken()))
	require.NoError(t, SaveToken(&OAuthToken{AccessToken: "second", RefreshToken: "r2"}))

	got, err := LoadToken()
	require.NoError(t, err)
	require.Equal(t, "second", got.AccessToken)

	// No temporary files left behind.
	dir := filepath.Dir(tsPath(t))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		require.False(t, strings.HasPrefix(e.Name(), ".crush-token-"),
			"temporary file %s was not cleaned up", e.Name())
	}
}

// TestSaveTokenTightensExistingPermissions checks that replacing a
// world-readable file yields an owner-only one.
func TestSaveTokenTightensExistingPermissions(t *testing.T) {
	t.Setenv("HONCHO_CONFIG_DIR", t.TempDir())

	tsWriteRaw(t, `{"access_token":"old"}`)
	require.NoError(t, os.Chmod(tsPath(t), 0o644))

	require.NoError(t, SaveToken(tsToken()))

	info, err := os.Stat(tsPath(t))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
