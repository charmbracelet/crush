package ui

import (
	"image"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

type uiState uint8

const (
	uiStateMain uiState = iota
)

type Model struct {
	app           *app.App
	width, height int
	state         uiState

	keyMap KeyMap

	dialog *dialog.Overlay
}

func New(app *app.App) *Model {
	return &Model{
		app:    app,
		dialog: dialog.NewOverlay(),
		keyMap: DefaultKeyMap(),
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch m.state {
		case uiStateMain:
			switch {
			case key.Matches(msg, m.keyMap.Quit):
				quitDialog := dialog.NewQuit()
				if !m.dialog.ContainsDialog(quitDialog.ID()) {
					m.dialog.AddDialog(quitDialog)
					return m, nil
				}
			}
		}
	}

	updatedDialog, cmd := m.dialog.Update(msg)
	m.dialog = updatedDialog
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) View() tea.View {
	var v tea.View

	// The screen area we're working with
	area := image.Rect(0, 0, m.width, m.height)
	layers := []*lipgloss.Layer{}

	if dialogView := m.dialog.View(); dialogView != "" {
		dialogWidth, dialogHeight := lipgloss.Width(dialogView), lipgloss.Height(dialogView)
		dialogArea := centerRect(area, dialogWidth, dialogHeight)
		layers = append(layers,
			lipgloss.NewLayer(dialogView).
				X(dialogArea.Min.X).
				Y(dialogArea.Min.Y),
		)
	}

	v.Layer = lipgloss.NewCanvas(layers...)

	return v
}

// centerRect returns a new [Rectangle] centered within the given area with the
// specified width and height.
func centerRect(area uv.Rectangle, width, height int) uv.Rectangle {
	centerX := area.Min.X + area.Dx()/2
	centerY := area.Min.Y + area.Dy()/2
	minX := centerX - width/2
	minY := centerY - height/2
	maxX := minX + width
	maxY := minY + height
	return image.Rect(minX, minY, maxX, maxY)
}
