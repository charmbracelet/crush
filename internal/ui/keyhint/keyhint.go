// Package keyhint renders keyboard shortcut hints with platform-appropriate
// modifier labels. Only the displayed label changes; key matching always uses
// the raw binding.
//
// macOS writes the control modifier as the glyph ⌃ (U+2303), so "ctrl+p" is
// shown as "⌃p". Command (⌘) is deliberately not used: macOS and terminal
// emulators consume Command shortcuts before they reach the application, so
// the displayed modifier has to match the key users actually press. Other
// modifiers keep their spelled-out form, so "ctrl+shift+v" renders as
// "⌃shift+v".
package keyhint

import (
	"runtime"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
)

const (
	// ctrlName is how key bindings spell the control modifier.
	ctrlName = "ctrl"
	// ctrlGlyph is the macOS control modifier glyph (U+2303). It is plain
	// Unicode rather than a Nerd Font symbol, so it needs no patched font.
	ctrlGlyph = "⌃"
)

// isDarwin reports whether hints should use macOS key labels. It is a
// variable so tests can exercise both platforms.
var isDarwin = runtime.GOOS == "darwin"

// labelReplacer rewrites Ctrl chords to the macOS control glyph. The
// "ctrl+" entry is what keeps the plus out of the result: without it,
// "ctrl+p" would become "⌃+p". The bare "ctrl" entry covers standalone
// mentions such as "press ctrl".
var labelReplacer = strings.NewReplacer(ctrlName+"+", ctrlGlyph, ctrlName, ctrlGlyph)

// Label rewrites modifier names in a key hint for the current platform.
// Other platforms are returned unchanged.
func Label(label string) string {
	if !isDarwin {
		return label
	}
	return labelReplacer.Replace(label)
}

// relabel returns a copy of b with its help key label rewritten for the
// current platform. The actual keys and enabled state are preserved.
func relabel(b key.Binding) key.Binding {
	h := b.Help()
	if h.Key == "" {
		return b
	}
	b.SetHelp(Label(h.Key), h.Desc)
	return b
}

// relabelAll returns copies of the bindings with platform-appropriate help
// labels. A nil slice stays nil so help views can skip empty groups, and
// platforms without label rewrites return the original slice.
func relabelAll(bindings []key.Binding) []key.Binding {
	if !isDarwin || bindings == nil {
		return bindings
	}
	out := make([]key.Binding, len(bindings))
	for i, b := range bindings {
		out[i] = relabel(b)
	}
	return out
}

// Wrap returns km with platform-appropriate key labels, to be rendered by
// help.Model. Off macOS it returns km unchanged.
//
// Every help view must be rendered through Wrap (or Label for hand-built
// hint lines); key matching must never go through either.
func Wrap(km help.KeyMap) help.KeyMap {
	if !isDarwin {
		return km
	}
	return wrapped{km}
}

// wrapped rewrites the key labels of an embedded help.KeyMap.
type wrapped struct {
	help.KeyMap
}

// ShortHelp implements help.KeyMap.
func (w wrapped) ShortHelp() []key.Binding {
	return relabelAll(w.KeyMap.ShortHelp())
}

// FullHelp implements help.KeyMap.
func (w wrapped) FullHelp() [][]key.Binding {
	groups := w.KeyMap.FullHelp()
	if groups == nil {
		return nil
	}
	out := make([][]key.Binding, len(groups))
	for i, group := range groups {
		out[i] = relabelAll(group)
	}
	return out
}
