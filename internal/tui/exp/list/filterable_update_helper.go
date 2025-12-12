package list

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// IsMovementKey checks if the given keypress is a movement key
func IsMovementKey(msg tea.KeyPressMsg, keyMap KeyMap) bool {
	return key.Matches(msg, keyMap.Down) ||
		key.Matches(msg, keyMap.Up) ||
		key.Matches(msg, keyMap.DownOneItem) ||
		key.Matches(msg, keyMap.UpOneItem) ||
		key.Matches(msg, keyMap.HalfPageDown) ||
		key.Matches(msg, keyMap.HalfPageUp) ||
		key.Matches(msg, keyMap.PageDown) ||
		key.Matches(msg, keyMap.PageUp) ||
		key.Matches(msg, keyMap.End) ||
		key.Matches(msg, keyMap.Home)
}