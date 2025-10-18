package editor

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textarea"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/commandhistory"
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
	"github.com/charmbracelet/lipgloss/v2"
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
	textarea           *textarea.Model
	attachments        []message.Attachment
	deleteMode         bool
	readyPlaceholder   string
	workingPlaceholder string

	keyMap EditorKeyMap

	// File path completions
	currentQuery          string
	completionsStartIndex int
	isCompletionsOpen     bool

	// Command history fields
	history         []commandhistory.CommandHistory
	historyIndex    int
	tempInput       string
	isInHistoryMode bool
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

const (
	maxAttachments = 5
)

type OpenEditorMsg struct {
	Text string
}

func (m *editorCmp) openEditor(value string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Use platform-appropriate default editor
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "nvim"
		}
	}

	tmpfile, err := os.CreateTemp("", "msg_*.md")
	if err != nil {
		return util.ReportError(err)
	}
	defer tmpfile.Close() //nolint:errcheck
	if _, err := tmpfile.WriteString(value); err != nil {
		return util.ReportError(err)
	}
	c := exec.CommandContext(context.TODO(), editor, tmpfile.Name())
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
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

	// Add to history before processing
	cmds := []tea.Cmd{m.addToHistory(value)}

	switch value {
	case "exit", "quit":
		m.textarea.Reset()
		m.historyIndex = len(m.history)
		m.isInHistoryMode = false
		cmds = append(cmds, util.CmdHandler(dialogs.OpenDialogMsg{Model: quit.NewQuitDialog()}))
		return tea.Batch(cmds...)
	}

	m.textarea.Reset()
	m.historyIndex = len(m.history)
	m.isInHistoryMode = false
	attachments := m.attachments

	m.attachments = nil
	if value == "" {
		return tea.Batch(cmds...)
	}

	// Change the placeholder when sending a new message.
	m.randomizePlaceholders()

	cmds = append(cmds,
		util.CmdHandler(chat.SendMsg{
			Text:        value,
			Attachments: attachments,
		}),
	)
	return tea.Batch(cmds...)
}

func (m *editorCmp) repositionCompletions() tea.Msg {
	x, y := m.completionsPosition()
	return completions.RepositionCompletionsMsg{X: x, Y: y}
}

func (m *editorCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, m.repositionCompletions
	case filepicker.FilePickedMsg:
		if len(m.attachments) >= maxAttachments {
			return m, util.ReportError(fmt.Errorf("cannot add more than %d images", maxAttachments))
		}
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
		}

	case commands.OpenExternalEditorMsg:
		if m.app.CoderAgent.IsSessionBusy(m.session.ID) {
			return m, util.ReportWarn("Agent is working, please wait...")
		}
		return m, m.openEditor(m.textarea.Value())
	case OpenEditorMsg:
		m.textarea.SetValue(msg.Text)
		m.textarea.MoveToEnd()
	case tea.PasteMsg:
		path := strings.ReplaceAll(string(msg), "\\ ", " ")
		// try to get an image
		path, err := filepath.Abs(strings.TrimSpace(path))
		if err != nil {
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
		isAllowedType := false
		for _, ext := range filepicker.AllowedTypes {
			if strings.HasSuffix(path, ext) {
				isAllowedType = true
				break
			}
		}
		if !isAllowedType {
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
		tooBig, _ := filepicker.IsFileTooBig(path, filepicker.MaxAttachmentSize)
		if tooBig {
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

		content, err := os.ReadFile(path)
		if err != nil {
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
		mimeBufferSize := min(512, len(content))
		mimeType := http.DetectContentType(content[:mimeBufferSize])
		fileName := filepath.Base(path)
		attachment := message.Attachment{FilePath: path, FileName: fileName, MimeType: mimeType, Content: content}
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
		// Completions
		case msg.String() == "/" && !m.isCompletionsOpen &&
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
		// Handle history navigation before other keys
		if m.textarea.Focused() && !m.isCompletionsOpen {
			if key.Matches(msg, m.keyMap.HistoryUp) {
				return m, m.navigateHistory(-1)
			}
			if key.Matches(msg, m.keyMap.HistoryDown) {
				return m, m.navigateHistory(1)
			}
		}
		if key.Matches(msg, m.keyMap.OpenEditor) {
			if m.app.CoderAgent.IsSessionBusy(m.session.ID) {
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
				if strings.HasPrefix(word, "/") {
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

// Add method to load history for session
func (m *editorCmp) loadHistory(ctx context.Context) error {
	if m.session.ID == "" {
		return nil
	}

	history, err := m.app.CommandHistory.ListBySession(ctx, m.session.ID, 0) // 0 = no limit
	if err != nil {
		return err
	}

	m.history = history
	m.historyIndex = len(m.history)
	return nil
}

// Add method to navigate history
func (m *editorCmp) navigateHistory(direction int) tea.Cmd {
	if len(m.history) == 0 {
		return nil
	}

	// Save current input when first entering history mode
	if !m.isInHistoryMode {
		m.tempInput = m.textarea.Value()
		m.isInHistoryMode = true
	}

	newIndex := m.historyIndex + direction

	if direction < 0 { // Up arrow
		if newIndex >= 0 {
			m.historyIndex = newIndex
			// Since history is in descending order (latest first), 
			// we can access directly by index
			m.textarea.SetValue(m.history[m.historyIndex].Command)
			m.textarea.CursorEnd()
		}
	} else { // Down arrow
		if newIndex < len(m.history) {
			m.historyIndex = newIndex
			m.textarea.SetValue(m.history[m.historyIndex].Command)
			m.textarea.CursorEnd()
		} else {
			// Return to saved input
			m.textarea.SetValue(m.tempInput)
			m.textarea.CursorEnd()
			m.isInHistoryMode = false
			m.historyIndex = len(m.history)
		}
	}

	return nil
}

// Add method to add command to history
func (m *editorCmp) addToHistory(command string) tea.Cmd {
	if m.session.ID == "" || strings.TrimSpace(command) == "" {
		return nil
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := m.app.CommandHistory.Add(ctx, m.session.ID, command)
		if err != nil {
			return util.ReportError(err)
		}

		// Reload history to include the new command
		if err := m.loadHistory(ctx); err != nil {
			return util.ReportError(err)
		}

		return nil
	}
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
	if m.app.CoderAgent != nil && m.app.CoderAgent.IsBusy() {
		m.textarea.Placeholder = m.workingPlaceholder
	} else {
		m.textarea.Placeholder = m.readyPlaceholder
	}
	if m.app.Permissions.SkipRequests() {
		m.textarea.Placeholder = "Yolo mode!"
	}
	if len(m.attachments) == 0 {
		content := t.S().Base.Padding(1).Render(
			m.textarea.View(),
		)
		return content
	}
	content := t.S().Base.Padding(0, 1, 1, 1).Render(
		lipgloss.JoinVertical(lipgloss.Top,
			m.attachmentsContent(),
			m.textarea.View(),
		),
	)
	return content
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
	attachmentStyles := t.S().Base.
		MarginLeft(1).
		Background(t.FgMuted).
		Foreground(t.FgBase)
	for i, attachment := range m.attachments {
		var filename string
		if len(attachment.FileName) > 10 {
			filename = fmt.Sprintf(" %s %s...", styles.DocumentIcon, attachment.FileName[0:7])
		} else {
			filename = fmt.Sprintf(" %s %s", styles.DocumentIcon, attachment.FileName)
		}
		if m.deleteMode {
			filename = fmt.Sprintf("%d%s", i, filename)
		}
		styledAttachments = append(styledAttachments, attachmentStyles.Render(filename))
	}
	content := lipgloss.JoinHorizontal(lipgloss.Left, styledAttachments...)
	return content
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
	c.history = []commandhistory.CommandHistory{}
	c.historyIndex = 0
	c.isInHistoryMode = false
	c.tempInput = ""

	return func() tea.Msg {
		if err := c.loadHistory(context.Background()); err != nil {
			return util.ReportError(err)
		}
		return nil
	}
}

func (c *editorCmp) IsCompletionsOpen() bool {
	return c.isCompletionsOpen
}

func (c *editorCmp) HasAttachments() bool {
	return len(c.attachments) > 0
}

func normalPromptFunc(info textarea.PromptInfo) string {
	t := styles.CurrentTheme()
	if info.LineNumber == 0 {
		return "  > "
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
		app:             app,
		textarea:        ta,
		keyMap:          DefaultEditorKeyMap(),
		history:         []commandhistory.CommandHistory{},
		historyIndex:    0,
		tempInput:       "",
		isInHistoryMode: false,
	}
	e.setEditorPrompt()

	e.randomizePlaceholders()
	e.textarea.Placeholder = e.readyPlaceholder

	return e
}
