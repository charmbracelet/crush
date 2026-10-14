package model

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"charm.land/bubbles/v2/key"
	"github.com/stretchr/testify/require"
)

// countBindings counts key.Binding fields independently of keyRegistry.
func countBindings(t reflect.Type) int {
	n := 0
	for f := range t.Fields() {
		switch {
		case f.Type == reflect.TypeOf(key.Binding{}):
			n++
		case f.Type.Kind() == reflect.Struct:
			n += countBindings(f.Type)
		}
	}
	return n
}

func TestKeyRegistryCoversAllBindings(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	reg := keyRegistry(&km)
	require.Equal(t, countBindings(reflect.TypeOf(km)), len(reg))
	require.Same(t, &km.Editor.SendMessage, reg["editor.send_message"])
	require.Same(t, &km.Chat.NewSession, reg["chat.new_session"])
	require.Same(t, &km.Quit, reg["quit"])
}

func TestApplyKeyOverridesSetsKeys(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	warns := applyKeyOverrides(&km, map[string][]string{"quit": {"ctrl+q", "ctrl+x"}})
	require.Empty(t, warns)
	require.Equal(t, []string{"ctrl+q", "ctrl+x"}, km.Quit.Keys())
	require.Equal(t, "ctrl+q", km.Quit.Help().Key)
	require.Equal(t, "quit", km.Quit.Help().Desc)
	require.True(t, km.Quit.Enabled())
}

func TestApplyKeyOverridesEmptyDisables(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	warns := applyKeyOverrides(&km, map[string][]string{"suspend": {}})
	require.Empty(t, warns)
	require.False(t, km.Suspend.Enabled())
}

func TestApplyKeyOverridesUnknownWarns(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	warns := applyKeyOverrides(&km, map[string][]string{"chat.nope": {"x"}})
	require.Len(t, warns, 1)
	require.Contains(t, warns[0], "chat.nope")
}

func TestApplyKeyOverridesConflictWarns(t *testing.T) {
	t.Parallel()

	// Same group (chat.*): ctrl+b already toggles the sidebar.
	km := DefaultKeyMap()
	warns := applyKeyOverrides(&km, map[string][]string{"chat.new_session": {"ctrl+b"}})
	require.Len(t, warns, 1)
	require.Contains(t, warns[0], "chat.new_session")
	require.Contains(t, warns[0], "chat.toggle_sidebar")

	// Different groups never conflict.
	km = DefaultKeyMap()
	require.Empty(t, applyKeyOverrides(&km, map[string][]string{"editor.open_editor": {"ctrl+b"}}))

	// A disabled action does not conflict.
	km = DefaultKeyMap()
	require.Empty(t, applyKeyOverrides(&km, map[string][]string{
		"chat.toggle_sidebar": {},
		"chat.new_session":    {"ctrl+b"},
	}))
}

func TestLoadKeyMapMissingFile(t *testing.T) {
	t.Parallel()

	km, warns := LoadKeyMap(filepath.Join(t.TempDir(), "keybindings.json"))
	require.Empty(t, warns)
	require.Equal(t, DefaultKeyMap().Quit.Keys(), km.Quit.Keys())
}

func TestLoadKeyMapBadJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "keybindings.json")
	require.NoError(t, os.WriteFile(path, []byte("{nope"), 0o644))
	km, warns := LoadKeyMap(path)
	require.Len(t, warns, 1)
	require.Equal(t, DefaultKeyMap().Quit.Keys(), km.Quit.Keys())
}

func TestLoadKeyMapAppliesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "keybindings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"quit": ["ctrl+q"]}`), 0o644))
	km, warns := LoadKeyMap(path)
	require.Empty(t, warns)
	require.Equal(t, []string{"ctrl+q"}, km.Quit.Keys())
}

func TestApplyKeyOverridesIgnoresDisplayOnly(t *testing.T) {
	t.Parallel()

	// "down" is also on the display-only chat.up_down: no conflict.
	km := DefaultKeyMap()
	require.Empty(t, applyKeyOverrides(&km, map[string][]string{"chat.down": {"down", "s"}}))

	// Binding a display-only action is flagged as useless.
	km = DefaultKeyMap()
	warns := applyKeyOverrides(&km, map[string][]string{"chat.up_down": {"x"}})
	require.Len(t, warns, 1)
	require.Contains(t, warns[0], "display-only")
}

func TestApplyKeyOverridesGlobalConflictsWithAll(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	warns := applyKeyOverrides(&km, map[string][]string{"quit": {"ctrl+n"}})
	require.Len(t, warns, 1)
	require.Contains(t, warns[0], "chat.new_session")

	// Keeping a default key that another group shares is not a new conflict.
	km = DefaultKeyMap()
	require.Empty(t, applyKeyOverrides(&km, map[string][]string{"tab": {"tab", "ctrl+x"}}))
}

func TestEnhancedKeyHelpKeepsUserKeys(t *testing.T) {
	t.Parallel()

	km := DefaultKeyMap()
	applyKeyOverrides(&km, map[string][]string{"models": {"ctrl+k"}})
	u := &UI{keyMap: km}
	u.updateEnhancedKeyHelp()
	require.Equal(t, "ctrl+k", u.keyMap.Models.Help().Key)
	// Not overridden: help follows the enhanced terminal.
	require.Equal(t, "shift+enter", u.keyMap.Editor.Newline.Help().Key)
}

// A kitty-protocol terminal keeps the footer's alt+m: ctrl+m only arrives
// there, so advertising it would drop the key every terminal sends.
func TestEnhancedKeyHelpKeepsAltM(t *testing.T) {
	t.Parallel()

	u := &UI{keyMap: DefaultKeyMap()}
	u.updateEnhancedKeyHelp()
	require.Equal(t, key.Help{Key: "alt+m", Desc: "model"}, u.keyMap.Models.Help())
}

func TestKeyRegistrySkipsUnexported(t *testing.T) {
	t.Parallel()

	type withHidden struct {
		A      key.Binding
		hidden key.Binding
	}
	reg := map[string]*key.Binding{}
	v := withHidden{}
	_ = v.hidden
	walkBindings(reflect.ValueOf(&v).Elem(), "", reg)
	require.Len(t, reg, 1)
	require.Contains(t, reg, "a")
}
