package editor

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNewEditorKeyMapWithCustom(t *testing.T) {
	t.Parallel()

	t.Run("default keymap", func(t *testing.T) {
		t.Parallel()
		keyMap := NewEditorKeyMapWithCustom(nil)

		// Should have both shift+enter and ctrl+j for newline
		require.Contains(t, keyMap.Newline.Keys(), "shift+enter")
		require.Contains(t, keyMap.Newline.Keys(), "ctrl+j")
	})

	t.Run("custom keymap without conflicts", func(t *testing.T) {
		t.Parallel()
		customKeymaps := config.KeyMaps{
			"editor_newline": "ctrl+n",
		}
		keyMap := NewEditorKeyMapWithCustom(customKeymaps)

		// Should use custom newline key
		require.Contains(t, keyMap.Newline.Keys(), "ctrl+n")
		require.NotContains(t, keyMap.Newline.Keys(), "ctrl+j")
	})

	t.Run("suspend key conflicts with newline", func(t *testing.T) {
		t.Parallel()
		customKeymaps := config.KeyMaps{
			config.CommandSuspend: "ctrl+j",
		}
		keyMap := NewEditorKeyMapWithCustom(customKeymaps)

		// Should only have shift+enter, not ctrl+j
		require.Contains(t, keyMap.Newline.Keys(), "shift+enter")
		require.NotContains(t, keyMap.Newline.Keys(), "ctrl+j")
	})

	t.Run("explicit newline key overrides conflict resolution", func(t *testing.T) {
		t.Parallel()
		customKeymaps := config.KeyMaps{
			config.CommandSuspend: "ctrl+j",
			"editor_newline":      "ctrl+j", // Explicitly set newline to ctrl+j despite conflict
		}
		keyMap := NewEditorKeyMapWithCustom(customKeymaps)

		// Should use explicit newline key even if it conflicts
		require.Contains(t, keyMap.Newline.Keys(), "ctrl+j")
		require.NotContains(t, keyMap.Newline.Keys(), "shift+enter")
	})
}
