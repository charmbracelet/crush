package editor

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/crush/internal/message"
	"slices"
)

// handleSelectionKeyBindings processes selection-related key presses
func (m *editorCmp) handleSelectionKeyBindings(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	// CRITICAL: Handle SelectAll (Ctrl+A/Cmd+A) FIRST to override terminal behavior
	// This prevents terminal-wide selection and selects only input field content
	if key.Matches(msg, m.keyMap.SelectAll) {
		m.SelectAll()
		return m, nil
	}
	
	// Handle copy key using established clipboard pattern
	if key.Matches(msg, m.keyMap.Copy) {
		if m.HasSelection() {
			selectedText := m.GetSelectedText()
			if selectedText == "" {
				return m, util.ReportWarn("No text selected")
			}
			
			// Clear selection after copying (established pattern)
			m.ClearSelection()
			
			return m, tea.Sequence(
				// Use both OSC 52 and native clipboard for compatibility
				tea.SetClipboard(selectedText),
				func() tea.Msg {
					_ = clipboard.WriteAll(selectedText)
					return nil
				},
				util.ReportInfo("Selected text copied to clipboard"),
			)
		}
		return m, util.ReportWarn("No text selected")
	}
	
	// Handle line start navigation
	if key.Matches(msg, m.keyMap.LineStart) {
		m.textarea.CursorStart()
		return m, nil
	}
	
	// Clear selection when typing or moving cursor (except for copy/select all)
	if !key.Matches(msg, m.keyMap.Copy) && !key.Matches(msg, m.keyMap.SelectAll) {
		if m.HasSelection() {
			m.ClearSelection()
		}
	}
	
	return m, nil
}

// handleCompletionsKeyBindings processes completion-related key presses
func (m *editorCmp) handleCompletionsKeyBindings(msg tea.KeyPressMsg, curIdx int) (tea.Cmd, bool) {
	switch {
	// Open command palette when "/" is pressed on empty prompt
	case msg.String() == "/" && len(strings.TrimSpace(m.textarea.Value())) == 0:
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: commands.NewCommandDialog(m.session.ID),
		}), true
		
	// Completions
	case msg.String() == "@" && !m.isCompletionsOpen &&
		// only show if beginning of prompt, or if previous char is a space or newline:
		(len(m.textarea.Value()) == 0 || unicode.IsSpace(rune(m.textarea.Value()[len(m.textarea.Value())-1]))):
		m.isCompletionsOpen = true
		m.currentQuery = ""
		m.completionsStartIndex = curIdx
		return m.startCompletions, true
		
	case m.isCompletionsOpen && curIdx <= m.completionsStartIndex:
		return util.CmdHandler(completions.CloseCompletionsMsg{}), true
	}
	
	return nil, false
}

// handleAttachmentKeyBindings processes attachment-related key presses
func (m *editorCmp) handleAttachmentKeyBindings(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	// Handle attachment delete mode
	if key.Matches(msg, DeleteKeyMaps.AttachmentDeleteMode) {
		m.deleteMode = true
		return m, nil
	}
	
	// Handle delete all attachments
	if key.Matches(msg, DeleteKeyMaps.DeleteAllAttachments) && m.deleteMode {
		m.deleteMode = false
		m.attachments = nil
		return m, nil
	}
	
	// Handle escape from delete mode
	if key.Matches(msg, DeleteKeyMaps.Escape) {
		m.deleteMode = false
		return m, nil
	}
	
	// Handle digit-based attachment deletion
	rune := msg.Code
	if m.deleteMode && unicode.IsDigit(rune) {
		num := int(rune - '0')
		m.deleteMode = false
		if num < 10 && len(m.attachments) > num {
			if num == 0 {
				m.attachments = m.attachments[num+1:]
			} else {
				m.attachments = slices.Delete(m.attachments, num, num+1)
			}
			return m, nil
		}
	}
	
	return m, nil
}

// deleteAttachment helper function to remove attachment by index
func deleteAttachment(attachments []message.Attachment, index int) []message.Attachment {
	if index < 0 || index >= len(attachments) {
		return attachments
	}
	return append(attachments[:index], attachments[index+1:]...)
}

// handleEditorKeyBindings processes editor-related key presses
func (m *editorCmp) handleEditorKeyBindings(msg tea.KeyPressMsg) (util.Model, tea.Cmd) {
	// Handle open external editor
	if key.Matches(msg, m.keyMap.OpenEditor) {
		if m.app.AgentCoordinator.IsSessionBusy(m.session.ID) {
			return m, util.ReportWarn("Agent is working, please wait...")
		}
		return m, m.openEditor(m.textarea.Value())
	}
	
	// Handle newline insertion
	if key.Matches(msg, m.keyMap.Newline) {
		m.textarea.InsertRune('\n')
		return m, util.CmdHandler(completions.CloseCompletionsMsg{})
	}
	
	// Handle enter key for message sending
	if m.textarea.Focused() && key.Matches(msg, m.keyMap.SendMessage) {
		value := m.textarea.Value()
		if strings.HasSuffix(value, "\\") {
			// If the last character is a backslash, remove it and add a newline.
			m.textarea.SetValue(strings.TrimSuffix(value, "\\"))
		} else {
			// Otherwise, send the message
			return m, m.send()
		}
	}
	
	return m, nil
}