package model

import "charm.land/bubbles/v2/textarea"

// configureTextareaKeyMap keeps the textarea defaults for whole-word movement
// on Alt+Arrow while Ctrl+Arrow is handled by the UI for CamelHumps semantics.
func configureTextareaKeyMap(ta *textarea.Model) {
	ta.KeyMap.WordForward.SetKeys("alt+right", "alt+f")
	ta.KeyMap.WordBackward.SetKeys("alt+left", "alt+b")
}
