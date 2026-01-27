package model

import (
	"context"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/message"
)

// promptHistoryLoadedMsg is sent when prompt history is loaded.
type promptHistoryLoadedMsg struct {
	messages []string
}

// loadPromptHistory loads user messages for history navigation.
func (m *UI) loadPromptHistory() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var messages []message.Message
		var err error

		if m.session != nil {
			messages, err = m.com.App.Messages.ListUserMessages(ctx, m.session.ID)
		} else {
			messages, err = m.com.App.Messages.ListAllUserMessages(ctx)
		}
		if err != nil {
			slog.Error("failed to load prompt history", "error", err)
			return promptHistoryLoadedMsg{messages: nil}
		}

		texts := make([]string, 0, len(messages))
		for _, msg := range messages {
			if text := msg.Content().Text; text != "" {
				texts = append(texts, text)
			}
		}
		return promptHistoryLoadedMsg{messages: texts}
	}
}

// handleHistoryUp handles up arrow for history navigation.
func (m *UI) handleHistoryUp(msg tea.Msg) tea.Cmd {
	// Navigate to older history entry from cursor position (0,0).
	if m.textarea.Length() == 0 || m.isAtEditorStart() {
		if m.historyPrev() {
			// we send this so that the textarea moves the view to the correct position
			// without this the cursor will show up in the wrong place.
			ta, cmd := m.textarea.Update(nil)
			m.textarea = ta
			return cmd
		}
	}

	// First move cursor to start before entering history.
	if m.textarea.Line() == 0 {
		m.textarea.CursorStart()
		return nil
	}

	// Let textarea handle normal cursor movement.
	ta, cmd := m.textarea.Update(msg)
	m.textarea = ta
	return cmd
}

// handleHistoryDown handles down arrow for history navigation.
func (m *UI) handleHistoryDown(msg tea.Msg) tea.Cmd {
	// Navigate to newer history entry from end of text.
	if m.isAtEditorEnd() {
		if m.historyNext() {
			// we send this so that the textarea moves the view to the correct position
			// without this the cursor will show up in the wrong place.
			ta, cmd := m.textarea.Update(nil)
			m.textarea = ta
			return cmd
		}
	}

	// First move cursor to end before navigating history.
	if m.textarea.Line() == max(m.textarea.LineCount()-1, 0) {
		m.textarea.MoveToEnd()
		ta, cmd := m.textarea.Update(nil)
		m.textarea = ta
		return cmd
	}

	// Let textarea handle normal cursor movement.
	ta, cmd := m.textarea.Update(msg)
	m.textarea = ta
	return cmd
}

// handleHistoryEscape handles escape for exiting history navigation.
func (m *UI) handleHistoryEscape(msg tea.Msg) tea.Cmd {
	// Return to current draft when browsing history.
	if m.promptHistory.index >= 0 {
		m.promptHistory.index = -1
		m.textarea.Reset()
		m.textarea.InsertString(m.promptHistory.draft)
		ta, cmd := m.textarea.Update(nil)
		m.textarea = ta
		return cmd
	}

	// Let textarea handle escape normally.
	ta, cmd := m.textarea.Update(msg)
	m.textarea = ta
	return cmd
}

// updateHistoryDraft updates history state when text is modified.
func (m *UI) updateHistoryDraft(oldValue string) {
	if m.textarea.Value() != oldValue {
		m.promptHistory.draft = m.textarea.Value()
		m.promptHistory.index = -1
	}
}

// historyPrev changes the text area content to the previous message in the history
// it returns false if it could not find the previous message.
func (m *UI) historyPrev() bool {
	if len(m.promptHistory.messages) == 0 {
		return false
	}
	if m.promptHistory.index == -1 {
		m.promptHistory.draft = m.textarea.Value()
	}
	nextIndex := m.promptHistory.index + 1
	if nextIndex >= len(m.promptHistory.messages) {
		return false
	}
	m.promptHistory.index = nextIndex
	m.textarea.Reset()
	m.textarea.InsertString(m.promptHistory.messages[nextIndex])
	m.textarea.MoveToBegin()
	return true
}

// historyNext changes the text area content to the next message in the history
// it returns false if it could not find the next message.
func (m *UI) historyNext() bool {
	if m.promptHistory.index < 0 {
		return false
	}
	nextIndex := m.promptHistory.index - 1
	if nextIndex < 0 {
		m.promptHistory.index = -1
		m.textarea.Reset()
		m.textarea.InsertString(m.promptHistory.draft)
		return true
	}
	m.promptHistory.index = nextIndex
	m.textarea.Reset()
	m.textarea.InsertString(m.promptHistory.messages[nextIndex])
	return true
}

// historyReset resets the history, but does not clear the message
// it just sets the current draft to empty and the position in the history.
func (m *UI) historyReset() {
	m.promptHistory.index = -1
	m.promptHistory.draft = ""
}

// isAtEditorStart returns true if we are at the 0 line and 0 col in the textarea.
func (m *UI) isAtEditorStart() bool {
	return m.textarea.Line() == 0 && m.textarea.LineInfo().ColumnOffset == 0
}

// isAtEditorEnd returns true if we are in the last line and the last column in the textarea.
func (m *UI) isAtEditorEnd() bool {
	lineCount := m.textarea.LineCount()
	if lineCount == 0 {
		return true
	}
	if m.textarea.Line() != lineCount-1 {
		return false
	}
	info := m.textarea.LineInfo()
	return info.CharOffset >= info.CharWidth-1 || info.CharWidth == 0
}
