package filepicker

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/image"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	MaxAttachmentSize   = int64(5 * 1024 * 1024) // 5MB
	FilePickerID        = "filepicker"
	fileSelectionHeight = 10
	previewHeight       = 20
)

type FilePickedMsg struct {
	Attachment message.Attachment
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

var AllowedTypes = []string{".jpg", ".jpeg", ".png"}

func NewFilePickerCmp(workingDir string) FilePicker {
	t := styles.CurrentTheme()
	fp := filepicker.New()
	fp.AllowedTypes = AllowedTypes

	if workingDir != "" {
		fp.CurrentDirectory = workingDir
	} else {
		// Fallback to current working directory, then home directory
		if cwd, err := os.Getwd(); err == nil {
			fp.CurrentDirectory = cwd
		} else {
			fp.CurrentDirectory = home.Dir()
		}
	}

	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = false
	fp.Styles = t.S().FilePicker
	fp.Cursor = ""
	fp.SetHeight(fileSelectionHeight)

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

func (m *model) Update(msg tea.Msg) (util.Model, tea.Cmd) {
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
			if m.filePicker.CurrentDirectory == home.Dir() {
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
			func() tea.Msg {
				isFileLarge, err := IsFileTooBig(path, MaxAttachmentSize)
				if err != nil {
					return util.ReportError(fmt.Errorf("unable to read the image: %w", err))
				}
				if isFileLarge {
					return util.ReportError(fmt.Errorf("file too large, max 5MB"))
				}

				content, err := os.ReadFile(path)
				if err != nil {
					return util.ReportError(fmt.Errorf("unable to read the image: %w", err))
				}

				mimeBufferSize := min(512, len(content))
				mimeType := http.DetectContentType(content[:mimeBufferSize])
				fileName := filepath.Base(path)
				attachment := message.Attachment{FilePath: path, FileName: fileName, MimeType: mimeType, Content: content}
				return FilePickedMsg{
					Attachment: attachment,
				}
			},
		)
	}
	m.image, cmd = m.image.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	t := styles.CurrentTheme()

	strs := []string{
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Add Image", m.width-4)),
	}

	// hide image preview if the terminal is too small
	if x, y := m.imagePreviewSize(); x > 0 && y > 0 {
		strs = append(strs, m.imagePreview())
	}

	strs = append(
		strs,
		m.filePicker.View(),
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		strs...,
	)
	return m.style().Render(content)
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
	const padding = 2

	t := styles.CurrentTheme()
	w, h := m.imagePreviewSize()

	if m.currentImage() == "" {
		imgPreview := t.S().Base.
			Width(w - padding).
			Height(h - padding).
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
	if m.wHeight-fileSelectionHeight-8 > previewHeight {
		return m.width - 4, previewHeight
	}
	return 0, 0
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
	_, imageHeight := m.imagePreviewSize()
	dialogHeight := fileSelectionHeight + imageHeight + 4
	row := (m.wHeight - dialogHeight) / 2

	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}

func IsFileTooBig(filePath string, sizeLimit int64) (bool, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("error getting file info: %w", err)
	}

	if fileInfo.Size() > sizeLimit {
		return true, nil
	}

	return false, nil
}
