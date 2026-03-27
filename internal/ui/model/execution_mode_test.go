package model

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

func TestCurrentExecutionModeDefaultsToAuto(t *testing.T) {
	ui := testExecutionModeUI(t, `{"options":{"disable_provider_auto_update":true},"tools":{}}`)
	require.Equal(t, executionModeAuto, ui.currentExecutionMode())
	require.Equal(t, "auto", ui.com.Config().Options.PreferredPermissionMode)
}

func TestCycleExecutionModeCyclesAskAutoYolo(t *testing.T) {
	ui := testExecutionModeUI(t, `{"options":{"disable_provider_auto_update":true},"tools":{}}`)

	require.NoError(t, ui.com.Store().SetPreferredPermissionMode(config.ScopeGlobal, "default"))
	require.Equal(t, executionModeAsk, ui.currentExecutionMode())

	cmd := ui.cycleExecutionMode()
	require.NotNil(t, cmd)
	_, ok := cmd().(executionModeChangedMsg)
	require.True(t, ok)
	require.Equal(t, executionModeAuto, ui.currentExecutionMode())
	require.False(t, ui.com.App.Permissions.SkipRequests())
	require.Equal(t, "auto", ui.com.Config().Options.PreferredPermissionMode)

	cmd = ui.cycleExecutionMode()
	require.NotNil(t, cmd)
	_, ok = cmd().(executionModeChangedMsg)
	require.True(t, ok)
	require.Equal(t, executionModeYolo, ui.currentExecutionMode())
	require.False(t, ui.com.App.Permissions.SkipRequests())
	require.Equal(t, "yolo", ui.com.Config().Options.PreferredPermissionMode)

	cmd = ui.cycleExecutionMode()
	require.NotNil(t, cmd)
	_, ok = cmd().(executionModeChangedMsg)
	require.True(t, ok)
	require.Equal(t, executionModeAsk, ui.currentExecutionMode())
	require.False(t, ui.com.App.Permissions.SkipRequests())
	require.Equal(t, "default", ui.com.Config().Options.PreferredPermissionMode)
}

func testExecutionModeUI(t *testing.T, configContent string) *UI {
	t.Helper()

	baseDir := t.TempDir()
	dataHome := filepath.Join(baseDir, "data-home")
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("LOCALAPPDATA", dataHome)
	t.Setenv("APPDATA", dataHome)
	t.Setenv("USERPROFILE", baseDir)

	workingDir := filepath.Join(baseDir, "workspace")
	require.NoError(t, os.MkdirAll(workingDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "crush.json"), []byte(configContent), 0o644))

	store, err := config.Init(workingDir, filepath.Join(baseDir, "state"), false)
	require.NoError(t, err)

	dbDir := filepath.Join(baseDir, "db")
	require.NoError(t, os.MkdirAll(dbDir, 0o755))
	conn, err := db.Connect(context.Background(), dbDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	appCtx, cancel := context.WithCancel(context.Background())
	application, err := app.New(appCtx, conn, store)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		application.Shutdown()
		require.NoError(t, log.ResetForTesting())
	})

	return New(common.DefaultCommon(application), "", false)
}
