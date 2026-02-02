package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupOpenAICodex(t *testing.T) {
	t.Parallel()
	pc := &ProviderConfig{}
	pc.SetupOpenAICodex("test-account-id")

	require.NotNil(t, pc.ExtraHeaders)
	require.Equal(t, "test-account-id", pc.ExtraHeaders["chatgpt-account-id"])
	require.Equal(t, "responses=experimental", pc.ExtraHeaders["OpenAI-Beta"])
	require.Equal(t, "codex_cli_rs", pc.ExtraHeaders["originator"])

	require.NotNil(t, pc.ExtraBody)
	require.Equal(t, false, pc.ExtraBody["store"])
	require.Equal(t, "You are a helpful coding assistant.", pc.ExtraBody["instructions"])
	require.Contains(t, pc.ExtraBody["include"], "reasoning.encrypted_content")
}
