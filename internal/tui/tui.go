package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/pubsub"
	cmpChat "github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/components/completions"
	"github.com/opencode-ai/opencode/internal/tui/components/core/status"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/commands"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/filepicker"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/models"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/quit"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/sessions"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/page"
	"github.com/opencode-ai/opencode/internal/tui/page/chat"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// appModel represents the main application model that manages pages, dialogs, and UI state.
type appModel struct {
	width, height int
	keyMap        KeyMap

	currentPage  page.PageID
	previousPage page.PageID
	pages        map[page.PageID]util.Model
	loadedPages  map[page.PageID]bool

	status status.StatusCmp

	app *app.App

	dialog      dialogs.DialogCmp
	completions completions.Completions

	// Session
	selectedSessionID string // The ID of the currently selected session
}

// Init initializes the application model and returns initial commands.
func (a appModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmd := a.pages[a.currentPage].Init()
	cmds = append(cmds, cmd)
	a.loadedPages[a.currentPage] = true

	cmd = a.status.Init()
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

// Update handles incoming messages and updates the application state.
func (a *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a, a.handleWindowResize(msg)

	// Completions messages
	case completions.OpenCompletionsMsg, completions.FilterCompletionsMsg, completions.CloseCompletionsMsg:
		u, completionCmd := a.completions.Update(msg)
		a.completions = u.(completions.Completions)
		return a, completionCmd

	// Dialog messages
	case dialogs.OpenDialogMsg, dialogs.CloseDialogMsg:
		u, dialogCmd := a.dialog.Update(msg)
		a.dialog = u.(dialogs.DialogCmp)
		return a, dialogCmd
	case commands.ShowArgumentsDialogMsg:
		return a, util.CmdHandler(
			dialogs.OpenDialogMsg{
				Model: commands.NewCommandArgumentsDialog(
					msg.CommandID,
					msg.Content,
					msg.ArgNames,
				),
			},
		)
	// Page change messages
	case page.PageChangeMsg:
		return a, a.moveToPage(msg.ID)

	// Status Messages
	case util.InfoMsg, util.ClearStatusMsg:
		s, statusCmd := a.status.Update(msg)
		a.status = s.(status.StatusCmp)
		cmds = append(cmds, statusCmd)
		return a, tea.Batch(cmds...)

	// Session
	case cmpChat.SessionSelectedMsg:
		a.selectedSessionID = msg.ID
	case cmpChat.SessionClearedMsg:
		a.selectedSessionID = ""
	// Logs
	case pubsub.Event[logging.LogMessage]:
		// Send to the status component
		s, statusCmd := a.status.Update(msg)
		a.status = s.(status.StatusCmp)
		cmds = append(cmds, statusCmd)

		// If the current page is logs, update the logs view
		if a.currentPage == page.LogsPage {
			updated, pageCmd := a.pages[a.currentPage].Update(msg)
			a.pages[a.currentPage] = updated.(util.Model)
			cmds = append(cmds, pageCmd)
		}
		return a, tea.Batch(cmds...)
	// Commands
	case commands.SwitchSessionsMsg:
		return a, func() tea.Msg {
			allSessions, _ := a.app.Sessions.List(context.Background())
			return dialogs.OpenDialogMsg{
				Model: sessions.NewSessionDialogCmp(allSessions, a.selectedSessionID),
			}
		}

	case commands.SwitchModelMsg:
		return a, util.CmdHandler(
			dialogs.OpenDialogMsg{
				Model: models.NewModelDialogCmp(),
			},
		)
	// File Picker
	case chat.OpenFilePickerMsg:
		if a.dialog.ActiveDialogId() == filepicker.FilePickerID {
			// If the commands dialog is already open, close it
			return a, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: filepicker.NewFilePickerCmp(),
		})
	case tea.KeyPressMsg:
		return a, a.handleKeyPressMsg(msg)
	}
	s, _ := a.status.Update(msg)
	a.status = s.(status.StatusCmp)
	updated, cmd := a.pages[a.currentPage].Update(msg)
	a.pages[a.currentPage] = updated.(util.Model)
	if a.dialog.HasDialogs() {
		u, dialogCmd := a.dialog.Update(msg)
		a.dialog = u.(dialogs.DialogCmp)
		cmds = append(cmds, dialogCmd)
	}
	cmds = append(cmds, cmd)
	return a, tea.Batch(cmds...)
}

// handleWindowResize processes window resize events and updates all components.
func (a *appModel) handleWindowResize(msg tea.WindowSizeMsg) tea.Cmd {
	var cmds []tea.Cmd
	msg.Height -= 1 // Make space for the status bar
	a.width, a.height = msg.Width, msg.Height

	// Update status bar
	s, cmd := a.status.Update(msg)
	a.status = s.(status.StatusCmp)
	cmds = append(cmds, cmd)

	// Update the current page
	updated, cmd := a.pages[a.currentPage].Update(msg)
	a.pages[a.currentPage] = updated.(util.Model)
	cmds = append(cmds, cmd)

	// Update the dialogs
	dialog, cmd := a.dialog.Update(msg)
	a.dialog = dialog.(dialogs.DialogCmp)
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

// handleKeyPressMsg processes keyboard input and routes to appropriate handlers.
func (a *appModel) handleKeyPressMsg(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	// completions
	case a.completions.Open() && key.Matches(msg, a.completions.KeyMap().Up):
		u, cmd := a.completions.Update(msg)
		a.completions = u.(completions.Completions)
		return cmd

	case a.completions.Open() && key.Matches(msg, a.completions.KeyMap().Down):
		u, cmd := a.completions.Update(msg)
		a.completions = u.(completions.Completions)
		return cmd
	case a.completions.Open() && key.Matches(msg, a.completions.KeyMap().Select):
		u, cmd := a.completions.Update(msg)
		a.completions = u.(completions.Completions)
		return cmd
	case a.completions.Open() && key.Matches(msg, a.completions.KeyMap().Cancel):
		u, cmd := a.completions.Update(msg)
		a.completions = u.(completions.Completions)
		return cmd
	// dialogs
	case key.Matches(msg, a.keyMap.Quit):
		if a.dialog.ActiveDialogId() == quit.QuitDialogID {
			// if the quit dialog is already open, close the app
			return tea.Quit
		}
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: quit.NewQuitDialog(),
		})

	case key.Matches(msg, a.keyMap.Commands):
		if a.dialog.ActiveDialogId() == commands.CommandsDialogID {
			// If the commands dialog is already open, close it
			return util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: commands.NewCommandDialog(),
		})
	case key.Matches(msg, a.keyMap.Sessions):
		if a.dialog.ActiveDialogId() == sessions.SessionsDialogID {
			// If the sessions dialog is already open, close it
			return util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		var cmds []tea.Cmd
		if a.dialog.ActiveDialogId() == commands.CommandsDialogID {
			// If the commands dialog is open, close it first
			cmds = append(cmds, util.CmdHandler(dialogs.CloseDialogMsg{}))
		}
		cmds = append(cmds,
			func() tea.Msg {
				allSessions, _ := a.app.Sessions.List(context.Background())
				return dialogs.OpenDialogMsg{
					Model: sessions.NewSessionDialogCmp(allSessions, a.selectedSessionID),
				}
			},
		)
		return tea.Sequence(cmds...)
	// Page navigation
	case key.Matches(msg, a.keyMap.Logs):
		return a.moveToPage(page.LogsPage)

	default:
		if a.dialog.HasDialogs() {
			u, dialogCmd := a.dialog.Update(msg)
			a.dialog = u.(dialogs.DialogCmp)
			return dialogCmd
		} else {
			updated, cmd := a.pages[a.currentPage].Update(msg)
			a.pages[a.currentPage] = updated.(util.Model)
			return cmd
		}
	}
}

// moveToPage handles navigation between different pages in the application.
func (a *appModel) moveToPage(pageID page.PageID) tea.Cmd {
	if a.app.CoderAgent.IsBusy() {
		// For now we don't move to any page if the agent is busy
		return util.ReportWarn("Agent is busy, please wait...")
	}

	var cmds []tea.Cmd
	if _, ok := a.loadedPages[pageID]; !ok {
		cmd := a.pages[pageID].Init()
		cmds = append(cmds, cmd)
		a.loadedPages[pageID] = true
	}
	a.previousPage = a.currentPage
	a.currentPage = pageID
	if sizable, ok := a.pages[a.currentPage].(layout.Sizeable); ok {
		cmd := sizable.SetSize(a.width, a.height)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

// View renders the complete application interface including pages, dialogs, and overlays.
func (a *appModel) View() tea.View {
	pageView := a.pages[a.currentPage].View()
	components := []string{
		pageView.String(),
	}
	components = append(components, a.status.View().String())

	appView := lipgloss.JoinVertical(lipgloss.Top, components...)
	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(appView),
	}
	if a.dialog.HasDialogs() {
		logging.Info("Rendering dialogs")
		layers = append(
			layers,
			a.dialog.GetLayers()...,
		)
	}

	cursor := pageView.Cursor()
	activeView := a.dialog.ActiveView()
	if activeView != nil {
		cursor = activeView.Cursor()
	}

	if a.completions.Open() && cursor != nil {
		cmp := a.completions.View().String()
		x, y := a.completions.Position()
		layers = append(
			layers,
			lipgloss.NewLayer(cmp).X(x).Y(y),
		)
	}

	canvas := lipgloss.NewCanvas(
		layers...,
	)

	t := styles.CurrentTheme()
	view := tea.NewView(canvas.Render())
	view.SetBackgroundColor(t.BgBase)
	view.SetCursor(cursor)
	return view
}

// New creates and initializes a new TUI application model.
func New(app *app.App) tea.Model {
	startPage := chat.ChatPage
	model := &appModel{
		currentPage: startPage,
		app:         app,
		status:      status.NewStatusCmp(),
		loadedPages: make(map[page.PageID]bool),
		keyMap:      DefaultKeyMap(),

		pages: map[page.PageID]util.Model{
			chat.ChatPage: chat.NewChatPage(app),
			page.LogsPage: page.NewLogsPage(),
		},

		dialog:      dialogs.NewDialogCmp(),
		completions: completions.New(),
	}

	return model
}
