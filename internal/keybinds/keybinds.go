// Package keybinds maps action IDs to key tokens.
package keybinds

import (
	"fmt"
	"slices"
	"strings"
)

// actions lists every rebindable action. Anything dispatch never
// matches is left out: rebinding it would do nothing.
var actions = []string{
	"global.quit",
	"global.help",
	"global.commands",
	"global.models",
	"global.suspend",
	"global.sessions",
	"global.tab",
	"global.toggle_yolo",
	"editor.send_message",
	"editor.open_editor",
	"editor.newline",
	"editor.add_image",
	"editor.paste_image",
	"editor.paste_text",
	"editor.commands",
	"editor.attachment_delete_mode",
	"editor.escape",
	"editor.delete_all_attachments",
	"editor.history_prev",
	"editor.history_next",
	"editor.copy_selection",
	"editor.cut_selection",
	"chat.new_session",
	"chat.cancel",
	"chat.details",
	"chat.toggle_pills",
	"chat.pill_left",
	"chat.pill_right",
	"chat.down",
	"chat.up",
	"chat.down_one_item",
	"chat.up_one_item",
	"chat.page_down",
	"chat.page_up",
	"chat.half_page_down",
	"chat.half_page_up",
	"chat.home",
	"chat.end",
	"chat.end_follow",
	"chat.copy",
	"chat.clear_highlight",
	"chat.expand",
	"chat.scroll_left",
	"chat.scroll_right",
	"chat.focus_sidebar",
	"chat.focus_chat",
	"initialize.yes",
	"initialize.no",
	"initialize.enter",
	"initialize.switch",
	"completions.down",
	"completions.up",
	"completions.select",
	"completions.cancel",
	"completions.down_insert",
	"completions.up_insert",
	"dialog.close",
}

// Actions returns the action IDs in registry order.
func Actions() []string {
	return slices.Clone(actions)
}

// Valid reports whether the action is in the registry.
func Valid(action string) bool {
	return slices.Contains(actions, action)
}

// NormalizeToken maps a literal space to "space" so both spellings
// match the same key.
func NormalizeToken(token string) string {
	if token == " " {
		return "space"
	}
	return token
}

// Validate checks that the action is dotted and every key is non-empty.
// Unknown actions are allowed through here; they warn and fall back
// at apply time instead of failing the load.
func Validate(action string, keys []string) error {
	scope, name, ok := strings.Cut(action, ".")
	if !ok || scope == "" || name == "" {
		return fmt.Errorf("invalid action %q (expected scope.name)", action)
	}
	if len(keys) == 0 {
		return fmt.Errorf("action %q requires at least one key", action)
	}
	for _, k := range keys {
		if strings.TrimSpace(NormalizeToken(k)) == "" {
			return fmt.Errorf("action %q has an empty key", action)
		}
	}
	return nil
}
