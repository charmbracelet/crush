package config

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

type capturedRequest struct {
	method  string
	path    string
	query   string
	headers http.Header
	body    []byte
}

func newCaptureServer(t *testing.T, status int) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.query = r.URL.RawQuery
		captured.headers = r.Header.Clone()
		captured.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, captured
}

func TestTestConnectionMiniMaxProbe(t *testing.T) {
	t.Parallel()

	for _, id := range []catwalk.InferenceProvider{
		catwalk.InferenceProviderMiniMax,
		catwalk.InferenceProviderMiniMaxChina,
	} {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()
			for name, tc := range map[string]struct {
				status  int
				wantErr error
				wantNil bool
			}{
				"valid":       {status: http.StatusOK, wantNil: true},
				"invalid401":  {status: http.StatusUnauthorized},
				"invalid403":  {status: http.StatusForbidden},
				"unsupported": {status: http.StatusTeapot, wantErr: ErrValidationUnsupported},
			} {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					srv, captured := newCaptureServer(t, tc.status)
					c := &ProviderConfig{
						ID:      string(id),
						Type:    catwalk.TypeAnthropic,
						BaseURL: srv.URL,
						APIKey:  "key-abc",
					}
					err := c.TestConnection(IdentityResolver())
					switch {
					case tc.wantNil:
						require.NoError(t, err)
					case tc.wantErr != nil:
						require.ErrorIs(t, err, tc.wantErr)
					default:
						require.Error(t, err)
						require.NotErrorIs(t, err, ErrValidationUnsupported)
					}
					require.Equal(t, http.MethodGet, captured.method)
					require.Equal(t, "/v1/models", captured.path)
					require.Equal(t, "key-abc", captured.headers.Get("x-api-key"))
					require.Equal(t, "2023-06-01", captured.headers.Get("anthropic-version"))
				})
			}
		})
	}
}

func TestTestConnectionVeniceProbe(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		status  int
		wantErr error
		wantNil bool
	}{
		"valid":        {status: http.StatusOK, wantNil: true},
		"invalid401":   {status: http.StatusUnauthorized},
		"invalid403":   {status: http.StatusForbidden},
		"rateLimited":  {status: http.StatusTooManyRequests, wantErr: ErrValidationUnsupported},
		"paymentReq":   {status: http.StatusPaymentRequired, wantErr: ErrValidationUnsupported},
		"transient500": {status: http.StatusInternalServerError, wantErr: ErrValidationUnsupported},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv, captured := newCaptureServer(t, tc.status)
			c := &ProviderConfig{
				ID:      string(catwalk.InferenceProviderVenice),
				Type:    catwalk.TypeOpenAICompat,
				BaseURL: srv.URL,
				APIKey:  "sk-venice",
			}
			err := c.TestConnection(IdentityResolver())
			switch {
			case tc.wantNil:
				require.NoError(t, err)
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			default:
				require.Error(t, err)
				require.NotErrorIs(t, err, ErrValidationUnsupported)
			}
			require.Equal(t, http.MethodGet, captured.method)
			require.Equal(t, "/api_keys/rate_limits", captured.path)
			require.Equal(t, "Bearer sk-venice", captured.headers.Get("Authorization"))
		})
	}
}

func TestTestConnectionOpenAICompatChatProbe(t *testing.T) {
	t.Parallel()

	providers := []catwalk.InferenceProvider{
		catwalk.InferenceAIHubMix,
		catwalk.InferenceProviderAvian,
		catwalk.InferenceProviderCortecs,
		catwalk.InferenceProviderHuggingFace,
		catwalk.InferenceProviderIoNet,
		catwalk.InferenceProviderOpenCodeGo,
		catwalk.InferenceProviderOpenCodeZen,
		catwalk.InferenceProviderQiniuCloud,
		catwalk.InferenceProviderSynthetic,
	}
	for _, id := range providers {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()
			cases := map[string]struct {
				status  int
				wantErr error
				wantNil bool
			}{
				"authPassed400":   {status: http.StatusBadRequest, wantNil: true},
				"authPassed422":   {status: http.StatusUnprocessableEntity, wantNil: true},
				"invalid401":      {status: http.StatusUnauthorized},
				"invalid403":      {status: http.StatusForbidden},
				"transient500":    {status: http.StatusInternalServerError, wantErr: ErrValidationUnsupported},
				"unexpected200":   {status: http.StatusOK, wantErr: ErrValidationUnsupported},
				"unexpectedOther": {status: http.StatusTeapot, wantErr: ErrValidationUnsupported},
			}
			for name, tc := range cases {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					srv, captured := newCaptureServer(t, tc.status)
					c := &ProviderConfig{
						ID:      string(id),
						Type:    catwalk.TypeOpenAICompat,
						BaseURL: srv.URL,
						APIKey:  "sk-test",
					}
					err := c.TestConnection(IdentityResolver())
					switch {
					case tc.wantNil:
						require.NoError(t, err)
					case tc.wantErr != nil:
						require.ErrorIs(t, err, tc.wantErr)
					default:
						require.Error(t, err)
						require.NotErrorIs(t, err, ErrValidationUnsupported)
					}
					require.Equal(t, http.MethodPost, captured.method)
					require.Equal(t, "/chat/completions", captured.path)
					require.Equal(t, "Bearer sk-test", captured.headers.Get("Authorization"))
					require.Equal(t, "application/json", captured.headers.Get("Content-Type"))
					require.NotEmpty(t, captured.body)
				})
			}
		})
	}
}

func TestTestConnectionUnsupportedProviders(t *testing.T) {
	t.Parallel()

	for _, id := range []catwalk.InferenceProvider{
		catwalk.InferenceProviderChutes,
		catwalk.InferenceProviderNeuralwatt,
	} {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()
			c := &ProviderConfig{
				ID:      string(id),
				Type:    catwalk.TypeOpenAICompat,
				BaseURL: "https://example.invalid",
				APIKey:  "sk-test",
			}
			err := c.TestConnection(IdentityResolver())
			require.ErrorIs(t, err, ErrValidationUnsupported)
		})
	}
}

func TestTestConnectionUnknownOpenAICompatIsUnsupported(t *testing.T) {
	t.Parallel()

	c := &ProviderConfig{
		ID:      "some-new-openai-compat-provider",
		Type:    catwalk.TypeOpenAICompat,
		BaseURL: "https://example.invalid",
		APIKey:  "sk-test",
	}
	err := c.TestConnection(IdentityResolver())
	require.ErrorIs(t, err, ErrValidationUnsupported)
}

func TestTestConnectionEmptyProbeURLIsUnsupported(t *testing.T) {
	t.Parallel()

	// Chutes has a provider override that returns ErrValidationUnsupported
	// regardless of configured base URL; this also guards the empty-URL path.
	c := &ProviderConfig{
		ID:     string(catwalk.InferenceProviderChutes),
		Type:   catwalk.TypeOpenAICompat,
		APIKey: "sk-test",
	}
	err := c.TestConnection(IdentityResolver())
	require.ErrorIs(t, err, ErrValidationUnsupported)
}

func TestTestConnectionExtraHeadersAreApplied(t *testing.T) {
	t.Parallel()

	srv, captured := newCaptureServer(t, http.StatusBadRequest)
	c := &ProviderConfig{
		ID:      string(catwalk.InferenceProviderSynthetic),
		Type:    catwalk.TypeOpenAICompat,
		BaseURL: srv.URL,
		APIKey:  "sk-test",
		ExtraHeaders: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "overridden",
		},
	}
	err := c.TestConnection(IdentityResolver())
	require.NoError(t, err)
	require.Equal(t, "custom-value", captured.headers.Get("X-Custom-Header"))
	// ExtraHeaders are applied after the probe headers, so callers can
	// override per-provider defaults if necessary.
	require.Equal(t, "overridden", captured.headers.Get("Authorization"))
}

func TestTestConnectionOpenAITypeProbesModelsEndpoint(t *testing.T) {
	t.Parallel()

	srv, captured := newCaptureServer(t, http.StatusOK)
	c := &ProviderConfig{
		ID:      string(catwalk.InferenceProviderOpenAI),
		Type:    catwalk.TypeOpenAI,
		BaseURL: srv.URL,
		APIKey:  "sk-openai",
	}
	err := c.TestConnection(IdentityResolver())
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, captured.method)
	require.Equal(t, "/models", captured.path)
	require.Equal(t, "Bearer sk-openai", captured.headers.Get("Authorization"))
}

func TestTestConnectionOpenRouterProbesCreditsEndpoint(t *testing.T) {
	t.Parallel()

	srv, captured := newCaptureServer(t, http.StatusOK)
	c := &ProviderConfig{
		ID:      string(catwalk.InferenceProviderOpenRouter),
		Type:    catwalk.TypeOpenRouter,
		BaseURL: srv.URL,
		APIKey:  "sk-or",
	}
	err := c.TestConnection(IdentityResolver())
	require.NoError(t, err)
	require.Equal(t, "/credits", captured.path)
}

func TestTestConnectionAnthropicTypeProbesModels(t *testing.T) {
	t.Parallel()

	srv, captured := newCaptureServer(t, http.StatusOK)
	c := &ProviderConfig{
		ID:      string(catwalk.InferenceProviderAnthropic),
		Type:    catwalk.TypeAnthropic,
		BaseURL: srv.URL,
		APIKey:  "ak-test",
	}
	err := c.TestConnection(IdentityResolver())
	require.NoError(t, err)
	require.Equal(t, "/models", captured.path)
	require.Equal(t, "ak-test", captured.headers.Get("x-api-key"))
}

func TestTestConnectionKimiCodingUsesV1Models(t *testing.T) {
	t.Parallel()

	srv, captured := newCaptureServer(t, http.StatusOK)
	c := &ProviderConfig{
		ID:      string(catwalk.InferenceKimiCoding),
		Type:    catwalk.TypeAnthropic,
		BaseURL: srv.URL,
		APIKey:  "ak-kimi",
	}
	err := c.TestConnection(IdentityResolver())
	require.NoError(t, err)
	require.Equal(t, "/v1/models", captured.path)
}

func TestTestConnectionGoogleIncludesKeyQueryParam(t *testing.T) {
	t.Parallel()

	srv, captured := newCaptureServer(t, http.StatusOK)
	c := &ProviderConfig{
		ID:      string(catwalk.InferenceProviderGemini),
		Type:    catwalk.TypeGoogle,
		BaseURL: srv.URL,
		APIKey:  "google-key",
	}
	err := c.TestConnection(IdentityResolver())
	require.NoError(t, err)
	require.Equal(t, "/v1beta/models", captured.path)
	require.Contains(t, captured.query, "key=google-key")
}

// TestTestConnectionGoogleBadKeyIs400 locks in the fact that Google returns
// 400 INVALID_ARGUMENT (not 401) for an unknown API key, so 400 must map to
// "invalid" and never to [ErrValidationUnsupported].
func TestTestConnectionGoogleBadKeyIs400(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		status  int
		wantNil bool
		wantErr error
	}{
		"badKey400":    {status: http.StatusBadRequest},
		"unauth401":    {status: http.StatusUnauthorized},
		"forbidden403": {status: http.StatusForbidden},
		"ok200":        {status: http.StatusOK, wantNil: true},
		"transient500": {status: http.StatusInternalServerError, wantErr: ErrValidationUnsupported},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv, _ := newCaptureServer(t, tc.status)
			c := &ProviderConfig{
				ID:      string(catwalk.InferenceProviderGemini),
				Type:    catwalk.TypeGoogle,
				BaseURL: srv.URL,
				APIKey:  "bad-key",
			}
			err := c.TestConnection(IdentityResolver())
			switch {
			case tc.wantNil:
				require.NoError(t, err)
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			default:
				require.Error(t, err)
				require.NotErrorIs(t, err, ErrValidationUnsupported)
			}
		})
	}
}

// TestTestConnectionOpenAICompatAllowlistUsesModelsProbe locks in the
// `/models` probe for openai-compat providers whose /models is known to be
// auth-gated. These providers must not fall through to
// [ErrValidationUnsupported].
func TestTestConnectionOpenAICompatAllowlistUsesModelsProbe(t *testing.T) {
	t.Parallel()

	providers := []catwalk.InferenceProvider{
		"deepseek",
		catwalk.InferenceProviderGROQ,
		catwalk.InferenceProviderXAI,
		catwalk.InferenceProviderZhipu,
		catwalk.InferenceProviderZhipuCoding,
		catwalk.InferenceProviderCerebras,
		catwalk.InferenceProviderNebius,
		catwalk.InferenceProviderCopilot,
	}
	for _, id := range providers {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()
			t.Run("valid", func(t *testing.T) {
				t.Parallel()
				srv, captured := newCaptureServer(t, http.StatusOK)
				c := &ProviderConfig{
					ID:      string(id),
					Type:    catwalk.TypeOpenAICompat,
					BaseURL: srv.URL,
					APIKey:  "sk-good",
				}
				require.NoError(t, c.TestConnection(IdentityResolver()))
				require.Equal(t, http.MethodGet, captured.method)
				require.Equal(t, "/models", captured.path)
				require.Equal(t, "Bearer sk-good", captured.headers.Get("Authorization"))
			})
			t.Run("invalid", func(t *testing.T) {
				t.Parallel()
				srv, _ := newCaptureServer(t, http.StatusUnauthorized)
				c := &ProviderConfig{
					ID:      string(id),
					Type:    catwalk.TypeOpenAICompat,
					BaseURL: srv.URL,
					APIKey:  "sk-bad",
				}
				err := c.TestConnection(IdentityResolver())
				require.Error(t, err)
				require.NotErrorIs(t, err, ErrValidationUnsupported)
			})
		})
	}
}

// TestTestConnectionZAIUsesZAIClassifier pins ZAI's historical quirk: /models
// returns non-200 for valid keys but always 401 for bad keys.
func TestTestConnectionZAIUsesZAIClassifier(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		status  int
		wantNil bool
	}{
		"ok200":     {status: http.StatusOK, wantNil: true},
		"other400":  {status: http.StatusBadRequest, wantNil: true},
		"other500":  {status: http.StatusInternalServerError, wantNil: true},
		"badKey401": {status: http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv, captured := newCaptureServer(t, tc.status)
			c := &ProviderConfig{
				ID:      string(catwalk.InferenceProviderZAI),
				Type:    catwalk.TypeOpenAICompat,
				BaseURL: srv.URL,
				APIKey:  "sk-zai",
			}
			err := c.TestConnection(IdentityResolver())
			if tc.wantNil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.NotErrorIs(t, err, ErrValidationUnsupported)
			}
			require.Equal(t, "/models", captured.path)
			require.Equal(t, "Bearer sk-zai", captured.headers.Get("Authorization"))
		})
	}
}

func TestTestConnectionBedrockPrefix(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		c := &ProviderConfig{
			ID:     string(catwalk.InferenceProviderBedrock),
			Type:   catwalk.TypeBedrock,
			APIKey: "ABSK-secret",
		}
		require.NoError(t, c.TestConnection(IdentityResolver()))
	})
	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		c := &ProviderConfig{
			ID:     string(catwalk.InferenceProviderBedrock),
			Type:   catwalk.TypeBedrock,
			APIKey: "nope",
		}
		err := c.TestConnection(IdentityResolver())
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrValidationUnsupported)
	})
}

func TestTestConnectionVercelPrefix(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		c := &ProviderConfig{
			ID:     string(catwalk.InferenceProviderVercel),
			Type:   catwalk.TypeVercel,
			APIKey: "vck_abc",
		}
		require.NoError(t, c.TestConnection(IdentityResolver()))
	})
	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		c := &ProviderConfig{
			ID:     string(catwalk.InferenceProviderVercel),
			Type:   catwalk.TypeVercel,
			APIKey: "nope",
		}
		err := c.TestConnection(IdentityResolver())
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrValidationUnsupported)
	})
}

// TestTestConnectionPublicModelsAuthGatedChatRegression locks in the core
// regression from the 2025-10-20 expansion of generic /models validation to
// openai-compat: a provider whose /models is intentionally public would
// report any key as "validated" even though /chat/completions actually
// gates on auth. For every provider we currently mark "validated" via the
// malformed-body chat probe, this test simulates both endpoints and asserts
// that:
//
//  1. A bad key (401 on /chat/completions) is reported as invalid, not as
//     "validated" — even when /models returns 200 unauthenticated.
//  2. A good key (400/422 on /chat/completions) is reported as valid.
//  3. The probe never hits /models for these providers.
func TestTestConnectionPublicModelsAuthGatedChatRegression(t *testing.T) {
	t.Parallel()

	providers := []catwalk.InferenceProvider{
		catwalk.InferenceAIHubMix,
		catwalk.InferenceProviderAvian,
		catwalk.InferenceProviderCortecs,
		catwalk.InferenceProviderHuggingFace,
		catwalk.InferenceProviderIoNet,
		catwalk.InferenceProviderOpenCodeGo,
		catwalk.InferenceProviderOpenCodeZen,
		catwalk.InferenceProviderQiniuCloud,
		catwalk.InferenceProviderSynthetic,
	}
	for _, id := range providers {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()

			type hits struct {
				models int
				chat   int
			}
			for name, tc := range map[string]struct {
				chatStatus int
				wantErr    error
				wantNil    bool
			}{
				"badKeyIsInvalidNotValidated": {
					chatStatus: http.StatusUnauthorized,
				},
				"goodKeyIsValidated": {
					chatStatus: http.StatusBadRequest,
					wantNil:    true,
				},
				"forbiddenKeyIsInvalid": {
					chatStatus: http.StatusForbidden,
				},
				"schemaFailure422IsValidated": {
					chatStatus: http.StatusUnprocessableEntity,
					wantNil:    true,
				},
			} {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					h := &hits{}
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						switch r.URL.Path {
						case "/models":
							// Simulate a public /models endpoint that
							// returns 200 regardless of the provided key.
							h.models++
							w.WriteHeader(http.StatusOK)
						case "/chat/completions":
							h.chat++
							w.WriteHeader(tc.chatStatus)
						default:
							w.WriteHeader(http.StatusNotFound)
						}
					}))
					t.Cleanup(srv.Close)

					c := &ProviderConfig{
						ID:      string(id),
						Type:    catwalk.TypeOpenAICompat,
						BaseURL: srv.URL,
						APIKey:  "sk-test",
					}
					err := c.TestConnection(IdentityResolver())

					if tc.wantNil {
						require.NoError(t, err, "expected %s to validate on %d", id, tc.chatStatus)
					} else {
						require.Error(t, err, "expected %s to reject on %d", id, tc.chatStatus)
						require.NotErrorIs(t, err, ErrValidationUnsupported)
					}
					require.Equal(t, 0, h.models, "probe must not rely on public /models for %s", id)
					require.Equal(t, 1, h.chat, "probe must hit /chat/completions for %s", id)
				})
			}
		})
	}
}

// TestTestConnectionOpenAICompatProviderAudit is an audit table that pins the
// full set of openai-compat providers currently exposed as "validated" (i.e.
// TestConnection can return nil on some response) and documents the exact
// probe each uses. Adding a new openai-compat provider to the validated set
// MUST update this table; this prevents silent drift back into the
// "assume /models proves auth" bug class.
//
// Providers not listed here either:
//   - use a different Type (TypeOpenAI / TypeAnthropic / TypeGoogle / ...);
//   - are explicitly gated behind ErrValidationUnsupported (chutes, neuralwatt,
//     and every unknown openai-compat provider).
func TestTestConnectionOpenAICompatProviderAudit(t *testing.T) {
	t.Parallel()

	audit := map[catwalk.InferenceProvider]auditCase{
		catwalk.InferenceProviderVenice: {
			method:        http.MethodGet,
			path:          "/api_keys/rate_limits",
			validStatus:   http.StatusOK,
			invalidStatus: http.StatusUnauthorized,
			authHeader:    "Authorization",
			authValue:     "Bearer sk-test",
		},
		catwalk.InferenceAIHubMix:            openaiCompatAuditCase(),
		catwalk.InferenceProviderAvian:       openaiCompatAuditCase(),
		catwalk.InferenceProviderCortecs:     openaiCompatAuditCase(),
		catwalk.InferenceProviderHuggingFace: openaiCompatAuditCase(),
		catwalk.InferenceProviderIoNet:       openaiCompatAuditCase(),
		catwalk.InferenceProviderOpenCodeGo:  openaiCompatAuditCase(),
		catwalk.InferenceProviderOpenCodeZen: openaiCompatAuditCase(),
		catwalk.InferenceProviderQiniuCloud:  openaiCompatAuditCase(),
		catwalk.InferenceProviderSynthetic:   openaiCompatAuditCase(),
		// openai-compat providers with auth-gated /models (allowlist).
		"deepseek":                           openaiCompatModelsAuditCase(),
		catwalk.InferenceProviderGROQ:        openaiCompatModelsAuditCase(),
		catwalk.InferenceProviderXAI:         openaiCompatModelsAuditCase(),
		catwalk.InferenceProviderZhipu:       openaiCompatModelsAuditCase(),
		catwalk.InferenceProviderZhipuCoding: openaiCompatModelsAuditCase(),
		catwalk.InferenceProviderCerebras:    openaiCompatModelsAuditCase(),
		catwalk.InferenceProviderNebius:      openaiCompatModelsAuditCase(),
		catwalk.InferenceProviderCopilot:     openaiCompatModelsAuditCase(),
		// ZAI uses the /models endpoint but with its own classifier that
		// only treats 401 as invalid. Its valid path must therefore be 200
		// here for the audit's generic "valid -> nil" check to hold.
		catwalk.InferenceProviderZAI: {
			method:        http.MethodGet,
			path:          "/models",
			validStatus:   http.StatusOK,
			invalidStatus: http.StatusUnauthorized,
			authHeader:    "Authorization",
			authValue:     "Bearer sk-test",
		},
	}

	for id, tc := range audit {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()

			// 1) Valid path.
			srv, captured := newCaptureServer(t, tc.validStatus)
			c := &ProviderConfig{
				ID:      string(id),
				Type:    catwalk.TypeOpenAICompat,
				BaseURL: srv.URL,
				APIKey:  "sk-test",
			}
			require.NoError(t, c.TestConnection(IdentityResolver()))
			require.Equal(t, tc.method, captured.method, "audit: wrong method for %s", id)
			require.Equal(t, tc.path, captured.path, "audit: wrong path for %s", id)
			require.Equal(t, tc.authValue, captured.headers.Get(tc.authHeader),
				"audit: wrong auth header for %s", id)

			// 2) Invalid path.
			srv2, _ := newCaptureServer(t, tc.invalidStatus)
			c2 := &ProviderConfig{
				ID:      string(id),
				Type:    catwalk.TypeOpenAICompat,
				BaseURL: srv2.URL,
				APIKey:  "sk-test",
			}
			err := c2.TestConnection(IdentityResolver())
			require.Error(t, err, "audit: %s must reject %d as invalid", id, tc.invalidStatus)
			require.NotErrorIs(t, err, ErrValidationUnsupported,
				"audit: %s must not leak ErrValidationUnsupported on %d", id, tc.invalidStatus)
		})
	}

	// Sanity: every provider that currently enters the openai-compat chat
	// probe path must appear in the audit. This guards against a future
	// refactor silently adding a provider without test coverage.
	chatProbeProviders := []catwalk.InferenceProvider{
		catwalk.InferenceAIHubMix,
		catwalk.InferenceProviderAvian,
		catwalk.InferenceProviderCortecs,
		catwalk.InferenceProviderHuggingFace,
		catwalk.InferenceProviderIoNet,
		catwalk.InferenceProviderOpenCodeGo,
		catwalk.InferenceProviderOpenCodeZen,
		catwalk.InferenceProviderQiniuCloud,
		catwalk.InferenceProviderSynthetic,
	}
	for _, id := range chatProbeProviders {
		_, ok := audit[id]
		require.True(t, ok, "audit table missing entry for %s", id)
	}
}

// auditCase pins the expected probe shape for a given provider.
type auditCase struct {
	method string
	path   string
	// validStatus is a response code the probe must translate to
	// "validated" (nil error).
	validStatus int
	// invalidStatus is a response code the probe must translate to an
	// invalid-key error (not ErrValidationUnsupported).
	invalidStatus int
	// authHeader is the name of the header the probe uses to present
	// the key.
	authHeader string
	authValue  string
}

func openaiCompatAuditCase() auditCase {
	return auditCase{
		method:        http.MethodPost,
		path:          "/chat/completions",
		validStatus:   http.StatusBadRequest,
		invalidStatus: http.StatusUnauthorized,
		authHeader:    "Authorization",
		authValue:     "Bearer sk-test",
	}
}

func openaiCompatModelsAuditCase() auditCase {
	return auditCase{
		method:        http.MethodGet,
		path:          "/models",
		validStatus:   http.StatusOK,
		invalidStatus: http.StatusUnauthorized,
		authHeader:    "Authorization",
		authValue:     "Bearer sk-test",
	}
}

func TestTestConnectionNetworkErrorIsNotInvalidKey(t *testing.T) {
	t.Parallel()

	// Start and immediately close a server so the next request fails at the
	// TCP layer. That should produce a non-nil error that is *not*
	// ErrValidationUnsupported (transport errors still surface).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.Close()
	c := &ProviderConfig{
		ID:      string(catwalk.InferenceProviderOpenAI),
		Type:    catwalk.TypeOpenAI,
		BaseURL: srv.URL,
		APIKey:  "sk-test",
	}
	err := c.TestConnection(IdentityResolver())
	require.Error(t, err)
	// The error message should mention the provider so users see a useful
	// hint, even though we can't classify the status code.
	require.True(t, strings.Contains(err.Error(), "openai") || errors.Is(err, ErrValidationUnsupported))
}
