package dialog

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/session"
)

// CloseMsg is a message to close the current dialog.
type CloseMsg struct{}

// QuitMsg is a message to quit the application.
type QuitMsg = tea.QuitMsg

// OpenDialogMsg is a message to open a dialog.
type OpenDialogMsg struct {
	DialogID string
}

// SessionSelectedMsg is a message indicating a session has been selected.
type SessionSelectedMsg struct {
	Session session.Session
}

// ModelSelectedMsg is a message indicating a model has been selected.
type ModelSelectedMsg struct {
	Provider catwalk.Provider
	Model    catwalk.Model
}

// Messages for commands
type (
	NewSessionsMsg         struct{}
	OpenFilePickerMsg      struct{}
	ToggleHelpMsg          struct{}
	ToggleCompactModeMsg   struct{}
	ToggleThinkingMsg      struct{}
	OpenReasoningDialogMsg struct{}
	OpenExternalEditorMsg  struct{}
	ToggleYoloModeMsg      struct{}
	CompactMsg             struct {
		SessionID string
	}
)
