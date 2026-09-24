package shell

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
)

// namedKeys maps key names to key codes, matching the names users and
// models know from Bubble Tea ("enter", "ctrl+c", "shift+tab").
var namedKeys = map[string]uv.Key{
	"enter":     {Code: uv.KeyEnter},
	"return":    {Code: uv.KeyEnter},
	"tab":       {Code: uv.KeyTab},
	"esc":       {Code: uv.KeyEscape},
	"escape":    {Code: uv.KeyEscape},
	"backspace": {Code: uv.KeyBackspace},
	"delete":    {Code: uv.KeyDelete},
	"insert":    {Code: uv.KeyInsert},
	"up":        {Code: uv.KeyUp},
	"down":      {Code: uv.KeyDown},
	"left":      {Code: uv.KeyLeft},
	"right":     {Code: uv.KeyRight},
	"home":      {Code: uv.KeyHome},
	"end":       {Code: uv.KeyEnd},
	"pageup":    {Code: uv.KeyPgUp},
	"pgup":      {Code: uv.KeyPgUp},
	"pagedown":  {Code: uv.KeyPgDown},
	"pgdown":    {Code: uv.KeyPgDown},
	"space":     {Code: ' ', Text: " "},

	"f1":  {Code: uv.KeyF1},
	"f2":  {Code: uv.KeyF2},
	"f3":  {Code: uv.KeyF3},
	"f4":  {Code: uv.KeyF4},
	"f5":  {Code: uv.KeyF5},
	"f6":  {Code: uv.KeyF6},
	"f7":  {Code: uv.KeyF7},
	"f8":  {Code: uv.KeyF8},
	"f9":  {Code: uv.KeyF9},
	"f10": {Code: uv.KeyF10},
	"f11": {Code: uv.KeyF11},
	"f12": {Code: uv.KeyF12},
}

// ParseKey parses a key name like "enter", "down", "ctrl+c", "alt+enter",
// "shift+tab", "f5", or a single printable character into a key event that
// the terminal emulator can encode.
//
// The event is written through the emulator's input path, so the encoding
// matches whatever mode the child enabled (application cursor keys, and so
// on). Sending raw escape sequences instead is fragile because TUIs switch
// those modes.
func ParseKey(name string) (uv.KeyPressEvent, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return uv.KeyPressEvent{}, errors.New("empty key")
	}

	var mod uv.KeyMod
	parts := strings.Split(name, "+")
	if len(parts) > 1 {
		for _, part := range parts[:len(parts)-1] {
			switch strings.ToLower(part) {
			case "ctrl", "control":
				mod |= uv.ModCtrl
			case "alt", "option":
				mod |= uv.ModAlt
			case "shift":
				mod |= uv.ModShift
			case "meta", "super", "win", "windows", "cmd", "command":
				mod |= uv.ModMeta
			case "hyper":
				mod |= uv.ModHyper
			default:
				return uv.KeyPressEvent{}, fmt.Errorf("unknown modifier %q in key %q", part, name)
			}
		}
	}

	key := parts[len(parts)-1]
	lower := strings.ToLower(key)

	// A named key (with modifiers, or alone) takes the key-code path.
	if named, ok := namedKeys[lower]; ok {
		k := named
		k.Mod |= mod
		if mod&uv.ModCtrl != 0 {
			k.Text = "" // ctrl+key encodes a control character
		}
		return uv.KeyPressEvent(k), nil
	}

	// A bare modifier is not a key.
	if _, ok := namedKeys[lower]; !ok && len(parts) == 1 {
		switch lower {
		case "ctrl", "control", "alt", "option", "shift", "meta", "super", "win", "windows", "cmd", "command", "hyper":
			return uv.KeyPressEvent{}, fmt.Errorf("%q is a modifier, not a key", name)
		}
	}

	// A single printable character carries its own text so shifted and
	// composed characters send the symbol itself.
	if utf8.RuneCountInString(key) == 1 {
		r, _ := utf8.DecodeRuneInString(key)
		k := uv.Key{Code: r, Text: key, Mod: mod}
		if mod&uv.ModCtrl != 0 {
			k.Text = "" // ctrl+letter encodes a control character
		}
		return uv.KeyPressEvent(k), nil
	}

	return uv.KeyPressEvent{}, fmt.Errorf("unknown key %q", name)
}
