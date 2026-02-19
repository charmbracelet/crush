package plugin

// PluginCommand represents a command that plugins can register to appear in the
// command palette (ctrl+p menu).
type PluginCommand struct {
	// ID is a unique identifier for the command.
	ID string

	// Title is the display name shown in the command palette.
	Title string

	// Description is a brief description of what the command does.
	Description string

	// Shortcut is an optional keyboard shortcut hint (e.g., "ctrl+shift+p").
	// This is display-only; the actual keybinding must be registered separately.
	Shortcut string
}

// PluginCommandHandler is called when a plugin command is selected.
// It receives the command and returns a PluginAction to perform.
type PluginCommandHandler func(cmd PluginCommand) PluginAction

// PluginAction represents an action that the plugin wants the UI to perform.
type PluginAction interface {
	isPluginAction()
}

// OpenDialogAction requests the UI to open a plugin dialog.
type OpenDialogAction struct {
	// DialogID identifies the dialog to open.
	DialogID string
}

func (OpenDialogAction) isPluginAction() {}

// SendPromptAction requests the UI to send a prompt to the agent.
type SendPromptAction struct {
	// Prompt is the text to send.
	Prompt string
}

func (SendPromptAction) isPluginAction() {}

// NoAction indicates no action should be taken.
type NoAction struct{}

func (NoAction) isPluginAction() {}

// PluginCommandRegistration contains the command and its handler.
type PluginCommandRegistration struct {
	Command PluginCommand
	Handler PluginCommandHandler
}

// commandRegistry holds registered plugin commands.
var commandRegistry []PluginCommandRegistration

// RegisterCommand registers a command to appear in the command palette.
// This should be called during plugin initialization (typically in init()).
func RegisterCommand(cmd PluginCommand, handler PluginCommandHandler) {
	commandRegistry = append(commandRegistry, PluginCommandRegistration{
		Command: cmd,
		Handler: handler,
	})
}

// RegisteredCommands returns all registered plugin commands.
func RegisteredCommands() []PluginCommandRegistration {
	return commandRegistry
}

// ClearCommands clears the command registry (for testing).
func ClearCommands() {
	commandRegistry = nil
}
