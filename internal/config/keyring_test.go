package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func init() {
	// Use the in-memory mock backend for all tests.
	keyring.MockInit()
}

func TestKeyringStore_Available(t *testing.T) {
	ks := &KeyringStore{available: true}
	assert.True(t, ks.Available())

	ks = &KeyringStore{available: false}
	assert.False(t, ks.Available())

	// nil receiver
	var nilKS *KeyringStore
	assert.False(t, nilKS.Available())
}

func TestKeyringStore_APIKey(t *testing.T) {
	ks := &KeyringStore{available: true}

	// Set and Get
	require.NoError(t, ks.SetAPIKey("openai", "sk-test-12345"))
	key, err := ks.GetAPIKey("openai")
	require.NoError(t, err)
	assert.Equal(t, "sk-test-12345", key)

	// Overwrite
	require.NoError(t, ks.SetAPIKey("openai", "sk-new-key"))
	key, err = ks.GetAPIKey("openai")
	require.NoError(t, err)
	assert.Equal(t, "sk-new-key", key)

	// Delete
	require.NoError(t, ks.DeleteAPIKey("openai"))
	_, err = ks.GetAPIKey("openai")
	assert.Error(t, err)
}

func TestKeyringStore_OAuthToken(t *testing.T) {
	ks := &KeyringStore{available: true}

	token := &oauth.Token{
		AccessToken:  "ghu_abc123",
		RefreshToken: "ghr_xyz789",
		ExpiresIn:    3600,
		ExpiresAt:    1700000000,
	}

	require.NoError(t, ks.SetOAuthToken("copilot", token))

	got, err := ks.GetOAuthToken("copilot")
	require.NoError(t, err)
	assert.Equal(t, token.AccessToken, got.AccessToken)
	assert.Equal(t, token.RefreshToken, got.RefreshToken)
	assert.Equal(t, token.ExpiresIn, got.ExpiresIn)
	assert.Equal(t, token.ExpiresAt, got.ExpiresAt)

	// Delete
	require.NoError(t, ks.DeleteOAuthToken("copilot"))
	_, err = ks.GetOAuthToken("copilot")
	assert.Error(t, err)
}

func TestKeyringStore_Unavailable(t *testing.T) {
	ks := &KeyringStore{available: false}

	assert.Error(t, ks.SetAPIKey("openai", "sk-123"))
	_, err := ks.GetAPIKey("openai")
	assert.Error(t, err)
	assert.Error(t, ks.DeleteAPIKey("openai"))

	assert.Error(t, ks.SetOAuthToken("copilot", &oauth.Token{}))
	_, err = ks.GetOAuthToken("copilot")
	assert.Error(t, err)
	assert.Error(t, ks.DeleteOAuthToken("copilot"))
}

func TestKeyringStore_MultipleProviders(t *testing.T) {
	ks := &KeyringStore{available: true}

	require.NoError(t, ks.SetAPIKey("openai", "sk-openai"))
	require.NoError(t, ks.SetAPIKey("anthropic", "sk-anthropic"))

	k1, err := ks.GetAPIKey("openai")
	require.NoError(t, err)
	assert.Equal(t, "sk-openai", k1)

	k2, err := ks.GetAPIKey("anthropic")
	require.NoError(t, err)
	assert.Equal(t, "sk-anthropic", k2)
}

func TestIsKeyringMarker(t *testing.T) {
	assert.True(t, IsKeyringMarker(KeyringMarker))
	assert.True(t, IsKeyringMarker("__keyring__"))
	assert.False(t, IsKeyringMarker("sk-12345"))
	assert.False(t, IsKeyringMarker(""))
	assert.False(t, IsKeyringMarker("$OPENAI_API_KEY"))
}

func TestIsLiteralSecret(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"sk-12345", true},
		{"ghu_abc123", true},
		{"", false},
		{KeyringMarker, false},
		{"$OPENAI_API_KEY", false},
		{"${OPENAI_API_KEY}", false},
		{"$(some-command)", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, tt.expected, isLiteralSecret(tt.value))
		})
	}
}
