package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/plugin"
	uv "github.com/charmbracelet/ultraviolet"
)

// PluginAdapterID is the prefix for plugin dialog IDs.
const PluginAdapterID = "plugin:"

// PluginDialogAdapter wraps a plugin.PluginDialog to implement Dialog.
type PluginDialogAdapter struct {
	com     *common.Common
	dialog  plugin.PluginDialog
	app     *plugin.App
	wWidth  int
	wHeight int
	keyMap  pluginDialogKeyMap
}

type pluginDialogKeyMap struct {
	Close key.Binding
}

func defaultPluginDialogKeyMap() pluginDialogKeyMap {
	return pluginDialogKeyMap{
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "close"),
		),
	}
}

// NewPluginDialogAdapter creates a new adapter for a plugin dialog.
func NewPluginDialogAdapter(com *common.Common, dialog plugin.PluginDialog, app *plugin.App) *PluginDialogAdapter {
	adapter := &PluginDialogAdapter{
		com:    com,
		dialog: dialog,
		app:    app,
		keyMap: defaultPluginDialogKeyMap(),
	}
	// Initialize the plugin dialog
	_ = dialog.Init()
	return adapter
}

// ID implements Dialog.
func (a *PluginDialogAdapter) ID() string {
	return PluginAdapterID + a.dialog.ID()
}

// HandleMsg implements Dialog.
func (a *PluginDialogAdapter) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.wWidth = msg.Width
		a.wHeight = msg.Height
		_, _, _ = a.dialog.Update(plugin.ResizeEvent{
			Width:  msg.Width,
			Height: msg.Height,
		})
		return nil

	case tea.KeyPressMsg:
		// Check for close key first
		if key.Matches(msg, a.keyMap.Close) {
			return ActionClose{}
		}

		// Convert to plugin KeyEvent
		keyStr := keyToString(msg)
		done, action, err := a.dialog.Update(plugin.KeyEvent{
			Key:   keyStr,
			Runes: []rune(msg.Text),
		})
		if err != nil {
			// Report error
			return nil
		}

		if done {
			return ActionClose{}
		}

		if action != nil {
			return a.handleAction(action)
		}
	}
	return nil
}

// handleAction converts plugin actions to dialog actions.
func (a *PluginDialogAdapter) handleAction(action plugin.PluginAction) Action {
	switch act := action.(type) {
	case plugin.OpenDialogAction:
		return ActionOpenPluginDialog{DialogID: act.DialogID}
	case plugin.SendPromptAction:
		return ActionPluginSendPrompt{Prompt: act.Prompt}
	case plugin.NoAction:
		return nil
	}
	return nil
}

// Draw implements Dialog.
func (a *PluginDialogAdapter) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := a.com.Styles
	width, _ := a.dialog.Size()

	content := a.dialog.View()
	title := a.dialog.Title()

	rc := NewRenderContext(t, width+4) // Add padding for border
	rc.Title = title
	rc.AddPart(content)
	rc.Help = a.help().View(a)

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

func (a *PluginDialogAdapter) help() help.Model {
	h := help.New()
	h.Styles = a.com.Styles.DialogHelpStyles()
	return h
}

// ShortHelp implements help.KeyMap.
func (a *PluginDialogAdapter) ShortHelp() []key.Binding {
	return []key.Binding{a.keyMap.Close}
}

// FullHelp implements help.KeyMap.
func (a *PluginDialogAdapter) FullHelp() [][]key.Binding {
	return [][]key.Binding{{a.keyMap.Close}}
}

// ActionOpenPluginDialog requests the UI to open a plugin dialog.
type ActionOpenPluginDialog struct {
	DialogID string
}

// ActionPluginSendPrompt requests the UI to send a prompt.
type ActionPluginSendPrompt struct {
	Prompt string
}

// keyToString converts a tea.KeyPressMsg to a string representation.
func keyToString(msg tea.KeyPressMsg) string {
	var parts []string
	if msg.Mod.Contains(tea.ModCtrl) {
		parts = append(parts, "ctrl")
	}
	if msg.Mod.Contains(tea.ModAlt) {
		parts = append(parts, "alt")
	}
	if msg.Mod.Contains(tea.ModShift) {
		parts = append(parts, "shift")
	}

	keyName := string(msg.Code)
	switch msg.Code {
	case tea.KeyEnter:
		keyName = "enter"
	case tea.KeyTab:
		keyName = "tab"
	case tea.KeySpace:
		keyName = "space"
	case tea.KeyBackspace:
		keyName = "backspace"
	case tea.KeyDelete:
		keyName = "delete"
	case tea.KeyUp:
		keyName = "up"
	case tea.KeyDown:
		keyName = "down"
	case tea.KeyLeft:
		keyName = "left"
	case tea.KeyRight:
		keyName = "right"
	case tea.KeyHome:
		keyName = "home"
	case tea.KeyEnd:
		keyName = "end"
	case tea.KeyPgUp:
		keyName = "pgup"
	case tea.KeyPgDown:
		keyName = "pgdown"
	case tea.KeyEscape:
		keyName = "esc"
	default:
		if len(msg.Text) > 0 {
			keyName = string(msg.Text)
		}
	}

	parts = append(parts, keyName)
	return strings.Join(parts, "+")
}
