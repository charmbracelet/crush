package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// newCredentialsController builds a controller whose server config knows
// the hyper provider.
func newCredentialsController(t *testing.T, network string) *controllerV1 {
	t.Helper()
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("hyper", config.ProviderConfig{ID: "hyper", APIKey: "disk"})
	providers.Set("gemini", config.ProviderConfig{ID: "gemini", APIKey: "disk"})
	providers.Set("xai", config.ProviderConfig{ID: "xai", OAuthToken: &oauth.Token{AccessToken: "local"}})
	cfg := config.NewTestStore(&config.Config{Providers: providers})
	s := &Server{network: network}
	s.backend = backend.New(context.Background(), cfg, nil)
	return &controllerV1{backend: s.backend, server: s}
}

func doCredential(t *testing.T, c *controllerV1, method, provider, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, "/v1/credentials/"+provider, strings.NewReader(body))
	req.SetPathValue("provider", provider)
	rec := httptest.NewRecorder()
	if method == http.MethodPut {
		c.handlePutCredential(rec, req)
	} else {
		c.handleDeleteCredential(rec, req)
	}
	return rec
}

func TestPutCredential(t *testing.T) {
	t.Parallel()
	c := newCredentialsController(t, "unix")

	rec := doCredential(t, c, http.MethodPut, "hyper", `{"access_token":"tok","expires_at":"2030-01-01T00:00:00Z"}`)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	cred, ok := c.backend.ManagedCredentials().Get("hyper")
	require.True(t, ok)
	require.Equal(t, "tok", cred.AccessToken)
	require.Equal(t, 2030, cred.ExpiresAt.Year())
	pc, _ := c.backend.Config().Config().Providers.Get("hyper")
	require.Equal(t, config.ManagedKindAPIKey, pc.ManagedKind, "applied to the server config")
	require.Equal(t, "disk", pc.APIKey, "the token never lands on the provider config")
}

func TestPutCredential_BadRequests(t *testing.T) {
	t.Parallel()
	c := newCredentialsController(t, "unix")
	for name, body := range map[string]string{
		"not json":      `nope`,
		"missing token": `{"expires_at":"2030-01-01T00:00:00Z"}`,
		"bad expiry":    `{"access_token":"tok","expires_at":"tomorrow"}`,
	} {
		rec := doCredential(t, c, http.MethodPut, "hyper", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
	require.False(t, c.backend.ManagedCredentials().IsManaged("hyper"))
}

func TestPutCredential_UnknownProvider(t *testing.T) {
	t.Parallel()
	c := newCredentialsController(t, "unix")
	rec := doCredential(t, c, http.MethodPut, "nope", `{"access_token":"tok"}`)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCredentials_RefusedOnTCP(t *testing.T) {
	t.Parallel()
	for _, network := range []string{"tcp", "tcp4", "tcp6"} {
		c := newCredentialsController(t, network)
		require.Equal(t, http.StatusForbidden,
			doCredential(t, c, http.MethodPut, "hyper", `{"access_token":"tok"}`).Code, network)
		require.Equal(t, http.StatusForbidden,
			doCredential(t, c, http.MethodDelete, "hyper", "").Code, network)
		require.False(t, c.backend.ManagedCredentials().IsManaged("hyper"), network)
	}
}

// GET /v1/config must never echo a pushed credential.
func TestGetConfig_OmitsManagedToken(t *testing.T) {
	t.Parallel()
	c := newCredentialsController(t, "unix")
	const token = "$(echo pwned)-secret"
	require.Equal(t, http.StatusNoContent,
		doCredential(t, c, http.MethodPut, "hyper", `{"access_token":"`+token+`"}`).Code)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/config", nil)
	rec := httptest.NewRecorder()
	c.handleGetConfig(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "pwned")
	require.NotContains(t, rec.Body.String(), "secret")
}

func TestDeleteCredential(t *testing.T) {
	t.Parallel()
	c := newCredentialsController(t, "npipe")
	require.Equal(t, http.StatusNotFound, doCredential(t, c, http.MethodDelete, "hyper", "").Code)

	require.Equal(t, http.StatusNoContent,
		doCredential(t, c, http.MethodPut, "hyper", `{"access_token":"tok"}`).Code)
	require.Equal(t, http.StatusNoContent, doCredential(t, c, http.MethodDelete, "hyper", "").Code)
	require.False(t, c.backend.ManagedCredentials().IsManaged("hyper"))
}

func TestWrapEvent_CredentialsRequired(t *testing.T) {
	t.Parallel()
	p := wrapEvent(pubsub.Event[notify.Notification]{
		Type: pubsub.CreatedEvent,
		Payload: notify.Notification{
			Type:       notify.TypeCredentialsRequired,
			ProviderID: "hyper",
			Reason:     notify.CredentialsReasonExpiring,
		},
	})
	require.NotNil(t, p)
	require.Equal(t, "credentials_required", p.Type)
	require.JSONEq(t, `{"type":"created","payload":{"provider":"hyper","reason":"expiring"}}`, string(p.Payload))
}

// The kind is decided at push time: given explicitly, or inferred from
// whether the provider is signed in by OAuth. A kind the provider cannot
// take, or an unknown one, is refused.
func TestPutCredential_Kind(t *testing.T) {
	t.Parallel()
	c := newCredentialsController(t, "unix")
	kindOf := func(provider string) config.ManagedCredentialKind {
		cred, ok := c.backend.ManagedCredentials().Get(provider)
		require.True(t, ok)
		return cred.Kind
	}

	require.Equal(t, http.StatusNoContent, doCredential(t, c, http.MethodPut, "hyper", `{"access_token":"t"}`).Code)
	require.Equal(t, config.ManagedKindAPIKey, kindOf("hyper"), "inferred from the key")
	require.Equal(t, http.StatusNoContent, doCredential(t, c, http.MethodPut, "xai", `{"access_token":"t"}`).Code)
	require.Equal(t, config.ManagedKindOAuth, kindOf("xai"), "inferred from the login")
	require.Equal(t, http.StatusNoContent, doCredential(t, c, http.MethodPut, "hyper", `{"access_token":"t","kind":"oauth"}`).Code)
	require.Equal(t, config.ManagedKindOAuth, kindOf("hyper"), "explicit kind wins")

	require.Equal(t, http.StatusBadRequest, doCredential(t, c, http.MethodPut, "gemini", `{"access_token":"t","kind":"oauth"}`).Code)
	require.False(t, c.backend.ManagedCredentials().IsManaged("gemini"))
	require.Equal(t, http.StatusBadRequest, doCredential(t, c, http.MethodPut, "hyper", `{"access_token":"t","kind":"bearer"}`).Code)
}
