package dialog

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

// ActionClose is a message to close the current dialog.
type ActionClose struct{}

// ActionQuit is a message to quit the application.
type ActionQuit = tea.QuitMsg

// ActionOpenDialog is a message to open a dialog.
type ActionOpenDialog struct {
	DialogID string
}

// ActionSelectSession is a message indicating a session has been selected.
type ActionSelectSession struct {
	Session session.Session
}

// ActionSelectModel is a message indicating a model has been selected.
type ActionSelectModel struct {
	Provider  catwalk.Provider
	Model     config.SelectedModel
	ModelType config.SelectedModelType
}

// Messages for commands
type (
	ActionNewSession        struct{}
	ActionToggleHelp        struct{}
	ActionToggleCompactMode struct{}
	ActionToggleThinking    struct{}
	ActionExternalEditor    struct{}
	ActionToggleYoloMode    struct{}
	// ActionInitializeProject is a message to initialize a project.
	ActionInitializeProject struct{}
	ActionSummarize         struct {
		SessionID string
	}
	ActionPermissionResponse struct {
		Permission permission.PermissionRequest
		Action     PermissionAction
	}
	// ActionRunCustomCommand is a message to run a custom command.
	ActionRunCustomCommand struct {
		CommandID string
		// Used when running a user-defined command
		Content string
		// Used when running a prompt from MCP
		Client string
	}
	// ActionOpenCustomCommandArgumentsDialog is a message to open the custom command arguments dialog.
	ActionOpenCustomCommandArgumentsDialog struct {
		CommandID string
		// Used when running a user-defined command
		Content string
		// Used when running a prompt from MCP
		Client    string
		Arguments []commands.Argument
	}
)

// Messages for API key input dialog.
type (
	ActionChangeAPIKeyState struct {
		State APIKeyInputState
	}
)

// ActionCmd represents an action that carries a [tea.Cmd] to be passed to the
// Bubble Tea program loop.
type ActionCmd struct {
	Cmd tea.Cmd
}
