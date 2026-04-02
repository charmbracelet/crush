package openai_codex

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAuthorizationInput(t *testing.T) {
	t.Parallel()

	code, state := ParseAuthorizationInput("http://localhost:1455/auth/callback?code=abc&state=xyz")
	require.Equal(t, "abc", code)
	require.Equal(t, "xyz", state)

	code, state = ParseAuthorizationInput("abc#xyz")
	require.Equal(t, "abc", code)
	require.Equal(t, "xyz", state)

	code, state = ParseAuthorizationInput("code=123&state=456")
	require.Equal(t, "123", code)
	require.Equal(t, "456", state)
}

func TestExtractAccountID(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		jwtClaimKey: map[string]any{
			"chatgpt_account_id": "acc_123",
		},
	}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	token := "h." + base64.RawURLEncoding.EncodeToString(payloadJSON) + ".s"
	accountID, err := ExtractAccountID(token)
	require.NoError(t, err)
	require.Equal(t, "acc_123", accountID)
}
