package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestLoad_DisableAWSCredentials proves the whole pipeline: a machine with
// resolvable AWS credentials gets no Bedrock providers at all once the user
// turns off automatic import, while the default behavior is unchanged.
func TestLoad_DisableAWSCredentials(t *testing.T) {
	writeAWS := func(t *testing.T) {
		t.Helper()
		home := os.Getenv("HOME")
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".aws"), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(home, ".aws", "credentials"),
			[]byte("[default]\naws_access_key_id = AKIAFAKE\naws_secret_access_key = fake\n"),
			0o600,
		))
		t.Setenv("AWS_ACCESS_KEY_ID", "AKIAFAKE")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "fake")
	}

	t.Run("aws creds enable bedrock by default", func(t *testing.T) {
		workDir, dataDir := isolateReloadEnv(t)
		writeAWS(t)

		store, err := config.Load(t.Context(), workDir, dataDir, false)
		require.NoError(t, err)
		require.False(t, store.Config().Options.DisableAWSCredentials)
		_, ok := store.Config().Providers.Get("bedrock")
		require.True(t, ok, "resolvable AWS credentials should auto-enable Bedrock")
	})

	t.Run("aws-credentials false hides bedrock", func(t *testing.T) {
		workDir, dataDir := isolateReloadEnv(t)
		writeAWS(t)

		require.NoError(t, os.WriteFile(
			filepath.Join(workDir, "crushrc"),
			[]byte("option aws-credentials false\n"),
			0o644,
		))

		store, err := config.Load(t.Context(), workDir, dataDir, false)
		require.NoError(t, err)
		require.True(t, store.Config().Options.DisableAWSCredentials)
		_, ok := store.Config().Providers.Get("bedrock")
		require.False(t, ok, "automatic import off: no Bedrock provider may appear")
		_, ok = store.Config().Providers.Get("bedrock-europe")
		require.False(t, ok)
	})

	t.Run("json config matches the crushrc key", func(t *testing.T) {
		workDir, dataDir := isolateReloadEnv(t)
		writeAWS(t)

		require.NoError(t, os.WriteFile(
			filepath.Join(workDir, "crush.json"),
			[]byte(`{"options":{"disable_aws_credentials":true}}`),
			0o644,
		))

		store, err := config.Load(t.Context(), workDir, dataDir, false)
		require.NoError(t, err)
		_, ok := store.Config().Providers.Get("bedrock")
		require.False(t, ok)
	})
}
