package editor

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/filepicker"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/quit"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/editor"
)

var (
	errClipboardPlatformUnsupported = fmt.Errorf("clipboard operations are not supported on this platform")
	errClipboardUnknownFormat       = fmt.Errorf("unknown clipboard format")
)

type Editor interface {
	util.Model
	layout.Sizeable
	layout.Focusable
	layout.Help
	layout.Positional

	SetSession(session session.Session) tea.Cmd
	IsCompletionsOpen() bool
	HasAttachments() bool
	IsEmpty() bool
	Cursor() *tea.Cursor
}

type FileCompletionItem struct {
	Path string // The file path
}

type editorCmp struct {
	width              int
	height             int
	x, y               int
	app                *app.App
	session            session.Session
	textarea           textarea.Model
	attachments        []message.Attachment
	deleteMode         bool
	readyPlaceholder   string
	workingPlaceholder string

	keyMap EditorKeyMap

	// File path completions
	currentQuery          string
	completionsStartIndex int
	isCompletionsOpen     bool
}

var DeleteKeyMaps = DeleteAttachmentKeyMaps{
	AttachmentDeleteMode: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r+{i}", "delete attachment at index i"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc", "alt+esc"),
		key.WithHelp("esc", "cancel delete mode"),
	),
	DeleteAllAttachments: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("ctrl+r+r", "delete all attachments"),
	),
}

const maxFileResults = 25

type OpenEditorMsg struct {
	Text string
}

func (m *editorCmp) openEditor(value string) tea.Cmd {
	tmpfile, err := os.CreateTemp("", "msg_*.md")
	if err != nil {
		return util.ReportError(err)
	}
	defer tmpfile.Close() //nolint:errcheck
	if _, err := tmpfile.WriteString(value); err != nil {
		return util.ReportError(err)
	}
	cmd, err := editor.Command(
		"crush",
		tmpfile.Name(),
		editor.AtPosition(
			m.textarea.Line()+1,
			m.textarea.Column()+1,
		),
	)
	if err != nil {
		return util.ReportError(err)
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return util.ReportError(err)
		}
		content, err := os.ReadFile(tmpfile.Name())
		if err != nil {
			return util.ReportError(err)
		}
		if len(content) == 0 {
			return util.ReportWarn("Message is empty")
		}
		os.Remove(tmpfile.Name())
		return OpenEditorMsg{
			Text: strings.TrimSpace(string(content)),
		}
	})
}

func (m *editorCmp) Init() tea.Cmd {
	return nil
}

func (m *editorCmp) send() tea.Cmd {
	value := m.textarea.Value()
	value = strings.TrimSpace(value)

	switch value {
	case "exit", "quit":
		m.textarea.Reset()
		return util.CmdHandler(dialogs.OpenDialogMsg{Model: quit.NewQuitDialog()})
	}

	attachments := m.attachments

	if value == "" && !message.ContainsTextAttachment(attachments) {
		return nil
	}

	m.textarea.Reset()
	m.attachments = nil
	// Change the placeholder when sending a new message.
	m.randomizePlaceholders()

	return tea.Batch(
		util.CmdHandler(chat.SendMsg{
			Text:        value,
			Attachments: attachments,
		}),
	)
}

func (m *editorCmp) repositionCompletions() tea.Msg {
	x, y := m.completionsPosition()
	return completions.RepositionCompletionsMsg{X: x, Y: y}
}

func (m *editorCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, m.repositionCompletions
	case filepicker.FilePickedMsg:
		m.attachments = append(m.attachments, msg.Attachment)
		return m, nil
	case completions.CompletionsOpenedMsg:
		m.isCompletionsOpen = true
	case completions.CompletionsClosedMsg:
		m.isCompletionsOpen = false
		m.currentQuery = ""
		m.completionsStartIndex = 0
	case completions.SelectCompletionMsg:
		if !m.isCompletionsOpen {
			return m, nil
		}
		if item, ok := msg.Value.(FileCompletionItem); ok {
			word := m.textarea.Word()
			// If the selected item is a file, insert its path into the textarea
			value := m.textarea.Value()
			value = value[:m.completionsStartIndex] + // Remove the current query
				item.Path + // Insert the file path
				value[m.completionsStartIndex+len(word):] // Append the rest of the value
			// XXX: This will always move the cursor to the end of the textarea.
			m.textarea.SetValue(value)
			m.textarea.MoveToEnd()
			if !msg.Insert {
				m.isCompletionsOpen = false
				m.currentQuery = ""
				m.completionsStartIndex = 0
			}
			absPath, _ := filepath.Abs(item.Path)
			// Skip attachment if file was already read and hasn't been modified.
			lastRead := filetracker.LastReadTime(absPath)
			if !lastRead.IsZero() {
				if info, err := os.Stat(item.Path); err == nil && !info.ModTime().After(lastRead) {
					return m, nil
				}
			}
			content, err := os.ReadFile(item.Path)
			if err != nil {
				// if it fails, let the LLM handle it later.
				return m, nil
			}
			filetracker.RecordRead(absPath)
			m.attachments = append(m.attachments, message.Attachment{
				FilePath: item.Path,
				FileName: filepath.Base(item.Path),
				MimeType: mimeOf(content),
				Content:  content,
			})
		}

	case commands.OpenExternalEditorMsg:
		if m.app.AgentCoordinator.IsSessionBusy(m.session.ID) {
			return m, util.ReportWarn("Agent is working, please wait...")
		}
		return m, m.openEditor(m.textarea.Value())
	case OpenEditorMsg:
		m.textarea.SetValue(msg.Text)
		m.textarea.MoveToEnd()
	case tea.PasteMsg:
		// If pasted text has more than 2 newlines, treat it as a file attachment.
		if strings.Count(msg.Content, "\n") > 2 {
			content := []byte(msg.Content)
			if len(content) > maxAttachmentSize {
				return m, util.ReportWarn("Paste is too big (>5mb)")
			}
			name := fmt.Sprintf("paste_%d.txt", m.pasteIdx())
			mimeType := mimeOf(content)
			attachment := message.Attachment{
				FileName: name,
				FilePath: name,
				MimeType: mimeType,
				Content:  content,
			}
			return m, util.CmdHandler(filepicker.FilePickedMsg{
				Attachment: attachment,
			})
		}

		// Try to parse as a file path.
		content, path, err := filepathToFile(msg.Content)
		if err != nil {
			// Not a file path, just update the textarea normally.
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

		if len(content) > maxAttachmentSize {
			return m, util.ReportWarn("File is too big (>5mb)")
		}

		mimeType := mimeOf(content)
		attachment := message.Attachment{
			FilePath: path,
			FileName: filepath.Base(path),
			MimeType: mimeType,
			Content:  content,
		}
		if !attachment.IsText() && !attachment.IsImage() {
			return m, util.ReportWarn("Invalid file content type: " + mimeType)
		}
		return m, util.CmdHandler(filepicker.FilePickedMsg{
			Attachment: attachment,
		})

	case commands.ToggleYoloModeMsg:
		m.setEditorPrompt()
		return m, nil
	case tea.KeyPressMsg:
		cur := m.textarea.Cursor()
		curIdx := m.textarea.Width()*cur.Y + cur.X
		switch {
		// Open command palette when "/" is pressed on empty prompt
		case msg.String() == "/" && m.IsEmpty():
			return m, util.CmdHandler(dialogs.OpenDialogMsg{
				Model: commands.NewCommandDialog(m.session.ID),
			})
		// Completions
		case msg.String() == "@" && !m.isCompletionsOpen &&
			// only show if beginning of prompt, or if previous char is a space or newline:
			(len(m.textarea.Value()) == 0 || unicode.IsSpace(rune(m.textarea.Value()[len(m.textarea.Value())-1]))):
			m.isCompletionsOpen = true
			m.currentQuery = ""
			m.completionsStartIndex = curIdx
			cmds = append(cmds, m.startCompletions)
		case m.isCompletionsOpen && curIdx <= m.completionsStartIndex:
			cmds = append(cmds, util.CmdHandler(completions.CloseCompletionsMsg{}))
		}
		if key.Matches(msg, DeleteKeyMaps.AttachmentDeleteMode) {
			m.deleteMode = true
			return m, nil
		}
		if key.Matches(msg, DeleteKeyMaps.DeleteAllAttachments) && m.deleteMode {
			m.deleteMode = false
			m.attachments = nil
			return m, nil
		}
		rune := msg.Code
		if m.deleteMode && unicode.IsDigit(rune) {
			num := int(rune - '0')
			m.deleteMode = false
			if num < 10 && len(m.attachments) > num {
				if num == 0 {
					m.attachments = m.attachments[num+1:]
				} else {
					m.attachments = slices.Delete(m.attachments, num, num+1)
				}
				return m, nil
			}
		}
		if key.Matches(msg, m.keyMap.OpenEditor) {
			if m.app.AgentCoordinator.IsSessionBusy(m.session.ID) {
				return m, util.ReportWarn("Agent is working, please wait...")
			}
			return m, m.openEditor(m.textarea.Value())
		}
		if key.Matches(msg, DeleteKeyMaps.Escape) {
			m.deleteMode = false
			return m, nil
		}
		if key.Matches(msg, m.keyMap.Newline) {
			m.textarea.InsertRune('\n')
			cmds = append(cmds, util.CmdHandler(completions.CloseCompletionsMsg{}))
		}
		// Handle image paste from clipboard
		if key.Matches(msg, m.keyMap.PasteImage) {
			imageData, err := readClipboard(clipboardFormatImage)

			if err != nil || len(imageData) == 0 {
				// If no image data found, try to get text data (could be file path)
				var textData []byte
				textData, err = readClipboard(clipboardFormatText)
				if err != nil || len(textData) == 0 {
					// If clipboard is empty, show a warning
					return m, util.ReportWarn("No data found in clipboard. Note: Some terminals may not support reading image data from clipboard directly.")
				}

				// Check if the text data is a file path
				textStr := string(textData)
				// First, try to interpret as a file path (existing functionality)
				path := strings.ReplaceAll(textStr, "\\ ", " ")
				path, err = filepath.Abs(strings.TrimSpace(path))
				if err == nil {
					isAllowedType := false
					for _, ext := range filepicker.AllowedTypes {
						if strings.HasSuffix(path, ext) {
							isAllowedType = true
							break
						}
					}
					if isAllowedType {
						tooBig, _ := filepicker.IsFileTooBig(path, filepicker.MaxAttachmentSize)
						if !tooBig {
							content, err := os.ReadFile(path)
							if err == nil {
								mimeBufferSize := min(512, len(content))
								mimeType := http.DetectContentType(content[:mimeBufferSize])
								fileName := filepath.Base(path)
								attachment := message.Attachment{FilePath: path, FileName: fileName, MimeType: mimeType, Content: content}
								return m, util.CmdHandler(filepicker.FilePickedMsg{
									Attachment: attachment,
								})
							}
						}
					}
				}

				// If not a valid file path, show a warning
				return m, util.ReportWarn("No image found in clipboard")
			} else {
				// We have image data from the clipboard
				// Create a temporary file to store the clipboard image data
				tempFile, err := os.CreateTemp("", "clipboard_image_crush_*")
				if err != nil {
					return m, util.ReportError(err)
				}
				defer tempFile.Close()

				// Write clipboard content to the temporary file
				_, err = tempFile.Write(imageData)
				if err != nil {
					return m, util.ReportError(err)
				}

				// Determine the file extension based on the image data
				mimeBufferSize := min(512, len(imageData))
				mimeType := http.DetectContentType(imageData[:mimeBufferSize])

				// Create an attachment from the temporary file
				fileName := filepath.Base(tempFile.Name())
				attachment := message.Attachment{
					FilePath: tempFile.Name(),
					FileName: fileName,
					MimeType: mimeType,
					Content:  imageData,
				}

				return m, util.CmdHandler(filepicker.FilePickedMsg{
					Attachment: attachment,
				})
			}
		}
		// Handle Enter key
		if m.textarea.Focused() && key.Matches(msg, m.keyMap.SendMessage) {
			value := m.textarea.Value()
			if strings.HasSuffix(value, "\\") {
				// If the last character is a backslash, remove it and add a newline.
				m.textarea.SetValue(strings.TrimSuffix(value, "\\"))
			} else {
				// Otherwise, send the message
				return m, m.send()
			}
		}
	}

	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	if m.textarea.Focused() {
		kp, ok := msg.(tea.KeyPressMsg)
		if ok {
			if kp.String() == "space" || m.textarea.Value() == "" {
				m.isCompletionsOpen = false
				m.currentQuery = ""
				m.completionsStartIndex = 0
				cmds = append(cmds, util.CmdHandler(completions.CloseCompletionsMsg{}))
			} else {
				word := m.textarea.Word()
				if strings.HasPrefix(word, "@") {
					// XXX: wont' work if editing in the middle of the field.
					m.completionsStartIndex = strings.LastIndex(m.textarea.Value(), word)
					m.currentQuery = word[1:]
					x, y := m.completionsPosition()
					x -= len(m.currentQuery)
					m.isCompletionsOpen = true
					cmds = append(cmds,
						util.CmdHandler(completions.FilterCompletionsMsg{
							Query:  m.currentQuery,
							Reopen: m.isCompletionsOpen,
							X:      x,
							Y:      y,
						}),
					)
				} else if m.isCompletionsOpen {
					m.isCompletionsOpen = false
					m.currentQuery = ""
					m.completionsStartIndex = 0
					cmds = append(cmds, util.CmdHandler(completions.CloseCompletionsMsg{}))
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *editorCmp) setEditorPrompt() {
	if m.app.Permissions.SkipRequests() {
		m.textarea.SetPromptFunc(4, yoloPromptFunc)
		return
	}
	m.textarea.SetPromptFunc(4, normalPromptFunc)
}

func (m *editorCmp) completionsPosition() (int, int) {
	cur := m.textarea.Cursor()
	if cur == nil {
		return m.x, m.y + 1 // adjust for padding
	}
	x := cur.X + m.x
	y := cur.Y + m.y + 1 // adjust for padding
	return x, y
}

func (m *editorCmp) Cursor() *tea.Cursor {
	cursor := m.textarea.Cursor()
	if cursor != nil {
		cursor.X = cursor.X + m.x + 1
		cursor.Y = cursor.Y + m.y + 1 // adjust for padding
	}
	return cursor
}

var readyPlaceholders = [...]string{
	"Ready!",
	"Ready...",
	"Ready?",
	"Ready for instructions",
}

var workingPlaceholders = [...]string{
	"Working!",
	"Working...",
	"Brrrrr...",
	"Prrrrrrrr...",
	"Processing...",
	"Thinking...",
}

func (m *editorCmp) randomizePlaceholders() {
	m.workingPlaceholder = workingPlaceholders[rand.Intn(len(workingPlaceholders))]
	m.readyPlaceholder = readyPlaceholders[rand.Intn(len(readyPlaceholders))]
}

func (m *editorCmp) View() string {
	t := styles.CurrentTheme()
	// Update placeholder
	if m.app.AgentCoordinator != nil && m.app.AgentCoordinator.IsBusy() {
		m.textarea.Placeholder = m.workingPlaceholder
	} else {
		m.textarea.Placeholder = m.readyPlaceholder
	}
	if m.app.Permissions.SkipRequests() {
		m.textarea.Placeholder = "Yolo mode!"
	}
	if len(m.attachments) == 0 {
		return t.S().Base.Padding(1).Render(
			m.textarea.View(),
		)
	}
	return t.S().Base.Padding(0, 1, 1, 1).Render(
		lipgloss.JoinVertical(
			lipgloss.Top,
			m.attachmentsContent(),
			m.textarea.View(),
		),
	)
}

func (m *editorCmp) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	m.textarea.SetWidth(width - 2)   // adjust for padding
	m.textarea.SetHeight(height - 2) // adjust for padding
	return nil
}

func (m *editorCmp) GetSize() (int, int) {
	return m.textarea.Width(), m.textarea.Height()
}

func (m *editorCmp) attachmentsContent() string {
	var styledAttachments []string
	t := styles.CurrentTheme()
	attachmentStyle := t.S().Base.
		Padding(0, 1).
		MarginRight(1).
		Background(t.FgMuted).
		Foreground(t.FgBase).
		Render
	iconStyle := t.S().Base.
		Foreground(t.BgSubtle).
		Background(t.Green).
		Padding(0, 1).
		Bold(true).
		Render
	rmStyle := t.S().Base.
		Padding(0, 1).
		Bold(true).
		Background(t.Red).
		Foreground(t.FgBase).
		Render
	for i, attachment := range m.attachments {
		filename := ansi.Truncate(filepath.Base(attachment.FileName), 10, "...")
		icon := styles.ImageIcon
		if attachment.IsText() {
			icon = styles.TextIcon
		}
		if m.deleteMode {
			styledAttachments = append(
				styledAttachments,
				rmStyle(fmt.Sprintf("%d", i)),
				attachmentStyle(filename),
			)
			continue
		}
		styledAttachments = append(
			styledAttachments,
			iconStyle(icon),
			attachmentStyle(filename),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, styledAttachments...)
}

func (m *editorCmp) SetPosition(x, y int) tea.Cmd {
	m.x = x
	m.y = y
	return nil
}

func (m *editorCmp) startCompletions() tea.Msg {
	ls := m.app.Config().Options.TUI.Completions
	depth, limit := ls.Limits()
	files, _, _ := fsext.ListDirectory(".", nil, depth, limit)
	slices.Sort(files)
	completionItems := make([]completions.Completion, 0, len(files))
	for _, file := range files {
		file = strings.TrimPrefix(file, "./")
		completionItems = append(completionItems, completions.Completion{
			Title: file,
			Value: FileCompletionItem{
				Path: file,
			},
		})
	}

	x, y := m.completionsPosition()
	return completions.OpenCompletionsMsg{
		Completions: completionItems,
		X:           x,
		Y:           y,
		MaxResults:  maxFileResults,
	}
}

// Blur implements Container.
func (c *editorCmp) Blur() tea.Cmd {
	c.textarea.Blur()
	return nil
}

// Focus implements Container.
func (c *editorCmp) Focus() tea.Cmd {
	return c.textarea.Focus()
}

// IsFocused implements Container.
func (c *editorCmp) IsFocused() bool {
	return c.textarea.Focused()
}

// Bindings implements Container.
func (c *editorCmp) Bindings() []key.Binding {
	return c.keyMap.KeyBindings()
}

// TODO: most likely we do not need to have the session here
// we need to move some functionality to the page level
func (c *editorCmp) SetSession(session session.Session) tea.Cmd {
	c.session = session
	return nil
}

func (c *editorCmp) IsCompletionsOpen() bool {
	return c.isCompletionsOpen
}

func (c *editorCmp) HasAttachments() bool {
	return len(c.attachments) > 0
}

func (c *editorCmp) IsEmpty() bool {
	return strings.TrimSpace(c.textarea.Value()) == ""
}

func normalPromptFunc(info textarea.PromptInfo) string {
	t := styles.CurrentTheme()
	if info.LineNumber == 0 {
		if info.Focused {
			return "  > "
		}
		return "::: "
	}
	if info.Focused {
		return t.S().Base.Foreground(t.GreenDark).Render("::: ")
	}
	return t.S().Muted.Render("::: ")
}

func yoloPromptFunc(info textarea.PromptInfo) string {
	t := styles.CurrentTheme()
	if info.LineNumber == 0 {
		if info.Focused {
			return fmt.Sprintf("%s ", t.YoloIconFocused)
		} else {
			return fmt.Sprintf("%s ", t.YoloIconBlurred)
		}
	}
	if info.Focused {
		return fmt.Sprintf("%s ", t.YoloDotsFocused)
	}
	return fmt.Sprintf("%s ", t.YoloDotsBlurred)
}

func New(app *app.App) Editor {
	t := styles.CurrentTheme()
	ta := textarea.New()
	ta.SetStyles(t.S().TextArea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.Focus()
	e := &editorCmp{
		// TODO: remove the app instance from here
		app:      app,
		textarea: ta,
		keyMap:   DefaultEditorKeyMap(),
	}
	e.setEditorPrompt()

	e.randomizePlaceholders()
	e.textarea.Placeholder = e.readyPlaceholder

	return e
}

var maxAttachmentSize = 5 * 1024 * 1024 // 5MB

var pasteRE = regexp.MustCompile(`paste_(\d+).txt`)

func (m *editorCmp) pasteIdx() int {
	result := 0
	for _, at := range m.attachments {
		found := pasteRE.FindStringSubmatch(at.FileName)
		if len(found) == 0 {
			continue
		}
		idx, err := strconv.Atoi(found[1])
		if err == nil {
			result = max(result, idx)
		}
	}
	return result + 1
}

func filepathToFile(name string) ([]byte, string, error) {
	path, err := filepath.Abs(strings.TrimSpace(strings.ReplaceAll(name, "\\", "")))
	if err != nil {
		return nil, "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return content, path, nil
}

func mimeOf(content []byte) string {
	mimeBufferSize := min(512, len(content))
	return http.DetectContentType(content[:mimeBufferSize])
}
