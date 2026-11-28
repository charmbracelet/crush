package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// A failed New must release the database reference it took over, or the
// pooled connection and the data directory lock stay held for the life
// of a server that keeps retrying the workspace.
func TestNewReleasesDatabaseWhenCoderAgentFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(home, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(home, ".local", "share", "crush"))

	workingDir, dataDir := t.TempDir(), t.TempDir()
	const crushJSON = `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai-compat",
    "base_url": "http://127.0.0.1:1", "api_key": "test",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 200000, "default_max_tokens": 4096}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "crush.json"), []byte(crushJSON), 0o644))

	store, err := config.Init(t.Context(), workingDir, dataDir, false)
	require.NoError(t, err)
	require.True(t, store.Config().IsConfigured())
	// Without a coder agent configuration InitCoderAgent fails.
	delete(store.Config().Agents, config.AgentCoder)

	conn, err := db.Connect(t.Context(), dataDir, db.WithDataDirLock(true))
	require.NoError(t, err)

	app, err := New(t.Context(), conn, store, skills.NewManager(nil, nil, nil))
	require.Error(t, err)
	require.Nil(t, app)

	// The only reference was released, so the pool closed the connection.
	require.Error(t, conn.PingContext(t.Context()))
}
