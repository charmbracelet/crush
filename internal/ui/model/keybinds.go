package model

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"github.com/charmbracelet/crush/internal/keybinds"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/completions"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// ApplyKeybinds overlays user overrides onto km, which must already hold
// defaults from DefaultKeyMap. Unknown actions are skipped with a
// warning; the default stands. Overrides that collide with another
// action in an overlapping dispatch domain warn as well.
func ApplyKeybinds(km *KeyMap, overrides map[string][]string) []string {
	var warnings []string
	for action, keys := range overrides {
		b := bindingFor(km, action)
		if b == nil {
			// completions.* and dialog.close are registry actions
			// too, consumed outside this keymap; they are not
			// unknown.
			if scope, _, _ := strings.Cut(action, "."); scope != "completions" && scope != "dialog" {
				warnings = append(warnings, fmt.Sprintf("unknown keybind action %q; keeping default", action))
			}
			continue
		}
		if len(keys) == 0 {
			b.SetEnabled(false)
			continue
		}
		b.SetKeys(keys...)
		b.SetHelp(keys[0], b.Help().Desc)
	}
	// Conflicts and unknown actions, sorted so repeated runs warn in
	// the same order; map iteration alone would shuffle them.
	warnings = append(warnings, conflictWarnings(km, overrides)...)
	slices.Sort(warnings)
	return warnings
}

// conflictWarnings reports overridden actions whose new keys collide
// with another action in an overlapping dispatch domain. Global keys
// run inside every focus branch, so they overlap everything; editor,
// chat, and initialize keys only see their own scope. Dialog and
// completions keys are modal when active and never collide.
func conflictWarnings(km *KeyMap, overrides map[string][]string) []string {
	seen := make(map[string]bool)
	var warnings []string
	for action := range overrides {
		scope, _, _ := strings.Cut(action, ".")
		if scope == "dialog" || scope == "completions" {
			continue
		}
		b := bindingFor(km, action)
		if b == nil {
			continue
		}
		for _, other := range keybinds.Actions() {
			if other == action || !overlaps(scope, other) {
				continue
			}
			ob := bindingFor(km, other)
			if ob == nil {
				continue
			}
			for _, k := range b.Keys() {
				if !slices.Contains(ob.Keys(), k) {
					continue
				}
				pair := action + "|" + other
				reverse := other + "|" + action
				if seen[pair] || seen[reverse] {
					continue
				}
				seen[pair] = true
				warnings = append(warnings, fmt.Sprintf("keybind %q and %q both bind %q; first match wins", action, other, k))
			}
		}
	}
	return warnings
}

// overlaps reports whether a key on scope can shadow or be shadowed by
// a key on the other action's scope.
func overlaps(scope, other string) bool {
	otherScope, _, _ := strings.Cut(other, ".")
	if otherScope == "dialog" || otherScope == "completions" {
		return false
	}
	return scope == "global" || otherScope == "global" || scope == otherScope
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
	case "global.summarize":
		return &km.Summarize
	case "global.toggle_thinking":
		return &km.ToggleThinking
	case "global.toggle_compact":
		return &km.ToggleCompact
	case "global.toggle_transparent":
		return &km.ToggleTransparent
	case "global.initialize_project":
		return &km.InitializeProject
	case "global.reasoning":
		return &km.Reasoning
	case "global.notifications":
		return &km.Notifications
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

// applyUserKeybinds overlays cfg keybinds onto the default keymap and
// fans the result out to every consumer that reads bindings by value:
// the completions popup, the dialog close key, the chat item copy and
// scroll keys, and the textarea select-all binding. Call once, before
// sub-components are constructed. Returns the warnings so the caller
// can surface them in the UI; slog alone lands in the log file, which
// TUI users never read.
func applyUserKeybinds(com *common.Common, km *KeyMap, ta *textarea.Model, comp *completions.Completions) []string {
	overrides := com.Config().Keybinds
	if len(overrides) == 0 {
		return nil
	}
	warnings := ApplyKeybinds(km, overrides)
	for _, w := range warnings {
		slog.Warn(w)
	}
	if keys, ok := overrides["dialog.close"]; ok {
		if len(keys) == 0 {
			dialog.CloseKey.SetEnabled(false)
		} else {
			dialog.CloseKey.SetKeys(keys...)
			dialog.CloseKey.SetHelp(keys[0], dialog.CloseKey.Help().Desc)
		}
	}
	syncItem := func(dst *key.Binding, src key.Binding) {
		if src.Enabled() {
			dst.SetKeys(src.Keys()...)
		} else {
			dst.SetEnabled(false)
		}
	}
	syncItem(&chat.ItemCopy, km.Chat.Copy)
	syncItem(&chat.ItemScrollLeft, km.Chat.ScrollLeft)
	syncItem(&chat.ItemScrollRight, km.Chat.ScrollRight)
	syncItem(&ta.KeyMap.SelectAll, km.Editor.SelectAll)
	ta.KeyMap.SelectAll.SetHelp(km.Editor.SelectAll.Help().Key, km.Editor.SelectAll.Help().Desc)
	syncQuestion := func(dst *key.Binding, keys []string) {
		if len(keys) == 0 {
			dst.SetEnabled(false)
			return
		}
		dst.SetKeys(keys...)
		dst.SetHelp(keys[0], dst.Help().Desc)
	}
	if keys, ok := overrides["dialog.select"]; ok {
		syncQuestion(&dialog.QuestionSelect, keys)
		syncQuestion(&dialog.QuestionDone, keys)
		syncQuestion(&dialog.QuestionConfirm, keys)
		syncQuestion(&dialog.QuestionSubmit, keys)
	}
	if keys, ok := overrides["dialog.question.toggle"]; ok {
		syncQuestion(&dialog.QuestionToggle, keys)
	}
	if keys, ok := overrides["dialog.question.yes"]; ok {
		syncQuestion(&dialog.QuestionYes, keys)
	}
	if keys, ok := overrides["dialog.question.no"]; ok {
		syncQuestion(&dialog.QuestionNo, keys)
	}
	if keys, ok := overrides["dialog.question.prev_tab"]; ok {
		syncQuestion(&dialog.QuestionPrevTab, keys)
	}
	if keys, ok := overrides["dialog.question.next_tab"]; ok {
		syncQuestion(&dialog.QuestionNextTab, keys)
	}
	if keys, ok := overrides["dialog.question.newline"]; ok {
		syncQuestion(&dialog.QuestionNewline, keys)
	}
	ckm := comp.KeyMap()
	ckm.Apply(overrides)
	comp.SetKeyMap(ckm)
	return warnings
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
