package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShellConfigLSPAdd(t *testing.T) {
	store := loadCrushSh(t, `lsp add gopls --command gopls --filetypes go --filetypes mod --root-markers go.mod --timeout 60`)

	l, ok := store.Config().LSP["gopls"]
	require.True(t, ok, "gopls LSP should be configured")
	require.Equal(t, "gopls", l.Command)
	require.Subset(t, l.FileTypes, []string{"go", "mod"})
	require.Contains(t, l.RootMarkers, "go.mod")
	require.Equal(t, 60, l.Timeout)
}

func TestShellConfigLSPRemove(t *testing.T) {
	store := loadCrushSh(t, `lsp add keepls --command keep-server
lsp add dropls --command drop-server
lsp remove dropls`)

	lsps := store.Config().LSP
	require.Contains(t, lsps, "keepls")
	require.NotContains(t, lsps, "dropls")
}
