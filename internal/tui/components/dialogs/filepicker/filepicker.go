package filepicker

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/v2/filepicker"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/tui/components/core"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/styles"
)

const (
	maxAttachmentSize  = int64(5 * 1024 * 1024) // 5MB
	FilePickerID       = "filepicker"
	fileSelectionHight = 10
)

type FilePicker interface {
	dialogs.DialogModel
}

type filePicker struct {
	wWidth       int
	wHeight      int
	width        int
	filepicker   filepicker.Model
	selectedFile string
}

func NewFilePickerCmp() FilePicker {
	t := styles.CurrentTheme()
	fp := filepicker.New()
	fp.AllowedTypes = []string{".jpg", ".jpeg", ".png"}
	fp.CurrentDirectory, _ = os.UserHomeDir()
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = false
	fp.Styles = t.S().FilePicker
	fp.Cursor = ""
	fp.SetHeight(fileSelectionHight)

	return &filePicker{filepicker: fp}
}

func (m *filePicker) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m *filePicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		m.width = min(70, m.wWidth)
		styles := m.filepicker.Styles
		styles.Directory = styles.Directory.Width(m.width - 4)
		styles.Selected = styles.Selected.PaddingLeft(1).Width(m.width - 4)
		styles.DisabledSelected = styles.DisabledSelected.PaddingLeft(1).Width(m.width - 4)
		styles.File = styles.File.Width(m.width)
		m.filepicker.Styles = styles
		return m, nil
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		m.selectedFile = path
	}

	return m, cmd
}

func (m *filePicker) View() tea.View {
	t := styles.CurrentTheme()

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Add Image", m.width-4)),
		m.imagePreview(),
		m.filepicker.View(),
	)
	return tea.NewView(m.style().Render(content))
}

func (m *filePicker) currentImage() string {
	for _, ext := range m.filepicker.AllowedTypes {
		if strings.HasSuffix(m.filepicker.HighlightedPath(), ext) {
			return m.filepicker.HighlightedPath()
		}
	}
	return ""
}

func (m *filePicker) imagePreview() string {
	if m.currentImage() == "" {
		return m.imagePreviewStyle().Render()
	}

	return ""
}

func (m *filePicker) imagePreviewStyle() lipgloss.Style {
	t := styles.CurrentTheme()
	w, h := m.imagePreviewSize()
	return t.S().Base.
		Width(w).
		Height(h).
		Margin(1).
		Background(t.BgOverlay)
}

func (m *filePicker) imagePreviewSize() (int, int) {
	return m.width - 4, min(20, m.wHeight/2)
}

func (m *filePicker) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

// ID implements FilePicker.
func (m *filePicker) ID() dialogs.DialogID {
	return FilePickerID
}

// Position implements FilePicker.
func (m *filePicker) Position() (int, int) {
	row := m.wHeight/4 - 2 // just a bit above the center
	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}
