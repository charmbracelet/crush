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
	"global.summarize",
	"global.toggle_thinking",
	"global.toggle_compact",
	"global.toggle_transparent",
	"global.initialize_project",
	"global.reasoning",
	"global.notifications",
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
	"dialog.select",
	"dialog.next",
	"dialog.previous",
	"dialog.tab",
	"dialog.copy",
	"dialog.mcp_auth.skip",
	"dialog.sessions.delete",
	"dialog.sessions.rename",
	"dialog.sessions.confirm_rename",
	"dialog.sessions.cancel_rename",
	"dialog.sessions.confirm_delete",
	"dialog.sessions.cancel_delete",
	"dialog.models.edit",
	"dialog.commands.shift_tab",
	"dialog.permissions.left",
	"dialog.permissions.right",
	"dialog.permissions.allow",
	"dialog.permissions.allow_session",
	"dialog.permissions.deny",
	"dialog.permissions.toggle_diff",
	"dialog.permissions.toggle_fullscreen",
	"dialog.permissions.scroll_up",
	"dialog.permissions.scroll_down",
	"dialog.permissions.scroll_left",
	"dialog.permissions.scroll_right",
	"dialog.filepicker.forward",
	"dialog.filepicker.backward",
	"dialog.question.toggle",
	"dialog.question.yes",
	"dialog.question.no",
	"dialog.question.prev_tab",
	"dialog.question.next_tab",
	"dialog.question.newline",
}

// Actions returns the action IDs in registry order.
func Actions() []string {
	return slices.Clone(actions)
}

// Valid reports whether the action is in the registry.
func Valid(action string) bool {
	return slices.Contains(actions, action)
}

// ValidShape reports whether the action has scope.name form with
// both parts non-empty. Membership is a separate question for Valid.
func ValidShape(action string) bool {
	scope, name, ok := strings.Cut(action, ".")
	return ok && scope != "" && name != ""
}

// Validate checks that the action is dotted and every key is non-empty.
// An empty key list disables the action: the binding stays but never
// matches. Unknown actions pass; they warn and fall back at apply time
// instead of failing the load.
func Validate(action string, keys []string) error {
	if !ValidShape(action) {
		return fmt.Errorf("invalid action %q (expected scope.name)", action)
	}
	for _, k := range keys {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("action %q has an empty key", action)
		}
	}
	return nil
}
