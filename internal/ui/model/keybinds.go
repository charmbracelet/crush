package model

import (
	"fmt"
	"slices"

	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/keybinds"
)

// ApplyKeybinds overlays user overrides onto km, which must already hold
// defaults from DefaultKeyMap. Unknown actions are skipped with a
// warning; the default stands.
func ApplyKeybinds(km *KeyMap, overrides map[string][]string) []string {
	var warnings []string
	for action, keys := range overrides {
		b := bindingFor(km, action)
		if b == nil {
			warnings = append(warnings, fmt.Sprintf("unknown keybind action %q; keeping default", action))
			continue
		}
		if len(keys) == 0 {
			continue
		}
		normalized := make([]string, len(keys))
		for i, k := range keys {
			normalized[i] = keybinds.NormalizeToken(k)
		}
		b.SetKeys(normalized...)
		b.SetHelp(normalized[0], b.Help().Desc)
	}
	// Sort so repeated runs warn in the same order; map iteration
	// alone would shuffle them.
	slices.Sort(warnings)
	return warnings
}

// bindingFor returns the binding for a registry action, or nil when
// the action is unknown. Actions the registry leaves out (chat.tab and
// friends) never reach here: config validation drops them first.
func bindingFor(km *KeyMap, action string) *key.Binding {
	switch action {
	case "global.quit":
		return &km.Quit
	case "global.help":
		return &km.Help
	case "global.commands":
		return &km.Commands
	case "global.models":
		return &km.Models
	case "global.suspend":
		return &km.Suspend
	case "global.sessions":
		return &km.Sessions
	case "global.tab":
		return &km.Tab
	case "global.toggle_yolo":
		return &km.ToggleYolo
	case "editor.send_message":
		return &km.Editor.SendMessage
	case "editor.open_editor":
		return &km.Editor.OpenEditor
	case "editor.newline":
		return &km.Editor.Newline
	case "editor.add_image":
		return &km.Editor.AddImage
	case "editor.paste_image":
		return &km.Editor.PasteImage
	case "editor.paste_text":
		return &km.Editor.PasteText
	case "editor.commands":
		return &km.Editor.Commands
	case "editor.attachment_delete_mode":
		return &km.Editor.AttachmentDeleteMode
	case "editor.escape":
		return &km.Editor.Escape
	case "editor.delete_all_attachments":
		return &km.Editor.DeleteAllAttachments
	case "editor.history_prev":
		return &km.Editor.HistoryPrev
	case "editor.history_next":
		return &km.Editor.HistoryNext
	case "editor.copy_selection":
		return &km.Editor.CopySelection
	case "editor.cut_selection":
		return &km.Editor.CutSelection
	case "chat.new_session":
		return &km.Chat.NewSession
	case "chat.cancel":
		return &km.Chat.Cancel
	case "chat.details":
		return &km.Chat.Details
	case "chat.toggle_pills":
		return &km.Chat.TogglePills
	case "chat.pill_left":
		return &km.Chat.PillLeft
	case "chat.pill_right":
		return &km.Chat.PillRight
	case "chat.down":
		return &km.Chat.Down
	case "chat.up":
		return &km.Chat.Up
	case "chat.down_one_item":
		return &km.Chat.DownOneItem
	case "chat.up_one_item":
		return &km.Chat.UpOneItem
	case "chat.page_down":
		return &km.Chat.PageDown
	case "chat.page_up":
		return &km.Chat.PageUp
	case "chat.half_page_down":
		return &km.Chat.HalfPageDown
	case "chat.half_page_up":
		return &km.Chat.HalfPageUp
	case "chat.home":
		return &km.Chat.Home
	case "chat.end":
		return &km.Chat.End
	case "chat.end_follow":
		return &km.Chat.EndFollow
	case "chat.copy":
		return &km.Chat.Copy
	case "chat.clear_highlight":
		return &km.Chat.ClearHighlight
	case "chat.expand":
		return &km.Chat.Expand
	case "chat.scroll_left":
		return &km.Chat.ScrollLeft
	case "chat.scroll_right":
		return &km.Chat.ScrollRight
	case "chat.focus_sidebar":
		return &km.Chat.FocusSidebar
	case "chat.focus_chat":
		return &km.Chat.FocusChat
	case "initialize.yes":
		return &km.Initialize.Yes
	case "initialize.no":
		return &km.Initialize.No
	case "initialize.enter":
		return &km.Initialize.Enter
	case "initialize.switch":
		return &km.Initialize.Switch
	default:
		return nil
	}
}
