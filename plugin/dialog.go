package plugin

// PluginDialog represents a dialog that plugins can display.
// The dialog model handles its own state and rendering.
type PluginDialog interface {
	// ID returns the unique identifier for this dialog.
	ID() string

	// Title returns the dialog title.
	Title() string

	// Init is called when the dialog is first opened.
	Init() error

	// Update handles input events and returns the next state.
	// Returns done=true when the dialog should close.
	Update(event DialogEvent) (done bool, action PluginAction, err error)

	// View returns the dialog content as a string.
	// The string can include ANSI escape codes for styling.
	View() string

	// Size returns the preferred width and height of the dialog.
	Size() (width, height int)
}

// DialogEvent represents an input event sent to a plugin dialog.
type DialogEvent interface {
	isDialogEvent()
}

// KeyEvent represents a key press.
type KeyEvent struct {
	Key   string // e.g., "enter", "esc", "up", "down", "a", "ctrl+c"
	Runes []rune // The runes typed (for text input)
}

func (KeyEvent) isDialogEvent() {}

// ResizeEvent is sent when the terminal is resized.
type ResizeEvent struct {
	Width  int
	Height int
}

func (ResizeEvent) isDialogEvent() {}

// PluginDialogFactory creates a dialog instance.
type PluginDialogFactory func(app *App) (PluginDialog, error)

// dialogRegistry holds registered plugin dialogs.
var dialogRegistry = make(map[string]PluginDialogFactory)

// RegisterDialog registers a dialog factory with the given ID.
// The ID should match the DialogID used in OpenDialogAction.
func RegisterDialog(id string, factory PluginDialogFactory) {
	dialogRegistry[id] = factory
}

// GetDialogFactory returns the factory for the given dialog ID.
func GetDialogFactory(id string) (PluginDialogFactory, bool) {
	factory, ok := dialogRegistry[id]
	return factory, ok
}

// RegisteredDialogs returns all registered dialog IDs.
func RegisteredDialogs() []string {
	ids := make([]string, 0, len(dialogRegistry))
	for id := range dialogRegistry {
		ids = append(ids, id)
	}
	return ids
}

// ClearDialogs clears the dialog registry (for testing).
func ClearDialogs() {
	dialogRegistry = make(map[string]PluginDialogFactory)
}
