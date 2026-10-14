package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
)

// KeybindingsPath returns the user keybindings file, which lives next to the
// themes/ directory.
func KeybindingsPath() string {
	return filepath.Join(filepath.Dir(config.GlobalConfig()), "keybindings.json")
}

// KeyEntry describes one bindable action for `agent-cli keys`.
type KeyEntry struct {
	Action  string   `json:"action"`
	Keys    []string `json:"keys"`
	Help    string   `json:"help"`
	Enabled bool     `json:"enabled"`
	// DisplayOnly bindings only feed the help bar; they never trigger anything.
	DisplayOnly bool `json:"display_only,omitempty"`
}

// KeyEntries lists every action of km, sorted by action name.
func KeyEntries(km *KeyMap) []KeyEntry {
	reg := keyRegistry(km)
	entries := make([]KeyEntry, 0, len(reg))
	for name, b := range reg {
		entries = append(entries, KeyEntry{
			Action:      name,
			Keys:        b.Keys(),
			Help:        b.Help().Desc,
			Enabled:     b.Enabled(),
			DisplayOnly: displayOnly[name],
		})
	}
	slices.SortFunc(entries, func(a, b KeyEntry) int { return strings.Compare(a.Action, b.Action) })
	return entries
}

// LoadKeyMap returns the default keymap with the overrides from path applied.
// A missing file is not an error; a broken one yields the defaults and a
// warning.
func LoadKeyMap(path string) (KeyMap, []string) {
	km := DefaultKeyMap()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return km, nil
	}
	if err != nil {
		return km, []string{fmt.Sprintf("keybindings: %v", err)}
	}
	var overrides map[string][]string
	if err := json.Unmarshal(data, &overrides); err != nil {
		return DefaultKeyMap(), []string{fmt.Sprintf("keybindings: %s: %v", filepath.Base(path), err)}
	}
	warns := applyKeyOverrides(&km, overrides)
	return km, warns
}

// logKeyWarnings writes each warning to the log.
func logKeyWarnings(warns []string) {
	for _, w := range warns {
		slog.Warn("Keybindings", "warning", w)
	}
}

// applyKeyOverrides rebinds actions in km. An empty key list disables the
// action. It warns about unknown actions and about an overridden action that
// shares a key with another enabled action of the same top-level group.
func applyKeyOverrides(km *KeyMap, overrides map[string][]string) []string {
	reg := keyRegistry(km)
	var warns []string
	applied := make([]string, 0, len(overrides))
	for _, name := range slices.Sorted(maps.Keys(overrides)) {
		b, ok := reg[name]
		if !ok {
			warns = append(warns, fmt.Sprintf("keybindings: unknown action %q (see `agent-cli keys`)", name))
			continue
		}
		if displayOnly[name] {
			warns = append(warns, fmt.Sprintf("keybindings: %s is display-only (help bar); binding it has no effect", name))
		}
		keys := overrides[name]
		b.SetKeys(keys...)
		b.SetEnabled(len(keys) > 0)
		if len(keys) > 0 {
			b.SetHelp(keys[0], b.Help().Desc)
		}
		applied = append(applied, name)
	}

	defaults := DefaultKeyMap()
	defReg := keyRegistry(&defaults)
	names := slices.Sorted(maps.Keys(reg))
	seen := map[[2]string]bool{}
	for _, name := range applied {
		a := reg[name]
		if !a.Enabled() || displayOnly[name] {
			continue
		}
		// Keys the action already had by default are not the user's doing.
		var added []string
		for _, k := range a.Keys() {
			if !slices.Contains(defReg[name].Keys(), k) {
				added = append(added, k)
			}
		}
		for _, other := range names {
			b := reg[other]
			if other == name || !b.Enabled() || displayOnly[other] || !keyGroupsClash(name, other) {
				continue
			}
			pair := [2]string{min(name, other), max(name, other)}
			if seen[pair] {
				continue
			}
			for _, k := range added {
				if slices.Contains(b.Keys(), k) {
					seen[pair] = true
					warns = append(warns, fmt.Sprintf("keybindings: %q is bound to both %s and %s", k, name, other))
					break
				}
			}
		}
	}
	return warns
}

// displayOnly lists bindings that only feed the help bar (no key.Matches), so
// they take part in neither conflict checks nor dispatch.
var displayOnly = map[string]bool{
	"chat.up_down":          true,
	"chat.up_down_one_item": true,
}

// keyGroupsClash reports whether two actions can be live at once: same
// top-level group, or either one is global (root level).
func keyGroupsClash(a, b string) bool {
	ga, gb := keyGroup(a), keyGroup(b)
	return ga == gb || ga == "" || gb == ""
}

// keyGroup is the top-level struct of an action ("chat" for chat.down) or ""
// for the global actions.
func keyGroup(name string) string {
	group, _, ok := strings.Cut(name, ".")
	if !ok {
		return ""
	}
	return group
}

// keyRegistry maps snake_case action paths (editor.send_message) to every
// key.Binding in km, so new bindings are bindable without a hand-kept list.
func keyRegistry(km *KeyMap) map[string]*key.Binding {
	reg := map[string]*key.Binding{}
	walkBindings(reflect.ValueOf(km).Elem(), "", reg)
	return reg
}

func walkBindings(v reflect.Value, prefix string, reg map[string]*key.Binding) {
	for i := range v.NumField() {
		if !v.Type().Field(i).IsExported() {
			continue
		}
		name := prefix + snakeCase(v.Type().Field(i).Name)
		f := v.Field(i)
		switch f.Type() {
		case reflect.TypeFor[key.Binding]():
			reg[name] = f.Addr().Interface().(*key.Binding)
		default:
			if f.Kind() == reflect.Struct {
				walkBindings(f, name+".", reg)
			}
		}
	}
}

func snakeCase(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]))) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// updateEnhancedKeyHelp shows the keys a terminal with key disambiguation
// really sends, unless the user rebound the action.
func (m *UI) updateEnhancedKeyHelp() {
	def := DefaultKeyMap()
	if slices.Equal(m.keyMap.Editor.Newline.Keys(), def.Editor.Newline.Keys()) {
		m.keyMap.Editor.Newline.SetHelp("shift+enter", "newline")
	}
}
