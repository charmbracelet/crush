package cmd

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/stretchr/testify/require"
)

func TestLogFilePathLocal(t *testing.T) {
	t.Parallel()

	got := logFilePath("/proj/.crush", false, "unix:///ignored.sock")
	require.Equal(t, filepath.Join("/proj/.crush", "logs", "crush.log"), got)
}

func TestLogFilePathClientServer(t *testing.T) {
	t.Parallel()

	a := logFilePath("/proj/.crush", true, "unix:///run/a.sock")
	b := logFilePath("/proj/.crush", true, "unix:///run/b.sock")
	require.NotEqual(t, a, b)
	require.Equal(t, "crush.log", filepath.Base(a))
	require.Equal(t, config.GlobalCacheDir(), filepath.Dir(filepath.Dir(a)))

	// Must match the path the server itself writes to.
	u, err := server.ParseHostURL("unix:///run/a.sock")
	require.NoError(t, err)
	require.Equal(t, serverLogPath(u), a)
	require.Contains(t, a, "server-"+safeHostName(u))
}
