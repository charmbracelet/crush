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

func isolatePromptOverrideEnv(t *testing.T) {
	t.Helper()
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(hostHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(hostHome, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(hostHome, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
}

// A --prompt flag sent by a client must land in the workspace's runtime
// overrides and round-trip back out, so a reconnecting client sees it.
func TestBackend_CreateWorkspacePromptOverrides(t *testing.T) {
	isolatePromptOverrideEnv(t)

	wd := t.TempDir()
	coder := filepath.Join(wd, "coder.md.tpl")
	require.NoError(t, os.WriteFile(coder, []byte("custom coder"), 0o644))

	srvCfg, err := config.Init(t.Context(), wd, "", false)
	require.NoError(t, err)
	b := backend.New(t.Context(), srvCfg, nil)

	cid := uuid.New().String()
	ws, created, err := b.CreateWorkspace(proto.Workspace{
		ClientID: cid,
		Path:     wd,
		DataDir:  filepath.Join(wd, ".crush"),
		Prompts:  config.PromptPaths{"coder": coder},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.DeleteWorkspace(ws.ID, cid) })

	require.Equal(t, coder, ws.Cfg.Overrides().Prompts["coder"])
	require.Equal(t, coder, created.Prompts["coder"])
	require.Empty(t, ws.Cfg.Config().Options.Prompts, "a flag must not leak into persisted config")
}

// A second --prompt run in an already-open directory cannot change its
// prompts, so it must fail loudly instead of silently running the old ones.
// A client that asks for no prompts may still join.
func TestBackend_CreateWorkspacePromptMismatch(t *testing.T) {
	isolatePromptOverrideEnv(t)

	wd := t.TempDir()
	srvCfg, err := config.Init(t.Context(), wd, "", false)
	require.NoError(t, err)
	b := backend.New(t.Context(), srvCfg, nil)

	create := func(prompts config.PromptPaths) (*backend.Workspace, error) {
		cid := uuid.New().String()
		ws, _, err := b.CreateWorkspace(proto.Workspace{
			ClientID: cid,
			Path:     wd,
			DataDir:  filepath.Join(wd, ".crush"),
			Prompts:  prompts,
		})
		if err == nil {
			t.Cleanup(func() { _ = b.DeleteWorkspace(ws.ID, cid) })
		}
		return ws, err
	}

	first := filepath.Join(wd, "first.md.tpl")
	second := filepath.Join(wd, "second.md.tpl")
	require.NoError(t, os.WriteFile(first, []byte("first"), 0o644))
	require.NoError(t, os.WriteFile(second, []byte("second"), 0o644))

	ws, err := create(config.PromptPaths{"coder": first})
	require.NoError(t, err)

	_, err = create(config.PromptPaths{"coder": second})
	require.ErrorIs(t, err, backend.ErrPromptOverrideMismatch)

	same, err := create(config.PromptPaths{"coder": first})
	require.NoError(t, err)
	require.Equal(t, ws.ID, same.ID)

	plain, err := create(nil)
	require.NoError(t, err)
	require.Equal(t, ws.ID, plain.ID)
}

func TestBackend_CreateWorkspaceRejectsUnknownPrompt(t *testing.T) {
	isolatePromptOverrideEnv(t)

	wd := t.TempDir()
	srvCfg, err := config.Init(t.Context(), wd, "", false)
	require.NoError(t, err)
	b := backend.New(t.Context(), srvCfg, nil)

	_, _, err = b.CreateWorkspace(proto.Workspace{
		ClientID: uuid.New().String(),
		Path:     wd,
		DataDir:  filepath.Join(wd, ".crush"),
		Prompts:  config.PromptPaths{"codr": "/tmp/x.md"},
	})
	require.ErrorContains(t, err, `unknown prompt "codr"`)
}
