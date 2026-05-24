package proto_test

import (
	"encoding/json"
	"testing"

	"github.com/taigrr/crush/internal/config"
	"github.com/taigrr/crush/internal/oauth"
	"github.com/taigrr/crush/internal/proto"
	"github.com/stretchr/testify/require"
)

func TestConfigProviderKeyRequestStringRoundTrip(t *testing.T) {
	t.Parallel()

	apiKey, err := json.Marshal("sk-test-123")
	require.NoError(t, err)

	src := proto.ConfigProviderKeyRequest{
		Scope:      config.ScopeGlobal,
		ProviderID: "openai",
		Kind:       proto.APIKeyKindString,
		APIKey:     apiKey,
	}
	b, err := json.Marshal(src)
	require.NoError(t, err)

	var got proto.ConfigProviderKeyRequest
	require.NoError(t, json.Unmarshal(b, &got))
	require.Equal(t, proto.APIKeyKindString, got.Kind)

	decoded, err := got.DecodeAPIKey()
	require.NoError(t, err)
	s, ok := decoded.(string)
	require.True(t, ok, "expected string, got %T", decoded)
	require.Equal(t, "sk-test-123", s)
}

func TestConfigProviderKeyRequestOAuthRoundTrip(t *testing.T) {
	t.Parallel()

	tok := &oauth.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresIn:    60,
		ExpiresAt:    1234567890,
	}
	apiKey, err := json.Marshal(tok)
	require.NoError(t, err)

	src := proto.ConfigProviderKeyRequest{
		Scope:      config.ScopeGlobal,
		ProviderID: "hyper",
		Kind:       proto.APIKeyKindOAuth,
		APIKey:     apiKey,
	}
	b, err := json.Marshal(src)
	require.NoError(t, err)

	var got proto.ConfigProviderKeyRequest
	require.NoError(t, json.Unmarshal(b, &got))
	require.Equal(t, proto.APIKeyKindOAuth, got.Kind)

	decoded, err := got.DecodeAPIKey()
	require.NoError(t, err)
	gotTok, ok := decoded.(*oauth.Token)
	require.True(t, ok, "expected *oauth.Token, got %T", decoded)
	require.Equal(t, tok, gotTok)
}

func TestConfigProviderKeyRequestUnknownKind(t *testing.T) {
	t.Parallel()

	req := proto.ConfigProviderKeyRequest{
		Kind:   proto.APIKeyKind("bogus"),
		APIKey: json.RawMessage(`"x"`),
	}
	_, err := req.DecodeAPIKey()
	require.Error(t, err)
	require.Contains(t, err.Error(), "bogus")
}

func TestConfigProviderKeyRequestMalformedPayload(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		kind proto.APIKeyKind
		raw  string
	}{
		{"string kind with object payload", proto.APIKeyKindString, `{"foo":"bar"}`},
		{"oauth kind with string payload", proto.APIKeyKindOAuth, `"not-a-token"`},
		{"oauth kind with invalid json", proto.APIKeyKindOAuth, `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := proto.ConfigProviderKeyRequest{
				Kind:   tc.kind,
				APIKey: json.RawMessage(tc.raw),
			}
			_, err := req.DecodeAPIKey()
			require.Error(t, err)
		})
	}
}
