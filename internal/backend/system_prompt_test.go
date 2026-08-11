package backend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBackend_CreateWorkspaceSystemPromptOverride(t *testing.T) {
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(hostHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(hostHome, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(hostHome, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")

	workingDir := t.TempDir()
	writeBackendConfig(t, workingDir)
	systemPromptPath := filepath.Join(workingDir, "system.md")

	srvCfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)
	b := backend.New(t.Context(), srvCfg, nil)

	cid := uuid.New().String()
	ws, created, err := b.CreateWorkspace(proto.Workspace{
		ClientID:         cid,
		Path:             workingDir,
		DataDir:          filepath.Join(workingDir, ".crush"),
		SystemPromptPath: systemPromptPath,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.DeleteWorkspace(ws.ID, cid) })

	require.Equal(t, systemPromptPath, ws.Cfg.Overrides().SystemPromptPath)
	require.Equal(t, systemPromptPath, created.SystemPromptPath)
	require.Empty(t, ws.Cfg.Config().Options.SystemPromptPath)

	fetched, err := b.GetWorkspaceProto(ws.ID)
	require.NoError(t, err)
	require.Equal(t, systemPromptPath, fetched.SystemPromptPath)
}

func writeBackendConfig(t *testing.T, workingDir string) {
	t.Helper()
	content := `{"options":{"disable_metrics":true,"context_paths":[],"disabled_skills":["crush-config","crush-hooks","jq"]}}`
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "crush.json"), []byte(content), 0o644))
}
