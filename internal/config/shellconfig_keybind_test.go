package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestShellConfigKeybindSet(t *testing.T) {
	store := loadCrushSh(t, `keybind set global.quit ctrl+q
keybind set editor.send_message ctrl+enter`)

	require.Equal(t, []string{"ctrl+q"}, store.Config().Keybinds["global.quit"])
	require.Equal(t, []string{"ctrl+enter"}, store.Config().Keybinds["editor.send_message"])
}

// Later sets win, matching every other builtin.
func TestShellConfigKeybindSetOverwrites(t *testing.T) {
	store := loadCrushSh(t, `keybind set global.quit ctrl+q
keybind set global.quit ctrl+x`)

	require.Equal(t, []string{"ctrl+x"}, store.Config().Keybinds["global.quit"])
}

// Multiple keys accumulate on one action.
func TestShellConfigKeybindSetMultiKey(t *testing.T) {
	store := loadCrushSh(t, `keybind set global.models ctrl+m ctrl+l`)

	require.Equal(t, []string{"ctrl+m", "ctrl+l"}, store.Config().Keybinds["global.models"])
}

func TestShellConfigKeybindUnset(t *testing.T) {
	store := loadCrushSh(t, `keybind set global.quit ctrl+q
keybind unset global.quit`)

	require.NotContains(t, store.Config().Keybinds, "global.quit")
}

func TestShellConfigKeybindReset(t *testing.T) {
	store := loadCrushSh(t, `keybind set global.quit ctrl+q
keybind set chat.new_session ctrl+n
keybind reset`)

	require.Empty(t, store.Config().Keybinds)
}

// A whitespace-only key is rejected as empty.
func TestShellConfigKeybindRejectsWhitespaceKey(t *testing.T) {
	_, err := loadCrushShErr(t, `keybind set chat.expand " "`)
	require.Error(t, err)
}

// Project keybinds override the global config per action; actions the
// project does not mention keep the global keys. Unset cannot reach
// across files: it only drops overrides made earlier in the same script.
func TestShellConfigKeybindsProjectBeatGlobal(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))
	t.Setenv("CRUSH_GLOBAL_CONFIG", filepath.Join(isolated, ".config", "crush"))
	t.Setenv("CRUSH_GLOBAL_DATA", filepath.Join(isolated, ".local", "share", "crush"))

	globalDir := filepath.Join(isolated, ".config", "crush")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "crushrc"),
		[]byte("keybind set global.quit ctrl+g\nkeybind set global.help ctrl+h\n"), 0o644))

	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "crushrc"),
		[]byte("keybind set global.quit ctrl+q\nkeybind unset global.help\n"), 0o644))

	store, err := config.Load(workDir, t.TempDir(), false)
	require.NoError(t, err)

	require.Equal(t, []string{"ctrl+q"}, store.Config().Keybinds["global.quit"], "project keybind lost to global")
	require.Equal(t, []string{"ctrl+h"}, store.Config().Keybinds["global.help"], "global keybind lost to an unrelated unset")
}

func TestShellConfigKeybindRejectsBadShape(t *testing.T) {
	_, err := loadCrushShErr(t, `keybind set quit ctrl+q`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected scope.name")
}

func TestShellConfigKeybindRejectsUnknownSubcommand(t *testing.T) {
	_, err := loadCrushShErr(t, `keybind frobnicate global.quit ctrl+q`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown subcommand")
}

func TestShellConfigKeybindSetRequiresKeys(t *testing.T) {
	_, err := loadCrushShErr(t, `keybind set global.quit`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage: keybind set")
}

// Unknown and excluded actions warn and drop at load instead of failing
// it; the valid override still lands.
func TestShellConfigKeybindUnknownDropsNotFails(t *testing.T) {
	store := loadCrushSh(t, `keybind set chat.tab x
keybind set bogus.action y
keybind set global.quit ctrl+q`)

	require.Equal(t, []string{"ctrl+q"}, store.Config().Keybinds["global.quit"])
	require.NotContains(t, store.Config().Keybinds, "chat.tab")
	require.NotContains(t, store.Config().Keybinds, "bogus.action")
}

func TestShellConfigKeybindDisable(t *testing.T) {
	store := loadCrushSh(t, `keybind set global.quit ctrl+q
keybind disable global.quit`)

	require.Equal(t, []string{}, store.Config().Keybinds["global.quit"])
}

func TestShellConfigKeybindDisableRequiresAction(t *testing.T) {
	_, err := loadCrushShErr(t, `keybind disable`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage: keybind disable")
}
