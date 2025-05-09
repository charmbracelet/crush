package dialog

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
	"github.com/opencode-ai/opencode/internal/tui/image"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	maxAttachmentSize = int64(5 * 1024 * 1024) // 5MB
	downArrow         = "down"
	upArrow           = "up"
)

type FilePrickerKeyMap struct {
	Enter          key.Binding
	Down           key.Binding
	Up             key.Binding
	Forward        key.Binding
	Backward       key.Binding
	OpenFilePicker key.Binding
	Esc            key.Binding
	InsertCWD      key.Binding
}

var filePickerKeyMap = FilePrickerKeyMap{
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select file/enter directory"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", downArrow),
		key.WithHelp("↓/j", "down"),
	),
	Up: key.NewBinding(
		key.WithKeys("k", upArrow),
		key.WithHelp("↑/k", "up"),
	),
	Forward: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "enter directory"),
	),
	Backward: key.NewBinding(
		key.WithKeys("h", "backspace"),
		key.WithHelp("h/backspace", "go back"),
	),
	OpenFilePicker: key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", "open file picker"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close/exit"),
	),
	InsertCWD: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "manual path input"),
	),
}

type filepickerCmp struct {
	basePath       string
	width          int
	height         int
	cursor         int
	err            error
	cursorChain    stack
	viewport       viewport.Model
	dirs           []os.DirEntry
	cwdDetails     *DirNode
	selectedFile   string
	cwd            textinput.Model
	ShowFilePicker bool
	app            *app.App
}

type DirNode struct {
	parent    *DirNode
	child     *DirNode
	directory string
}
type stack []int

func (s stack) Push(v int) stack {
	return append(s, v)
}

func (s stack) Pop() (stack, int) {
	l := len(s)
	return s[:l-1], s[l-1]
}

type AttachmentAddedMsg struct {
	Attachment message.Attachment
}

func (f *filepickerCmp) Init() tea.Cmd {
	return nil
}

func (f *filepickerCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		f.width = 60
		f.height = 20
		f.viewport.Width = 80
		f.viewport.Height = 22
		f.cursor = 0
		f.getCurrentFileBelowCursor()
	case tea.KeyMsg:
		if f.cwd.Focused() {
			f.cwd, cmd = f.cwd.Update(msg)
		}
		switch {
		case key.Matches(msg, filePickerKeyMap.InsertCWD):
			f.cwd.Focus()
			return f, cmd
		case key.Matches(msg, filePickerKeyMap.Esc):
			if f.cwd.Focused() {
				f.cwd.Blur()
			}
		case key.Matches(msg, filePickerKeyMap.Down):
			if !f.cwd.Focused() || msg.String() == downArrow {
				if f.cursor < len(f.dirs)-1 {
					f.cursor++
					f.getCurrentFileBelowCursor()
				}
			}
		case key.Matches(msg, filePickerKeyMap.Up):
			if !f.cwd.Focused() || msg.String() == upArrow {
				if f.cursor > 0 {
					f.cursor--
					f.getCurrentFileBelowCursor()
				}
			}
		case key.Matches(msg, filePickerKeyMap.Enter):
			var path string
			var isPathDir bool
			if f.cwd.Focused() {
				path = f.cwd.Value()
				fileInfo, err := os.Stat(path)
				if err != nil {
					logging.ErrorPersist("Invalid path")
					return f, cmd
				}
				isPathDir = fileInfo.IsDir()
			} else {
				path = filepath.Join(f.cwdDetails.directory, "/", f.dirs[f.cursor].Name())
				isPathDir = f.dirs[f.cursor].IsDir()
			}
			if isPathDir {
				newWorkingDir := DirNode{parent: f.cwdDetails, directory: path}
				f.cwdDetails.child = &newWorkingDir
				f.cwdDetails = f.cwdDetails.child
				f.cursorChain = f.cursorChain.Push(f.cursor)
				f.dirs = readDir(f.cwdDetails.directory, false)
				f.cursor = 0
				f.cwd.SetValue(f.cwdDetails.directory)
				f.getCurrentFileBelowCursor()
			} else {
				f.selectedFile = path
				return f.addAttachmentToMessage()
			}
		case key.Matches(msg, filePickerKeyMap.Esc):
			if !f.cwd.Focused() {
				f.cursorChain = make(stack, 0)
				f.cursor = 0
			} else {
				f.cwd.Blur()
			}
		case key.Matches(msg, filePickerKeyMap.Forward):
			if !f.cwd.Focused() {
				if f.dirs[f.cursor].IsDir() {
					path := filepath.Join(f.cwdDetails.directory, "/", f.dirs[f.cursor].Name())
					newWorkingDir := DirNode{parent: f.cwdDetails, directory: path}
					f.cwdDetails.child = &newWorkingDir
					f.cwdDetails = f.cwdDetails.child
					f.cursorChain = f.cursorChain.Push(f.cursor)
					f.dirs = readDir(f.cwdDetails.directory, false)
					f.cursor = 0
					f.cwd.SetValue(f.cwdDetails.directory)
					f.getCurrentFileBelowCursor()
				}
			}
		case key.Matches(msg, filePickerKeyMap.Backward):
			if !f.cwd.Focused() {
				if len(f.cursorChain) != 0 && f.cwdDetails.parent != nil {
					f.cursorChain, f.cursor = f.cursorChain.Pop()
					f.cwdDetails = f.cwdDetails.parent
					f.cwdDetails.child = nil
					f.dirs = readDir(f.cwdDetails.directory, false)
					f.cwd.SetValue(f.cwdDetails.directory)
					f.getCurrentFileBelowCursor()
				}
			}
		case key.Matches(msg, filePickerKeyMap.OpenFilePicker):
			f.dirs = readDir(f.cwdDetails.directory, false)
			f.cursor = 0
			f.getCurrentFileBelowCursor()
		}
	}
	return f, cmd
}

func (f *filepickerCmp) addAttachmentToMessage() (tea.Model, tea.Cmd) {
	modeInfo := GetSelectedModel(config.Get())
	if !modeInfo.SupportsAttachments {
		logging.ErrorPersist(fmt.Sprintf("Model %s doesn't support attachments", modeInfo.Name))
		return f, nil
	}

	selectedFilePath := f.selectedFile
	if !isExtSupported(selectedFilePath) {
		logging.ErrorPersist("Unsupported file")
		return f, nil
	}

	isFileLarge, err := image.ValidateFileSize(selectedFilePath, maxAttachmentSize)
	if err != nil {
		logging.ErrorPersist("unable to read the image")
		return f, nil
	}
	if isFileLarge {
		logging.ErrorPersist("file too large, max 5MB")
		return f, nil
	}

	content, err := os.ReadFile(selectedFilePath)
	if err != nil {
		logging.ErrorPersist("Unable read selected file")
		return f, nil
	}

	mimeBufferSize := min(512, len(content))
	mimeType := http.DetectContentType(content[:mimeBufferSize])
	fileName := filepath.Base(selectedFilePath)
	attachment := message.Attachment{FilePath: selectedFilePath, FileName: fileName, MimeType: mimeType, Content: content}
	f.selectedFile = ""
	return f, util.CmdHandler(AttachmentAddedMsg{attachment})
}

func (f *filepickerCmp) View() string {
	t := theme.CurrentTheme()
	const maxVisibleDirs = 20
	const maxWidth = 80

	adjustedWidth := maxWidth
	for _, file := range f.dirs {
		if len(file.Name()) > adjustedWidth-4 { // Account for padding
			adjustedWidth = len(file.Name()) + 4
		}
	}
	adjustedWidth = max(30, min(adjustedWidth, f.width-15)) + 1

	files := make([]string, 0, maxVisibleDirs)
	startIdx := 0

	if len(f.dirs) > maxVisibleDirs {
		halfVisible := maxVisibleDirs / 2
		if f.cursor >= halfVisible && f.cursor < len(f.dirs)-halfVisible {
			startIdx = f.cursor - halfVisible
		} else if f.cursor >= len(f.dirs)-halfVisible {
			startIdx = len(f.dirs) - maxVisibleDirs
		}
	}

	endIdx := min(startIdx+maxVisibleDirs, len(f.dirs))

	for i := startIdx; i < endIdx; i++ {
		file := f.dirs[i]
		itemStyle := styles.BaseStyle().Width(adjustedWidth)

		if i == f.cursor {
			itemStyle = itemStyle.
				Background(t.Primary()).
				Foreground(t.Background()).
				Bold(true)
		}
		filename := file.Name()

		if len(filename) > adjustedWidth-4 {
			filename = filename[:adjustedWidth-7] + "..."
		}
		if file.IsDir() {
			filename = filename + "/"
		}
		// No need to reassign filename if it's not changing

		files = append(files, itemStyle.Padding(0, 1).Render(filename))
	}

	// Pad to always show exactly 21 lines
	for len(files) < maxVisibleDirs {
		files = append(files, styles.BaseStyle().Width(adjustedWidth).Render(""))
	}

	currentPath := styles.BaseStyle().
		Height(1).
		Width(adjustedWidth).
		Render(f.cwd.View())

	viewportstyle := lipgloss.NewStyle().
		Width(f.viewport.Width).
		Background(t.Background()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.TextMuted()).
		BorderBackground(t.Background()).
		Padding(2).
		Render(f.viewport.View())
	var insertExitText string
	if f.IsCWDFocused() {
		insertExitText = "Press esc to exit typing path"
	} else {
		insertExitText = "Press i to start typing path"
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		currentPath,
		styles.BaseStyle().Width(adjustedWidth).Render(""),
		styles.BaseStyle().Width(adjustedWidth).Render(lipgloss.JoinVertical(lipgloss.Left, files...)),
		styles.BaseStyle().Width(adjustedWidth).Render(""),
		styles.BaseStyle().Foreground(t.TextMuted()).Width(adjustedWidth).Render(insertExitText),
	)

	f.cwd.SetValue(f.cwd.Value())
	contentStyle := styles.BaseStyle().Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4)

	return lipgloss.JoinHorizontal(lipgloss.Center, contentStyle.Render(content), viewportstyle)
}

type FilepickerCmp interface {
	tea.Model
	ToggleFilepicker(showFilepicker bool)
	IsCWDFocused() bool
}

func (f *filepickerCmp) ToggleFilepicker(showFilepicker bool) {
	f.ShowFilePicker = showFilepicker
}

func (f *filepickerCmp) IsCWDFocused() bool {
	return f.cwd.Focused()
}

func NewFilepickerCmp(app *app.App) FilepickerCmp {
	homepath, err := os.UserHomeDir()
	if err != nil {
		logging.Error("error loading user files")
		return nil
	}
	baseDir := DirNode{parent: nil, directory: homepath}
	dirs := readDir(homepath, false)
	viewport := viewport.New(0, 0)
	currentDirectory := textinput.New()
	currentDirectory.CharLimit = 200
	currentDirectory.Width = 44
	currentDirectory.Cursor.Blink = true
	currentDirectory.SetValue(baseDir.directory)
	return &filepickerCmp{cwdDetails: &baseDir, dirs: dirs, cursorChain: make(stack, 0), viewport: viewport, cwd: currentDirectory, app: app}
}

func (f *filepickerCmp) getCurrentFileBelowCursor() {
	if len(f.dirs) == 0 || f.cursor < 0 || f.cursor >= len(f.dirs) {
		logging.Error(fmt.Sprintf("Invalid cursor position. Dirs length: %d, Cursor: %d", len(f.dirs), f.cursor))
		f.viewport.SetContent("Preview unavailable")
		return
	}

	dir := f.dirs[f.cursor]
	filename := dir.Name()
	if !dir.IsDir() && isExtSupported(filename) {
		fullPath := f.cwdDetails.directory + "/" + dir.Name()

		go func() {
			imageString, err := image.ImagePreview(f.viewport.Width-4, fullPath)
			if err != nil {
				logging.Error(err.Error())
				f.viewport.SetContent("Preview unavailable")
				return
			}

			f.viewport.SetContent(imageString)
		}()
	} else {
		f.viewport.SetContent("Preview unavailable")
	}
}

func readDir(path string, showHidden bool) []os.DirEntry {
	logging.Info(fmt.Sprintf("Reading directory: %s", path))

	entriesChan := make(chan []os.DirEntry, 1)
	errChan := make(chan error, 1)

	go func() {
		dirEntries, err := os.ReadDir(path)
		if err != nil {
			logging.ErrorPersist(err.Error())
			errChan <- err
			return
		}
		entriesChan <- dirEntries
	}()

	select {
	case dirEntries := <-entriesChan:
		sort.Slice(dirEntries, func(i, j int) bool {
			if dirEntries[i].IsDir() == dirEntries[j].IsDir() {
				return dirEntries[i].Name() < dirEntries[j].Name()
			}
			return dirEntries[i].IsDir()
		})

		if showHidden {
			return dirEntries
		}

		var sanitizedDirEntries []os.DirEntry
		for _, dirEntry := range dirEntries {
			isHidden, _ := IsHidden(dirEntry.Name())
			if !isHidden {
				if dirEntry.IsDir() || isExtSupported(dirEntry.Name()) {
					sanitizedDirEntries = append(sanitizedDirEntries, dirEntry)
				}
			}
		}

		return sanitizedDirEntries

	case err := <-errChan:
		logging.ErrorPersist(fmt.Sprintf("Error reading directory %s", path), err)
		return []os.DirEntry{}

	case <-time.After(5 * time.Second):
		logging.ErrorPersist(fmt.Sprintf("Timeout reading directory %s", path), nil)
		return []os.DirEntry{}
	}
}

func IsHidden(file string) (bool, error) {
	return strings.HasPrefix(file, "."), nil
}

func isExtSupported(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return (ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".png")
}
