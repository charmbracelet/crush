package remotecontrol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRejectsMissingAndInsecure(t *testing.T) {
	t.Parallel()

	err := (Config{}).Validate()
	require.Error(t, err)

	err = (Config{
		RelayURL: "ws://localhost:8080",
		Username: "admin",
		Password: "crushsecret",
	}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "insecure default")

	err = (Config{
		RelayURL: "ws://example.com:8080",
		Username: "admin",
		Password: "good-password",
	}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "wss://")

	err = (Config{
		RelayURL: "ws://localhost:8080",
		Username: "admin",
		Password: "good-password",
	}).Validate()
	require.NoError(t, err)

	err = (Config{
		RelayURL: "wss://relay.example.com",
		Username: "admin",
		Password: "good-password",
	}).Validate()
	require.NoError(t, err)
}

func TestResolveConfigEnvAndFile(t *testing.T) {
	t.Setenv("CRUSH_REMOTE_URL", "wss://env.example")
	t.Setenv("CRUSH_REMOTE_USER", "env-user")
	t.Setenv("CRUSH_REMOTE_PASS", "env-pass-not-default")

	cfg, err := ResolveConfig("wss://file.example", "file-user", "", "", "")
	require.NoError(t, err)
	require.Equal(t, "wss://env.example", cfg.RelayURL)
	require.Equal(t, "env-user", cfg.Username)
	require.Equal(t, "env-pass-not-default", cfg.Password)

	cfg, err = ResolveConfig("wss://file.example", "file-user", "wss://explicit.example", "explicit-user", "explicit-pass")
	require.NoError(t, err)
	require.Equal(t, "wss://explicit.example", cfg.RelayURL)
	require.Equal(t, "explicit-user", cfg.Username)
	require.Equal(t, "explicit-pass", cfg.Password)
}

func TestResolveConfigRequiresPassword(t *testing.T) {
	t.Setenv("CRUSH_REMOTE_URL", "")
	t.Setenv("CRUSH_REMOTE_USER", "")
	t.Setenv("CRUSH_REMOTE_PASS", "")

	_, err := ResolveConfig("ws://localhost:8080", "admin", "", "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "CRUSH_REMOTE_PASS")
}
