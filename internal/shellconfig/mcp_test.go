package shellconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMCPRemove(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `mcp add github --command npx
mcp add local --type http --url "http://localhost:3000/mcp"
mcp remove github`)

	mcps := result["mcp"].(map[string]any)
	require.NotContains(t, mcps, "github")
	require.Contains(t, mcps, "local")
}

func TestMCPRemoveAlias(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `mcp add github --command npx
mcp rm github`)

	require.NotContains(t, result["mcp"].(map[string]any), "github")
}

func TestMCPOAuthFlags(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `mcp add secure --type http --url "https://mcp.example.com" \
  --oauth true \
  --oauth-client-id "my-client" \
  --oauth-client-secret "my-secret" \
  --oauth-callback-port 8085`)

	m := result["mcp"].(map[string]any)["secure"].(map[string]any)
	require.Equal(t, true, m["oauth"])
	require.Equal(t, "my-client", m["oauth_client_id"])
	require.Equal(t, "my-secret", m["oauth_client_secret"])
	require.Equal(t, float64(8085), m["oauth_callback_port"])
}

func TestMCPUnknownSubcommand(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/crushrc"
	_, err := LoadShellConfig(t.Context(), path, []byte(`mcp github --command npx`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown subcommand")
}
