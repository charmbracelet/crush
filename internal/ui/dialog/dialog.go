package dialog

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// CloseKey is the default key binding to close dialogs.
var CloseKey = key.NewBinding(
	key.WithKeys("esc", "alt+esc"),
	key.WithHelp("esc", "exit"),
)

// Dialog is a component that can be displayed on top of the UI.
type Dialog interface {
	ID() string
	Update(msg tea.Msg) tea.Msg
	View() string
}

// Overlay manages multiple dialogs as an overlay.
type Overlay struct {
	dialogs []Dialog
}

// NewOverlay creates a new [Overlay] instance.
func NewOverlay(dialogs ...Dialog) *Overlay {
	return &Overlay{
		dialogs: dialogs,
	}
}

// HasDialogs checks if there are any active dialogs.
func (d *Overlay) HasDialogs() bool {
	return len(d.dialogs) > 0
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

// OpenDialog opens a new dialog to the stack.
func (d *Overlay) OpenDialog(dialog Dialog) {
	d.dialogs = append(d.dialogs, dialog)
}

// CloseDialog closes the dialog with the specified ID from the stack.
func (d *Overlay) CloseDialog(dialogID string) {
	for i, dialog := range d.dialogs {
		if dialog.ID() == dialogID {
			d.removeDialog(i)
			return
		}
	}
}

// CloseFrontDialog closes the front dialog in the stack.
func (d *Overlay) CloseFrontDialog() {
	if len(d.dialogs) == 0 {
		return
	}
	d.removeDialog(len(d.dialogs) - 1)
}

// Dialog returns the dialog with the specified ID, or nil if not found.
func (d *Overlay) Dialog(dialogID string) Dialog {
	for _, dialog := range d.dialogs {
		if dialog.ID() == dialogID {
			return dialog
		}
	}
	return nil
}

// DialogLast returns the front dialog, or nil if there are no dialogs.
func (d *Overlay) DialogLast() Dialog {
	if len(d.dialogs) == 0 {
		return nil
	}
	return d.dialogs[len(d.dialogs)-1]
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
func (d *Overlay) Update(msg tea.Msg) tea.Msg {
	if len(d.dialogs) == 0 {
		return nil
	}

	idx := len(d.dialogs) - 1 // active dialog is the last one
	dialog := d.dialogs[idx]
	if dialog == nil {
		return nil
	}

	return dialog.Update(msg)
}

// CenterPosition calculates the centered position for the dialog.
func (d *Overlay) CenterPosition(area uv.Rectangle, dialogID string) uv.Rectangle {
	dialog := d.Dialog(dialogID)
	if dialog == nil {
		return uv.Rectangle{}
	}
	return d.centerPositionView(area, dialog.View())
}

func (d *Overlay) centerPositionView(area uv.Rectangle, view string) uv.Rectangle {
	viewWidth := lipgloss.Width(view)
	viewHeight := lipgloss.Height(view)
	return common.CenterRect(area, viewWidth, viewHeight)
}

// Draw renders the overlay and its dialogs.
func (d *Overlay) Draw(scr uv.Screen, area uv.Rectangle) {
	for _, dialog := range d.dialogs {
		view := dialog.View()
		center := d.centerPositionView(area, view)
		if area.Overlaps(center) {
			uv.NewStyledString(view).Draw(scr, center)
		}
	}
}

// removeDialog removes a dialog from the stack.
func (d *Overlay) removeDialog(idx int) {
	if idx < 0 || idx >= len(d.dialogs) {
		return
	}
	d.dialogs = append(d.dialogs[:idx], d.dialogs[idx+1:]...)
}
