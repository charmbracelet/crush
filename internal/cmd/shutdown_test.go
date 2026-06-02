package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/taigrr/crush/internal/client"
	"github.com/taigrr/crush/internal/config"
	"github.com/taigrr/crush/internal/server"
)

func TestEnsureServer_UsesSingleFlightSpawnPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping server spawn test in short mode")
	}

	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	host := filepath.Join(t.TempDir(), "crush.sock")
	hostURL, err := server.ParseHostURL("unix://" + host)
	require.NoError(t, err)

	prevHost := clientHost
	clientHost = "unix://" + host
	defer func() { clientHost = prevHost }()

	cmd := &cobra.Command{}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd.SetContext(ctx)
	cmd.Flags().Bool("debug", false, "")
	cmd.Flags().Bool("yolo", false, "")
	cmd.Flags().String("data-dir", t.TempDir(), "")
	cmd.Flags().String("cwd", t.TempDir(), "")

	err = ensureServer(cmd, hostURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		cli, cliErr := newControlClient(hostURL)
		if cliErr == nil {
			_ = cli.ShutdownServer(context.Background())
		}
	})

	require.FileExists(t, host)
	lockPath := filepath.Join(config.GlobalCacheDir(), "server-"+safeHostName(hostURL), "start.lock")
	_, err = os.Stat(lockPath)
	require.NoError(t, err)
}

func TestNewControlClient(t *testing.T) {
	hostURL, err := server.ParseHostURL(server.DefaultHost())
	require.NoError(t, err)

	cli, err := newControlClient(hostURL)
	require.NoError(t, err)
	require.NotNil(t, cli)
}

func TestShutdownCommandRequiresRunningServer(t *testing.T) {
	host := filepath.Join(t.TempDir(), "missing.sock")
	hostURL, err := server.ParseHostURL("unix://" + host)
	require.NoError(t, err)

	cli, err := client.NewClient("", hostURL.Scheme, hostURL.Host)
	require.NoError(t, err)

	err = cli.ShutdownServer(context.Background())
	require.Error(t, err)
}
