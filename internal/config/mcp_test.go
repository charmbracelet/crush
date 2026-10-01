package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetMCPServerDisabledConfigRequiresConfiguredServer(t *testing.T) {
	t.Parallel()

	store := NewTestStore(&Config{MCP: map[string]MCPConfig{"docker": {}}})
	err := store.SetMCPServerDisabledConfig(ScopeGlobal, "nope", true)
	require.ErrorContains(t, err, "not configured", "unknown servers must not create empty config entries")
}
