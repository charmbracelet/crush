package dialogs

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/util"
)

type DialogID string

// DialogModel represents a dialog component that can be displayed.
type DialogModel interface {
	util.Model
	Position() (int, int)
	ID() DialogID
}

// CloseCallback allows dialogs to perform cleanup when closed.
type CloseCallback interface {
	Close() tea.Cmd
}

// OpenDialogMsg is sent to open a new dialog with specified dimensions.
type OpenDialogMsg struct {
	Model DialogModel
}

// CloseDialogMsg is sent to close the topmost dialog.
type CloseDialogMsg struct{}

// DialogCmp manages a stack of dialogs with keyboard navigation.
type DialogCmp interface {
	util.Model

	Dialogs() []DialogModel
	HasDialogs() bool
	GetLayers() []*lipgloss.Layer
	ActiveModel() util.Model
	ActiveDialogID() DialogID
}

type dialogCmp struct {
	width, height int
	dialogs       []DialogModel
	idMap         map[DialogID]int
	keyMap        KeyMap
}

// NewDialogCmp creates a new dialog manager.
func NewDialogCmp() DialogCmp {
	return dialogCmp{
		dialogs: []DialogModel{},
		keyMap:  DefaultKeyMap(),
		idMap:   make(map[DialogID]int),
	}
}

func (d dialogCmp) Init() tea.Cmd {
	return nil
}

// Update handles dialog lifecycle and forwards messages to the active dialog.
func (d dialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var cmds []tea.Cmd
		d.width = msg.Width
		d.height = msg.Height
		for i := range d.dialogs {
			u, cmd := d.dialogs[i].Update(msg)
			d.dialogs[i] = u.(DialogModel)
			cmds = append(cmds, cmd)
		}
		return d, tea.Batch(cmds...)
	case OpenDialogMsg:
		return d.handleOpen(msg)
	case CloseDialogMsg:
		if len(d.dialogs) == 0 {
			return d, nil
		}
		inx := len(d.dialogs) - 1
		dialog := d.dialogs[inx]
		delete(d.idMap, dialog.ID())
		d.dialogs = d.dialogs[:len(d.dialogs)-1]
		if closeable, ok := dialog.(CloseCallback); ok {
			return d, closeable.Close()
		}
		return d, nil
	}
	if d.HasDialogs() {
		lastIndex := len(d.dialogs) - 1
		u, cmd := d.dialogs[lastIndex].Update(msg)
		d.dialogs[lastIndex] = u.(DialogModel)
		return d, cmd
	}
	return d, nil
}

func (d dialogCmp) View() string {
	return ""
}

func (d dialogCmp) handleOpen(msg OpenDialogMsg) (util.Model, tea.Cmd) {
	if d.HasDialogs() {
		dialog := d.dialogs[len(d.dialogs)-1]
		if dialog.ID() == msg.Model.ID() {
			return d, nil // Do not open a dialog if it's already the topmost one
		}
		if dialog.ID() == "quit" {
			return d, nil // Do not open dialogs on top of quit
		}
	}
	// if the dialog is already in the stack make it the last item
	if _, ok := d.idMap[msg.Model.ID()]; ok {
		existing := d.dialogs[d.idMap[msg.Model.ID()]]
		// Reuse the model so we keep the state
		msg.Model = existing
		d.dialogs = slices.Delete(d.dialogs, d.idMap[msg.Model.ID()], d.idMap[msg.Model.ID()]+1)
	}
	d.idMap[msg.Model.ID()] = len(d.dialogs)
	d.dialogs = append(d.dialogs, msg.Model)
	var cmds []tea.Cmd
	cmd := msg.Model.Init()
	cmds = append(cmds, cmd)
	_, cmd = msg.Model.Update(tea.WindowSizeMsg{
		Width:  d.width,
		Height: d.height,
	})
	cmds = append(cmds, cmd)
	return d, tea.Batch(cmds...)
}

func (d dialogCmp) Dialogs() []DialogModel {
	return d.dialogs
}

func (d dialogCmp) ActiveModel() util.Model {
	if len(d.dialogs) == 0 {
		return nil
	}
	return d.dialogs[len(d.dialogs)-1]
}

func (d dialogCmp) ActiveDialogID() DialogID {
	if len(d.dialogs) == 0 {
		return ""
	}
	return d.dialogs[len(d.dialogs)-1].ID()
}

func (d dialogCmp) GetLayers() []*lipgloss.Layer {
	layers := []*lipgloss.Layer{}
	for _, dialog := range d.Dialogs() {
		dialogView := dialog.View()
		row, col := dialog.Position()
		layers = append(layers, lipgloss.NewLayer(dialogView).X(col).Y(row))
	}
	return layers
}

func (d dialogCmp) HasDialogs() bool {
	return len(d.dialogs) > 0
}
