package editor

import (
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
)

type EditorKeyMap struct {
	AddFile     key.Binding
	SendMessage key.Binding
	OpenEditor  key.Binding
	Newline     key.Binding
}

func DefaultEditorKeyMap() EditorKeyMap {
	return NewEditorKeyMapWithCustom(nil)
}

func NewEditorKeyMapWithCustom(customKeymaps config.KeyMaps) EditorKeyMap {
	keyMap := EditorKeyMap{
		AddFile: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "add file"),
		),
		SendMessage: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send"),
		),
		OpenEditor: key.NewBinding(
			key.WithKeys("ctrl+o"),
			key.WithHelp("ctrl+o", "open editor"),
		),
		Newline: key.NewBinding(
			key.WithKeys("shift+enter", "ctrl+j"),
			// "ctrl+j" is a common keybinding for newline in many editors. If
			// the terminal supports "shift+enter", we substitute the help text
			// to reflect that.
			key.WithHelp("ctrl+j", "newline"),
		),
	}

	// Override with custom keymaps if provided
	if customKeymaps != nil {
		if addFileKey, ok := customKeymaps["editor_add_file"]; ok {
			keyMap.AddFile = key.NewBinding(
				key.WithKeys(string(addFileKey)),
				key.WithHelp(string(addFileKey), "add file"),
			)
		}
		if sendMessageKey, ok := customKeymaps["editor_send_message"]; ok {
			keyMap.SendMessage = key.NewBinding(
				key.WithKeys(string(sendMessageKey)),
				key.WithHelp(string(sendMessageKey), "send"),
			)
		}
		if openEditorKey, ok := customKeymaps["editor_open_editor"]; ok {
			keyMap.OpenEditor = key.NewBinding(
				key.WithKeys(string(openEditorKey)),
				key.WithHelp(string(openEditorKey), "open editor"),
			)
		}
		if newlineKey, ok := customKeymaps["editor_newline"]; ok {
			keyMap.Newline = key.NewBinding(
				key.WithKeys(string(newlineKey)),
				key.WithHelp(string(newlineKey), "newline"),
			)
		} else {
			// Check if suspend key conflicts with default newline key (ctrl+j)
			if suspendKey, ok := customKeymaps["suspend"]; ok && string(suspendKey) == "ctrl+j" {
				// If suspend is mapped to ctrl+j, remove it from newline and only use shift+enter
				keyMap.Newline = key.NewBinding(
					key.WithKeys("shift+enter"),
					key.WithHelp("shift+enter", "newline"),
				)
			}
		}
	}

	return keyMap
}

// KeyBindings implements layout.KeyMapProvider
func (k EditorKeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.AddFile,
		k.SendMessage,
		k.OpenEditor,
		k.Newline,
		AttachmentsKeyMaps.AttachmentDeleteMode,
		AttachmentsKeyMaps.DeleteAllAttachments,
		AttachmentsKeyMaps.Escape,
	}
}

type DeleteAttachmentKeyMaps struct {
	AttachmentDeleteMode key.Binding
	Escape               key.Binding
	DeleteAllAttachments key.Binding
}

// TODO: update this to use the new keymap concepts
var AttachmentsKeyMaps = DeleteAttachmentKeyMaps{
	AttachmentDeleteMode: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r+{i}", "delete attachment at index i"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel delete mode"),
	),
	DeleteAllAttachments: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("ctrl+r+r", "delete all attachments"),
	),
}
