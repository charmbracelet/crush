package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	powernapconfig "github.com/charmbracelet/x/powernap/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestHandlesRespectsSingleFileSupport(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	pyFile := filepath.Join(tmp, "main.py")
	require.NoError(t, os.WriteFile(pyFile, []byte("x = 1\n"), 0o644))

	server := &powernapconfig.ServerConfig{
		Command:           "pyright-langserver",
		FileTypes:         []string{"python"},
		RootMarkers:       []string{".git"},
		SingleFileSupport: true,
	}
	require.True(t, handles(server, pyFile, tmp), "single-file-capable server should handle file when no root markers present")

	server.SingleFileSupport = false
	require.False(t, handles(server, pyFile, tmp), "non-single-file server should be skipped when no root markers present")
}

func TestNewManagerPreservesDefaultSingleFileSupport(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{LSP: config.LSPs{"pyright": {Command: "pyright-langserver"}}}
	mgr := NewManager(config.NewTestStore(cfg))
	s, ok := mgr.manager.GetServer("pyright")
	require.True(t, ok, "pyright should be registered")
	require.True(t, s.SingleFileSupport, "SingleFileSupport must survive user override of a known default")
}
