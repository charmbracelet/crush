package ui

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// DialogOverlayKeyMap defines key bindings for dialogs.
type DialogOverlayKeyMap struct {
	Close key.Binding
}

// DefaultDialogOverlayKeyMap returns the default key bindings for dialogs.
func DefaultDialogOverlayKeyMap() DialogOverlayKeyMap {
	return DialogOverlayKeyMap{
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
		),
	}
}

// Dialog is a component that can be displayed on top of the UI.
type Dialog interface {
	Model[Dialog]
	ID() string
}

// Overlay manages multiple dialogs as an overlay.
type Overlay struct {
	dialogs []Dialog
	keyMap  DialogOverlayKeyMap
}

// NewDialogOverlay creates a new [Overlay] instance.
func NewDialogOverlay(dialogs ...Dialog) *Overlay {
	return &Overlay{
		dialogs: dialogs,
		keyMap:  DefaultDialogOverlayKeyMap(),
	}
}

// ContainsDialog checks if a dialog with the specified ID exists.
func (d *Overlay) ContainsDialog(dialogID string) bool {
	for _, dialog := range d.dialogs {
		if dialog.ID() == dialogID {
			return true
		}
	}
	return false
}

// AddDialog adds a new dialog to the stack.
func (d *Overlay) AddDialog(dialog Dialog) {
	d.dialogs = append(d.dialogs, dialog)
}

// BringToFront brings the dialog with the specified ID to the front.
func (d *Overlay) BringToFront(dialogID string) {
	for i, dialog := range d.dialogs {
		if dialog.ID() == dialogID {
			// Move the dialog to the end of the slice
			d.dialogs = append(d.dialogs[:i], d.dialogs[i+1:]...)
			d.dialogs = append(d.dialogs, dialog)
			return
		}
	}
}

// Update handles dialog updates.
func (d *Overlay) Update(msg tea.Msg) (*Overlay, tea.Cmd) {
	if len(d.dialogs) == 0 {
		return d, nil
	}

	idx := len(d.dialogs) - 1 // active dialog is the last one
	dialog := d.dialogs[idx]
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, d.keyMap.Close) {
			// Close the current dialog
			d.removeDialog(idx)
			return d, nil
		}
	}

	updatedDialog, cmd := dialog.Update(msg)
	if updatedDialog == nil {
		// Dialog requested to be closed
		d.removeDialog(idx)
		return d, cmd
	}

	// Update the dialog in the stack
	d.dialogs[idx] = updatedDialog

	return d, cmd
}

// View implements [Model].
func (d *Overlay) View() string {
	if len(d.dialogs) == 0 {
		return ""
	}

	// Compose all the dialogs into a single view
	canvas := lipgloss.NewCanvas()
	for _, dialog := range d.dialogs {
		layer := lipgloss.NewLayer(dialog.View())
		canvas.AddLayers(layer)
	}

	return canvas.Render()
}

// ShortHelp implements [help.KeyMap].
func (d *Overlay) ShortHelp() []key.Binding {
	return []key.Binding{
		d.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (d *Overlay) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{d.keyMap.Close},
	}
}

// removeDialog removes a dialog from the stack.
func (d *Overlay) removeDialog(idx int) {
	if idx < 0 || idx >= len(d.dialogs) {
		return
	}
	d.dialogs = append(d.dialogs[:idx], d.dialogs[idx+1:]...)
}
