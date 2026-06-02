package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/taigrr/crush/internal/client"
	"github.com/taigrr/crush/internal/proto"
	"github.com/taigrr/crush/internal/server"
)

func TestServerPersistsAfterLastWorkspaceDeleted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping daemon persistence integration test in short mode")
	}

	host := filepath.Join(t.TempDir(), "daemon.sock")
	hostURL, err := server.ParseHostURL("unix://" + host)
	require.NoError(t, err)

	prevHost := clientHost
	clientHost = "unix://" + host
	defer func() { clientHost = prevHost }()

	ctx := context.Background()
	cmd := rootCmd
	cmd.SetContext(ctx)
	cmd.Flags().Set("data-dir", t.TempDir())
	cmd.Flags().Set("cwd", t.TempDir())

	err = ensureServer(cmd, hostURL)
	require.NoError(t, err)

	cli, err := client.NewClient(t.TempDir(), hostURL.Scheme, hostURL.Host)
	require.NoError(t, err)

	ws, err := cli.CreateWorkspace(ctx, proto.Workspace{Path: t.TempDir()})
	require.NoError(t, err)

	err = cli.DeleteWorkspace(ctx, ws.ID)
	require.NoError(t, err)

	err = cli.Health(ctx)
	require.NoError(t, err, "server should still be healthy after last workspace is deleted")

	_ = cli.ShutdownServer(ctx)
}
