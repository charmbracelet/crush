package config_test

import (
	"testing"

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

// Space normalizes to its word form at load.
func TestShellConfigKeybindNormalizesSpace(t *testing.T) {
	store := loadCrushSh(t, `keybind set chat.expand " "`)

	require.Equal(t, []string{"space"}, store.Config().Keybinds["chat.expand"])
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
