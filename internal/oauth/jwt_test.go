package oauth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// idToken builds an unsigned JWT whose payload carries the given claims, the
// way an OpenID Connect provider would hand one back.
func idToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestEmailFromIDToken(t *testing.T) {
	t.Parallel()

	require.Equal(t, "dev@example.com", EmailFromIDToken(idToken(t, map[string]any{
		"email": "dev@example.com",
		"sub":   "1234",
	})))

	// Only display is derived from this, so a token without the claim is
	// simply unlabeled rather than an error.
	require.Empty(t, EmailFromIDToken(idToken(t, map[string]any{"sub": "1234"})))
	require.Empty(t, EmailFromIDToken(""))
	require.Empty(t, EmailFromIDToken("not-a-jwt"))
	require.Empty(t, EmailFromIDToken("a.!!!not base64!!!.b"))
}
