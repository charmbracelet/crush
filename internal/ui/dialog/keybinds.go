package dialog

import (
	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/ui/common"
)

// Question dialog bindings, set once at startup like dialog.CloseKey.
// Inline editors read the vars at construction so no constructor
// signature changes.
var (
	QuestionSelect  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select"))
	QuestionToggle  = key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle"))
	QuestionDone    = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "done"))
	QuestionYes     = key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "yes"))
	QuestionNo      = key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n", "no"))
	QuestionConfirm = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm"))
	QuestionPrevTab = key.NewBinding(key.WithKeys("[", "ctrl+left"), key.WithHelp("[", "prev tab"))
	QuestionNextTab = key.NewBinding(key.WithKeys("]", "ctrl+right"), key.WithHelp("]", "next tab"))
	QuestionNewline = key.NewBinding(key.WithKeys("shift+enter", "ctrl+j"), key.WithHelp("shift+enter", "newline"))
	QuestionSubmit  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
)

// applyDialogKeybinds overlays user overrides onto one dialog's
// bindings, keyed by the action suffix after "dialog." (e.g. "select",
// "sessions.delete"). Empty lists disable; unknown IDs already warned
// at load.
func applyDialogKeybinds(com *common.Common, bindings map[string]*key.Binding) {
	// Tests construct Common without a workspace; there is no config
	// to read overrides from, so defaults stand.
	if com.Workspace == nil {
		return
	}
	cfg := com.Config()
	if cfg == nil || len(cfg.Keybinds) == 0 {
		return
	}
	overrides := cfg.Keybinds
	for name, b := range bindings {
		keys, ok := overrides["dialog."+name]
		if !ok {
			continue
		}
		if len(keys) == 0 {
			b.SetEnabled(false)
			continue
		}
		b.SetKeys(keys...)
		b.SetHelp(keys[0], b.Help().Desc)
	}
}

// firstKey returns the first key of a binding for help text, or the
// empty string when unbound.
func firstKey(b key.Binding) string {
	keys := b.Keys()
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

// navHelp renders the nav pair as one compact "↑/↓ choose" row while
// both keys are still the arrows. Any other keys make the order a
// guess, so it falls back to the two labeled entries.
func navHelp(next, previous key.Binding) []key.Binding {
	if firstKey(previous) == "up" && firstKey(next) == "down" {
		return []key.Binding{key.NewBinding(
			key.WithKeys(firstKey(previous), firstKey(next)),
			key.WithHelp("↑/↓", "choose"),
		)}
	}
	return []key.Binding{previous, next}
}

// shortcutFor returns the first key of an action's override for panel
// hints, or fallback when the action has no override.
func shortcutFor(com *common.Common, actionID, fallback string) string {
	if com.Workspace == nil {
		return fallback
	}
	cfg := com.Config()
	if cfg == nil {
		return fallback
	}
	keys, ok := cfg.Keybinds[actionID]
	if !ok || len(keys) == 0 {
		return fallback
	}
	return keys[0]
}
