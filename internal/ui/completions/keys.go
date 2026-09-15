package completions

import (
	"charm.land/bubbles/v2/key"
)

// KeyMap defines the key bindings for the completions component.
type KeyMap struct {
	Down,
	Up,
	Select,
	Cancel key.Binding
	DownInsert,
	UpInsert key.Binding
}

// DefaultKeyMap returns the default key bindings for completions.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("down", "move down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("up", "move up"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "tab", "ctrl+y"),
			key.WithHelp("enter", "select"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "cancel"),
		),
		DownInsert: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("ctrl+n", "insert next"),
		),
		UpInsert: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "insert previous"),
		),
	}
}

// KeyBindings returns all key bindings as a slice.
func (k KeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.Down,
		k.Up,
		k.Select,
		k.Cancel,
	}
}

// Apply overlays user overrides onto the key map. Unknown actions are
// ignored; the defaults stand. An empty key list disables the binding.
func (k *KeyMap) Apply(overrides map[string][]string) {
	apply := func(action string, b *key.Binding) {
		keys, ok := overrides[action]
		if !ok {
			return
		}
		if len(keys) == 0 {
			b.SetEnabled(false)
			return
		}
		b.SetKeys(keys...)
		b.SetHelp(keys[0], b.Help().Desc)
	}
	apply("completions.down", &k.Down)
	apply("completions.up", &k.Up)
	apply("completions.select", &k.Select)
	apply("completions.cancel", &k.Cancel)
	apply("completions.down_insert", &k.DownInsert)
	apply("completions.up_insert", &k.UpInsert)
}

// FullHelp returns the full help for the key bindings.
func (k KeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp returns the short help for the key bindings.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
	}
}
