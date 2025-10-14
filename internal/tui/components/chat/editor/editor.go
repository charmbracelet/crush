package editor

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textarea"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
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

	// injected file dir lister
	listDirResolver fsext.DirectoryListerResolver

	// File path completions
	currentQuery          string
	completionsStartIndex int
	isCompletionsOpen     bool

	promptHistoryIndex int
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

	switch value {
	case "exit", "quit":
		m.textarea.Reset()
		return util.CmdHandler(dialogs.OpenDialogMsg{Model: quit.NewQuitDialog()})
	}

	m.textarea.Reset()
	attachments := m.attachments

	m.attachments = nil
	if value == "" {
		return nil
	}

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

func onCompletionItemSelect(fsys fs.FS, activeModelHasImageSupport func() (bool, string), item FileCompletionItem, insert bool, m *editorCmp) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	path := item.Path
	// check if item is an image
	if isExtOfAllowedImageType(path) {
		if imagesSupported, modelName := activeModelHasImageSupport(); !imagesSupported {
			// TODO(tauraamui): consolidate this kind of standard image attachment related warning
			return m, util.ReportWarn("File attachments are not supported by the current model: " + modelName)
		}
		slog.Debug("checking if image is too big", path, 1)
		tooBig, _ := filepicker.IsFileTooBigWithFS(os.DirFS(filepath.Dir(path)), path, filepicker.MaxAttachmentSize)
		if tooBig {
			return m, nil
		}

		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return m, nil
		}
		mimeBufferSize := min(512, len(content))
		mimeType := http.DetectContentType(content[:mimeBufferSize])
		fileName := filepath.Base(path)
		attachment := message.Attachment{FilePath: path, FileName: fileName, MimeType: mimeType, Content: content}
		cmd = util.CmdHandler(filepicker.FilePickedMsg{
			Attachment: attachment,
		})
	}

	word := m.textarea.Word()
	// If the selected item is a file, insert its path into the textarea
	originalValue := m.textarea.Value()
	newValue := originalValue[:m.completionsStartIndex] // Remove the current query
	if cmd == nil {
		newValue += path // insert the file path for non-images
	}
	newValue += originalValue[m.completionsStartIndex+len(word):] // Append the rest of the value
	// XXX: This will always move the cursor to the end of the textarea.
	m.textarea.SetValue(newValue)
	m.textarea.MoveToEnd()
	if !insert {
		m.isCompletionsOpen = false
		m.currentQuery = ""
		m.completionsStartIndex = 0
	}

	return m, cmd
}

func isExtOfAllowedImageType(path string) bool {
	isAllowedType := false
	// TODO(tauraamui) [17/09/2025]: this needs to be combined with the actual data inference/checking
	//                  of the contents that happens when we resolve the "mime" type
	for _, ext := range filepicker.AllowedTypes {
		if strings.HasSuffix(path, ext) {
			isAllowedType = true
			break
		}
	}
	return isAllowedType
}

type ResolveAbs func(path string) (string, error)

func onPaste(msg tea.PasteMsg) tea.Msg {
	return filepicker.OnPaste(filepicker.ResolveFS, string(msg))
}

func activeModelHasImageSupport() (bool, string) {
	agentCfg := config.Get().Agents["coder"]
	model := config.Get().GetModelByType(agentCfg.Model)
	return model.SupportsImages, model.Name
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
			return onCompletionItemSelect(os.DirFS("."), activeModelHasImageSupport, item, msg.Insert, m)
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
		agentCfg := config.Get().Agents["coder"]
		model := config.Get().GetModelByType(agentCfg.Model)
		if !model.SupportsImages {
			return m, util.ReportWarn("File attachments are not supported by the current model: " + model.Name)
		}
		return m, util.CmdHandler(onPaste(msg)) // inject fsys accessible from PWD
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

			cmds = append(cmds, m.startCompletions())
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
		// History
		if key.Matches(msg, m.keyMap.Previous) || key.Matches(msg, m.keyMap.Next) {
			m.textarea.SetValue(m.handleMessageHistory(msg))
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
	if perm := m.app.Permissions; perm != nil {
		if perm.SkipRequests() {
			m.textarea.SetPromptFunc(4, yoloPromptFunc)
			return
		}
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

func (m *editorCmp) startCompletions() func() tea.Msg {
	return func() tea.Msg {
		files, _, _ := m.listDirResolver()(".", nil)
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

func (m *editorCmp) getUserMessagesAsText(ctx context.Context) ([]string, error) {
	allMessages, err := m.app.Messages.List(ctx, m.session.ID)
	if err != nil {
		return nil, err
	}

	var userMessages []string
	for _, msg := range allMessages {
		if msg.Role == message.User {
			userMessages = append(userMessages, msg.Content().Text)
		}
	}
	return userMessages, nil
}

func (m *editorCmp) handleMessageHistory(msg tea.KeyMsg) string {
	ctx := context.Background()
	userMessages, err := m.getUserMessagesAsText(ctx)
	if err != nil {
		return "" // Do nothing.
	}
	userMessages = append(userMessages, "") // Give the user a reset option.
	if len(userMessages) > 0 {
		if key.Matches(msg, m.keyMap.Previous) {
			if m.promptHistoryIndex == 0 {
				m.promptHistoryIndex = len(userMessages) - 1
			} else {
				m.promptHistoryIndex -= 1
			}
		}
		if key.Matches(msg, m.keyMap.Next) {
			if m.promptHistoryIndex == len(userMessages)-1 {
				m.promptHistoryIndex = 0
			} else {
				m.promptHistoryIndex += 1
			}
		}
	}
	return userMessages[m.promptHistoryIndex]
}

func newTextArea() *textarea.Model {
	t := styles.CurrentTheme()
	ta := textarea.New()
	ta.SetStyles(t.S().TextArea)
	ta.ShowLineNumbers = false
	ta.CharLimit = -1
	ta.SetVirtualCursor(false)
	ta.Focus()
	return ta
}

func newEditor(app *app.App, resolveDirLister fsext.DirectoryListerResolver) *editorCmp {
	e := editorCmp{
		// TODO: remove the app instance from here
		app:             app,
		textarea:        newTextArea(),
		keyMap:          DefaultEditorKeyMap(),
		listDirResolver: resolveDirLister,
	}
	e.setEditorPrompt()

	e.randomizePlaceholders()
	e.textarea.Placeholder = e.readyPlaceholder

	return &e
}

func New(app *app.App) Editor {
	ls := app.Config().Options.TUI.Completions.Limits
	return newEditor(app, fsext.ResolveDirectoryLister(ls()))
}
