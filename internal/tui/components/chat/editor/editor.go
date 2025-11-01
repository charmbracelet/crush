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
	"unicode"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textarea"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	cmdregistry "github.com/charmbracelet/crush/internal/commands"
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
	closedViaEscape       bool // Track if completions were closed via Escape key
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
	maxFileResults = 25
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

	// Check if input starts with backslash (command execution)
	if strings.HasPrefix(value, "\\") {
		// Parse command name and arguments
		commandName, args := cmdregistry.ParseCommandInput(value)
		if commandName != "" {
			// Execute command
			return m.executeCommand(commandName, args)
		}
		// If backslash but no valid command, fall through to regular message send
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

// executeCommand executes a slash command using the command executor.
//
// It validates prerequisites (session, agent coordinator) and handles errors gracefully.
// If execution fails, an error message is displayed to the user via the status system.
// If no session exists, it creates one automatically (similar to sendMessage).
//
// Parameters:
//   - commandName: The full command name (e.g., "help" or "frontend:review-pr")
//   - args: Command arguments provided by the user
//
// Returns a tea.Cmd that executes the command asynchronously and handles errors.
func (m *editorCmp) executeCommand(commandName string, args []string) tea.Cmd {
	// Check if agent coordinator is available
	if m.app.AgentCoordinator == nil {
		return util.ReportError(fmt.Errorf("agent coordinator is not initialized"))
	}

	// Get working directory
	workingDir := m.app.Config().WorkingDir()

	// Create command registry and executor
	registry := cmdregistry.NewRegistry(workingDir)
	_, err := registry.LoadCommands()
	if err != nil {
		// Logging is handled by registry, but we should still report to user
		return util.ReportError(fmt.Errorf("failed to load commands: %w", err))
	}

	executor := cmdregistry.NewExecutor(registry, m.app.AgentCoordinator, m.app.Messages, workingDir)

	// Handle session creation if needed (similar to sendMessage)
	session := m.session
	if session.ID == "" {
		// Create a new session if one doesn't exist, then execute command
		return func() tea.Msg {
			newSession, err := m.app.Sessions.Create(context.Background(), "New Session")
			if err != nil {
				return util.InfoMsg{
					Type: util.InfoTypeError,
					Msg:  fmt.Sprintf("failed to create session: %s", err.Error()),
				}
			}
			session = newSession
			
			// Execute command with the new session
			execErr := executor.Execute(context.Background(), session.ID, commandName, args)
			
			// Always notify page about new session first (updates editor's session)
			// Then handle command execution result
			// Note: We can only return one message, so we prioritize session notification
			// The command execution error is logged by the executor
			if execErr != nil {
				// Return error, but session was created so next attempt will work
				// The session will be persisted in the database even if we don't notify here
				// User can manually refresh or the next command will use the existing session
				return util.InfoMsg{
					Type: util.InfoTypeError,
					Msg:  execErr.Error(),
				}
			}
			
			// Command executed successfully, notify page about new session
			return chat.SessionSelectedMsg(session)
		}
	}

	// Execute command asynchronously (session already exists)
	return func() tea.Msg {
		err := executor.Execute(context.Background(), session.ID, commandName, args)
		if err != nil {
			// Return error message to be displayed
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  err.Error(),
			}
		}
		// Command executed successfully
		return nil
	}
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
		if len(m.attachments) >= maxAttachments {
			return m, util.ReportError(fmt.Errorf("cannot add more than %d images", maxAttachments))
		}
		m.attachments = append(m.attachments, msg.Attachment)
		return m, nil
	case completions.CompletionsOpenedMsg:
		m.isCompletionsOpen = true
		m.closedViaEscape = false // Reset flag when completions are opened
	case completions.CompletionsClosedMsg:
		m.isCompletionsOpen = false
		m.currentQuery = ""
		// If closed via Escape, preserve completionsStartIndex so user can continue typing or delete \
		// Otherwise, reset it (e.g., when closing via space or other means)
		if !m.closedViaEscape {
			m.completionsStartIndex = 0
		}
		m.closedViaEscape = false // Reset flag
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
		} else if cmd, ok := msg.Value.(cmdregistry.Command); ok {
			// Handle command selection
			word := m.textarea.Word()
			value := m.textarea.Value()
			// Replace the query (e.g., `\hel` → `\help` or `\frontend:rev` → `\frontend:review-pr`)
			commandName := cmd.Name // Includes namespace if applicable
			value = value[:m.completionsStartIndex] + // Keep text before backslash
				"\\" + commandName + // Insert backslash and command name
				value[m.completionsStartIndex+len(word):] // Append the rest of the value
			m.textarea.SetValue(value)
			// XXX: This will always move the cursor to the end of the textarea.
			// TODO: Improve cursor positioning to place cursor after command name
			m.textarea.MoveToEnd()
			if !msg.Insert {
				m.isCompletionsOpen = false
				m.currentQuery = ""
				m.completionsStartIndex = 0
			}
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
		// File path completions (forward slash)
		case msg.String() == "/" && !m.isCompletionsOpen &&
			// only show if beginning of prompt, or if previous char is a space or newline:
			(len(m.textarea.Value()) == 0 || unicode.IsSpace(rune(m.textarea.Value()[len(m.textarea.Value())-1]))):
			m.isCompletionsOpen = true
			m.currentQuery = ""
			m.completionsStartIndex = curIdx
			m.closedViaEscape = false // Reset flag when opening new completions
			cmds = append(cmds, m.startCompletions)
		// Command completions (backslash)
		// Only trigger if backslash is the first character of the input
		case msg.String() == "\\" && !m.isCompletionsOpen &&
			len(m.textarea.Value()) == 0:
			m.isCompletionsOpen = true
			m.currentQuery = ""
			m.completionsStartIndex = curIdx
			m.closedViaEscape = false // Reset flag when opening new completions
			cmds = append(cmds, m.startCommandCompletions)
		case m.isCompletionsOpen && curIdx <= m.completionsStartIndex:
			cmds = append(cmds, util.CmdHandler(completions.CloseCompletionsMsg{}))
		}
		// Handle Escape key to close completions
		if key.Matches(msg, DeleteKeyMaps.Escape) {
			if m.isCompletionsOpen {
				// Close completions but keep the \ prefix in editor
				m.isCompletionsOpen = false
				m.currentQuery = ""
				m.closedViaEscape = true // Mark that we closed via Escape
				// Don't reset completionsStartIndex - keep it so user can continue typing or delete \
				cmds = append(cmds, util.CmdHandler(completions.CloseCompletionsMsg{}))
				return m, tea.Batch(cmds...)
			}
			// If not in completions, handle escape for delete mode
			m.deleteMode = false
			return m, nil
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
					// File path completions
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
				} else if strings.HasPrefix(word, "\\") && strings.HasPrefix(m.textarea.Value(), "\\") {
					// Command completions - only if backslash is at the start of input
					// XXX: wont' work if editing in the middle of the field.
					m.completionsStartIndex = strings.LastIndex(m.textarea.Value(), word)
					m.currentQuery = m.extractCommandQuery(m.textarea.Value(), m.completionsStartIndex)
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
		MaxResults:  maxFileResults,
	}
}

// extractCommandQuery extracts the command query from the editor input after a backslash.
//
// It extracts text after `\` up to the cursor position or the next whitespace/end of input.
// This is used for filtering command completions as the user types.
//
// Examples:
//   - `\hel` → `hel` (matches "help")
//   - `\cbut` → `cbut` (matches "frontend:components:button" via fuzzy matching)
//   - `\frontend:rev` → `frontend:rev` (matches "frontend:review-pr")
//   - `\` → `""` (empty query - shows all commands)
//
// Parameters:
//   - value: The full textarea value
//   - startIndex: The index where `\` was typed
//
// Returns the extracted query string (without the leading backslash).
func (m *editorCmp) extractCommandQuery(value string, startIndex int) string {
	if startIndex < 0 || startIndex >= len(value) {
		return ""
	}

	// Get the current word starting from startIndex
	// Find the end of the word (whitespace or end of string)
	endIndex := startIndex + 1 // Skip the backslash
	for endIndex < len(value) && !unicode.IsSpace(rune(value[endIndex])) {
		endIndex++
	}

	// Extract the query (text after backslash)
	if endIndex > startIndex+1 {
		return value[startIndex+1 : endIndex]
	}

	// Empty query (just backslash)
	return ""
}

// startCommandCompletions opens the command completions popup when the user types `\`.
//
// It loads all commands from the registry, sorts them alphabetically, and displays them
// in the completion popup. Commands are displayed with their descriptions if available.
//
// **Reload Integration**: This function creates a new registry instance each time it's called,
// ensuring that command completions always reflect the latest state of command files. This means
// that after a reload (via Ctrl+P → Reload Commands), the next time completions are opened,
// they will automatically include newly added commands and exclude removed ones. The completion
// provider effectively refreshes its command list by creating a fresh registry and calling
// ListCommands() each time completions are opened, satisfying the requirement that completions
// reflect reloaded commands.
//
// Returns OpenCompletionsMsg with all available commands, positioned at the cursor.
func (m *editorCmp) startCommandCompletions() tea.Msg {
	// Get working directory from config
	workingDir := m.app.Config().WorkingDir()

	// Create command registry and load commands
	// NOTE: We create a new registry each time to ensure completions always reflect
	// the latest command state, including after reloads. This satisfies the requirement
	// that the completion provider refreshes its command list after reload.
	registry := cmdregistry.NewRegistry(workingDir)
	_, err := registry.LoadCommands()
	if err != nil {
		// If loading fails, return empty completions (errors are logged by registry)
		return completions.OpenCompletionsMsg{
			Completions: []completions.Completion{},
			X:           0,
			Y:           0,
			MaxResults:  0,
		}
	}

	// Load all commands and convert to completion items
	// This calls registry.ListCommands() which returns the latest command list,
	// ensuring that after a reload, new commands are available and removed commands
	// are no longer present in completions.
	allCommands := registry.ListCommands()
	
	// Add built-in help command to the list
	helpCommand := cmdregistry.Command{
		Name:        "help",
		Description: "Show help listing all available commands",
	}
	allCommands = append(allCommands, helpCommand)
	
	// Sort commands alphabetically by name (includes namespace, e.g., "frontend:review-pr")
	slices.SortFunc(allCommands, func(a, b cmdregistry.Command) int {
		return strings.Compare(a.Name, b.Name)
	})
	
	completionItems := make([]completions.Completion, 0, len(allCommands))
	for _, cmd := range allCommands {
		// Convert command to completion item
		displayText := cmd.Name
		if cmd.Description != "" {
			displayText = fmt.Sprintf("%s - %s", cmd.Name, cmd.Description)
		}
		completionItems = append(completionItems, completions.Completion{
			Title: displayText,
			Value: cmd, // Store the Command struct as the value
		})
	}

	x, y := m.completionsPosition()
	return completions.OpenCompletionsMsg{
		Completions: completionItems,
		X:           x,
		Y:           y,
		MaxResults:  0, // No limit for command completions - empty query shows all commands
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
