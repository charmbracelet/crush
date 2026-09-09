package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestConfigInitIsolatedFromUserConfig pins the package-wide isolation set
// up by TestMain: config.Init must not pick up the developer's own
// crush.json. Without it, every tool-list assertion in this package counts
// whatever MCP servers happen to be configured on the machine running the
// tests, and the suite spawns those servers for real.
func TestConfigInitIsolatedFromUserConfig(t *testing.T) {
	t.Parallel()
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)
	require.Empty(t, cfg.Config().MCP,
		"the ambient user config leaked into the test config: TestMain must isolate CRUSH_GLOBAL_CONFIG and CRUSH_GLOBAL_DATA")
	require.Empty(t, cfg.Config().LSP,
		"the ambient user config leaked into the test config: TestMain must isolate CRUSH_GLOBAL_CONFIG and CRUSH_GLOBAL_DATA")
}
