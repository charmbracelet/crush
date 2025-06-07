package filepicker

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/v2/filepicker"
	"github.com/charmbracelet/bubbles/v2/help"
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/image"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const (
	maxAttachmentSize  = int64(5 * 1024 * 1024) // 5MB
	FilePickerID       = "filepicker"
	fileSelectionHight = 10
)

type FilePickedMsg struct {
	FilePath string
}

type FilePicker interface {
	dialogs.DialogModel
}

type model struct {
	wWidth          int
	wHeight         int
	width           int
	filePicker      filepicker.Model
	highlightedFile string
	image           image.Model
	keyMap          KeyMap
	help            help.Model
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

	image := image.New(1, 1, "")

	help := help.New()
	help.Styles = t.S().Help
	return &model{
		filePicker: fp,
		image:      image,
		keyMap:     DefaultKeyMap(),
		help:       help,
	}
}

func (m *model) Init() tea.Cmd {
	return m.filePicker.Init()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		m.width = min(70, m.wWidth)
		styles := m.filePicker.Styles
		styles.Directory = styles.Directory.Width(m.width - 4)
		styles.Selected = styles.Selected.PaddingLeft(1).Width(m.width - 4)
		styles.DisabledSelected = styles.DisabledSelected.PaddingLeft(1).Width(m.width - 4)
		styles.File = styles.File.Width(m.width)
		m.filePicker.Styles = styles
		return m, nil
	case tea.KeyPressMsg:
		if key.Matches(msg, m.keyMap.Close) {
			return m, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		if key.Matches(msg, m.filePicker.KeyMap.Back) {
			// make sure we don't go back if we are at the home directory
			homeDir, _ := os.UserHomeDir()
			if m.filePicker.CurrentDirectory == homeDir {
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	var cmds []tea.Cmd
	m.filePicker, cmd = m.filePicker.Update(msg)
	cmds = append(cmds, cmd)
	if m.highlightedFile != m.currentImage() && m.currentImage() != "" {
		w, h := m.imagePreviewSize()
		cmd = m.image.Redraw(uint(w-2), uint(h-2), m.currentImage())
		cmds = append(cmds, cmd)
	}
	m.highlightedFile = m.currentImage()

	// Did the user select a file?
	if didSelect, path := m.filePicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		return m, tea.Sequence(
			util.CmdHandler(dialogs.CloseDialogMsg{}),
			util.CmdHandler(FilePickedMsg{FilePath: path}),
		)
	}
	m.image, cmd = m.image.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *model) View() tea.View {
	t := styles.CurrentTheme()

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Add Image", m.width-4)),
		m.imagePreview(),
		m.filePicker.View(),
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)
	return tea.NewView(m.style().Render(content))
}

func (m *model) currentImage() string {
	for _, ext := range m.filePicker.AllowedTypes {
		if strings.HasSuffix(m.filePicker.HighlightedPath(), ext) {
			return m.filePicker.HighlightedPath()
		}
	}
	return ""
}

func (m *model) imagePreview() string {
	t := styles.CurrentTheme()
	w, h := m.imagePreviewSize()
	if m.currentImage() == "" {
		imgPreview := t.S().Base.
			Width(w).
			Height(h).
			Background(t.BgOverlay)

		return m.imagePreviewStyle().Render(imgPreview.Render())
	}

	return m.imagePreviewStyle().Width(w).Height(h).Render(m.image.View())
}

func (m *model) imagePreviewStyle() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.Padding(1, 1, 1, 1)
}

func (m *model) imagePreviewSize() (int, int) {
	return m.width - 4, min(20, m.wHeight/2)
}

func (m *model) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

// ID implements FilePicker.
func (m *model) ID() dialogs.DialogID {
	return FilePickerID
}

// Position implements FilePicker.
func (m *model) Position() (int, int) {
	row := m.wHeight/4 - 2 // just a bit above the center
	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}
