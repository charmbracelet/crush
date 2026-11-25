package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExperimentalExportEnvKeepsEnvPriority(t *testing.T) {
	t.Setenv("CRUSH_CLIENT_SERVER", "false")
	t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "")
	t.Setenv("CRUSH_SERVER_IDLE_TIMEOUT", "")
	t.Setenv("CRUSH_SERVER_DETACH_GRACE", "")

	on, ready, idle := true, 30, 0
	(&ExperimentalOptions{
		ClientServer:       &on,
		ServerReadyTimeout: &ready,
		ServerIdleTimeout:  &idle,
	}).ExportEnv()

	require.Equal(t, "false", os.Getenv("CRUSH_CLIENT_SERVER"))
	require.Equal(t, "30s", os.Getenv("CRUSH_SERVER_READY_TIMEOUT"))
	require.Equal(t, "0", os.Getenv("CRUSH_SERVER_IDLE_TIMEOUT"))
	require.Empty(t, os.Getenv("CRUSH_SERVER_DETACH_GRACE"))
}

func TestLoadExperimental(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "crush.json"),
		[]byte(`{"options":{"experimental":{"client_server":true,"server_detach_grace":5}}}`), 0o644))

	exp, err := LoadExperimental(t.Context(), dir)
	require.NoError(t, err)
	require.NotNil(t, exp)
	require.True(t, *exp.ClientServer)
	require.Equal(t, 5, *exp.ServerDetachGrace)
}
