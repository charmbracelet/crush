package shellconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLSPRemove(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `lsp add gopls --command gopls
lsp add pyright --command pyright-langserver
lsp remove gopls`)

	lsps := result["lsp"].(map[string]any)
	require.NotContains(t, lsps, "gopls")
	require.Contains(t, lsps, "pyright")
}

func TestLSPRemoveAlias(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `lsp add gopls --command gopls
lsp rm gopls`)

	require.NotContains(t, result["lsp"].(map[string]any), "gopls")
}

func TestLSPUnknownSubcommand(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/crushrc"
	_, err := LoadShellConfig(path, []byte(`lsp gopls --command gopls`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown subcommand")
}
