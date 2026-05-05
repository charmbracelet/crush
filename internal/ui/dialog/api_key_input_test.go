package dialog

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestAPIKeyStateForVerifyErr pins the mapping between TestConnection errors
// and the dialog state the UI should transition into. In particular, an
// [config.ErrValidationUnsupported] error must yield the unverified state
// (so the UI shows "saved (not verified)" instead of "invalid").
func TestAPIKeyStateForVerifyErr(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want APIKeyInputState
	}{
		"nilIsVerified": {
			err:  nil,
			want: APIKeyInputStateVerified,
		},
		"unsupportedIsUnverified": {
			err:  config.ErrValidationUnsupported,
			want: APIKeyInputStateUnverified,
		},
		"wrappedUnsupportedIsUnverified": {
			err:  fmt.Errorf("probing provider: %w", config.ErrValidationUnsupported),
			want: APIKeyInputStateUnverified,
		},
		"plainErrorIsError": {
			err:  errors.New("bad key"),
			want: APIKeyInputStateError,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, apiKeyStateForVerifyErr(tc.err))
		})
	}
}

// TestProviderConfigForVerifyPropagatesDefaultHeaders locks in the UI
// contract that any catwalk-declared DefaultHeaders are copied into the
// ProviderConfig.ExtraHeaders used for the validation probe. Without this,
// providers that require routing/tenant headers (e.g. DefaultHeaders
// supplied by the catwalk provider definition) would probe with a stripped
// header set and potentially be misclassified as "not verified".
func TestProviderConfigForVerifyPropagatesDefaultHeaders(t *testing.T) {
	t.Parallel()

	provider := catwalk.Provider{
		ID:          "test-provider",
		Name:        "Test Provider",
		Type:        catwalk.TypeOpenAICompat,
		APIEndpoint: "https://example.invalid",
		DefaultHeaders: map[string]string{
			"X-Tenant":   "acme",
			"X-Route":    "primary",
			"X-Shared":   "from-default",
			"User-Agent": "crush-test",
		},
	}
	cfg := providerConfigForVerify(provider, "sk-test")

	require.Equal(t, string(provider.ID), cfg.ID)
	require.Equal(t, provider.Name, cfg.Name)
	require.Equal(t, provider.Type, cfg.Type)
	require.Equal(t, provider.APIEndpoint, cfg.BaseURL)
	require.Equal(t, "sk-test", cfg.APIKey)
	require.Equal(t, provider.DefaultHeaders, cfg.ExtraHeaders,
		"DefaultHeaders must be propagated to ExtraHeaders")

	// Mutating the returned config must not leak back into the provider
	// definition (the dialog reuses the provider value across retries).
	cfg.ExtraHeaders["X-Tenant"] = "mutated"
	require.Equal(t, "acme", provider.DefaultHeaders["X-Tenant"],
		"providerConfigForVerify must copy DefaultHeaders, not alias them")
}

func TestProviderConfigForVerifyWithNoDefaultHeaders(t *testing.T) {
	t.Parallel()

	provider := catwalk.Provider{
		ID:          "test-provider",
		Type:        catwalk.TypeOpenAICompat,
		APIEndpoint: "https://example.invalid",
	}
	cfg := providerConfigForVerify(provider, "sk-test")
	require.Nil(t, cfg.ExtraHeaders,
		"no DefaultHeaders should yield no ExtraHeaders allocation")
}

// TestProviderConfigForVerifyHeadersHitTheWire is an end-to-end UI-level
// check: after providerConfigForVerify builds the probe config, calling
// TestConnection against a local server must deliver the DefaultHeaders on
// the outbound request. This guards against silent regressions where the
// header map is dropped between the dialog and the HTTP layer.
func TestProviderConfigForVerifyHeadersHitTheWire(t *testing.T) {
	t.Parallel()

	var captured http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		_, _ = io.Copy(io.Discard, r.Body)
		// 400 on the malformed-body chat-completions probe means "auth
		// passed, schema rejected" for the Synthetic-style override.
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	provider := catwalk.Provider{
		ID:          catwalk.InferenceProviderSynthetic,
		Name:        "Synthetic",
		Type:        catwalk.TypeOpenAICompat,
		APIEndpoint: srv.URL,
		DefaultHeaders: map[string]string{
			"X-Tenant": "acme",
			"X-Route":  "primary",
		},
	}
	cfg := providerConfigForVerify(provider, "sk-test")
	require.NoError(t, cfg.TestConnection(config.IdentityResolver()))

	require.NotNil(t, captured, "probe must have reached the test server")
	require.Equal(t, "acme", captured.Get("X-Tenant"))
	require.Equal(t, "primary", captured.Get("X-Route"))
	// Probe-defined headers should still be present alongside the
	// DefaultHeaders.
	require.Equal(t, "Bearer sk-test", captured.Get("Authorization"))
}

// TestAPIKeyInputStatesAreDistinct guards against someone accidentally making
// APIKeyInputStateUnverified equal to one of the other states (which would
// silently collapse the "saved (not verified)" path onto "validated" or
// "invalid").
func TestAPIKeyInputStatesAreDistinct(t *testing.T) {
	t.Parallel()

	states := []APIKeyInputState{
		APIKeyInputStateInitial,
		APIKeyInputStateVerifying,
		APIKeyInputStateVerified,
		APIKeyInputStateUnverified,
		APIKeyInputStateError,
	}
	seen := map[APIKeyInputState]struct{}{}
	for _, s := range states {
		_, dup := seen[s]
		require.False(t, dup, "state %d declared twice", s)
		seen[s] = struct{}{}
	}
}
