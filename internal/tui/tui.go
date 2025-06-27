package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	configv2 "github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	cmpChat "github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/core/status"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/compact"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/filepicker"
	initDialog "github.com/charmbracelet/crush/internal/tui/components/dialogs/init"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/models"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/permissions"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/quit"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/sessions"
	"github.com/charmbracelet/crush/internal/tui/page"
	"github.com/charmbracelet/crush/internal/tui/page/chat"
	"github.com/charmbracelet/crush/internal/tui/page/logs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

// appModel represents the main application model that manages pages, dialogs, and UI state.
type appModel struct {
	wWidth, wHeight int // Window dimensions
	width, height   int
	keyMap          KeyMap

	currentPage  page.PageID
	previousPage page.PageID
	pages        map[page.PageID]util.Model
	loadedPages  map[page.PageID]bool

	// Status
	status          status.StatusCmp
	showingFullHelp bool

	app *app.App

	dialog      dialogs.DialogCmp
	completions completions.Completions

	// Chat Page Specific
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

	// Check if we should show the init dialog
	cmds = append(cmds, func() tea.Msg {
		shouldShow, err := configv2.ProjectNeedsInitialization()
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "Failed to check init status: " + err.Error(),
			}
		}
		if shouldShow {
			return dialogs.OpenDialogMsg{
				Model: initDialog.NewInitDialogCmp(),
			}
		}
		return nil
	})

	return tea.Batch(cmds...)
}

// Update handles incoming messages and updates the application state.
func (a *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyboardEnhancementsMsg:
		return a, nil
	case tea.WindowSizeMsg:
		return a, a.handleWindowResize(msg.Width, msg.Height)

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
		if a.currentPage == logs.LogsPage {
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
	// Compact
	case commands.CompactMsg:
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: compact.NewCompactDialogCmp(a.app.CoderAgent, msg.SessionID, true),
		})

	// Model Switch
	case models.ModelSelectedMsg:
		model, err := a.app.CoderAgent.Update(msg.Model)
		if err != nil {
			return a, util.ReportError(err)
		}

		return a, util.ReportInfo(fmt.Sprintf("Model changed to %s", model.Name))

	// File Picker
	case chat.OpenFilePickerMsg:
		if a.dialog.ActiveDialogID() == filepicker.FilePickerID {
			// If the commands dialog is already open, close it
			return a, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: filepicker.NewFilePickerCmp(),
		})
	// Permissions
	case pubsub.Event[permission.PermissionRequest]:
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: permissions.NewPermissionDialogCmp(msg.Payload),
		})
	case permissions.PermissionResponseMsg:
		switch msg.Action {
		case permissions.PermissionAllow:
			a.app.Permissions.Grant(msg.Permission)
		case permissions.PermissionAllowForSession:
			a.app.Permissions.GrantPersistent(msg.Permission)
		case permissions.PermissionDeny:
			a.app.Permissions.Deny(msg.Permission)
		}
		return a, nil
	// Agent Events
	case pubsub.Event[agent.AgentEvent]:
		payload := msg.Payload

		// Forward agent events to dialogs
		if a.dialog.HasDialogs() && a.dialog.ActiveDialogID() == compact.CompactDialogID {
			u, dialogCmd := a.dialog.Update(payload)
			a.dialog = u.(dialogs.DialogCmp)
			cmds = append(cmds, dialogCmd)
		}

		// Handle auto-compact logic
		if payload.Done && payload.Type == agent.AgentEventTypeResponse && a.selectedSessionID != "" {
			// Get current session to check token usage
			session, err := a.app.Sessions.Get(context.Background(), a.selectedSessionID)
			if err == nil {
				model := a.app.CoderAgent.Model()
				contextWindow := model.ContextWindow
				tokens := session.CompletionTokens + session.PromptTokens
				if (tokens >= int64(float64(contextWindow)*0.95)) && !config.Get().Options.DisableAutoSummarize {
					// Show compact confirmation dialog
					cmds = append(cmds, util.CmdHandler(dialogs.OpenDialogMsg{
						Model: compact.NewCompactDialogCmp(a.app.CoderAgent, a.selectedSessionID, false),
					}))
				}
			}
		}

		return a, tea.Batch(cmds...)
	// Key Press Messages
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
func (a *appModel) handleWindowResize(width, height int) tea.Cmd {
	var cmds []tea.Cmd
	a.wWidth, a.wHeight = width, height
	if a.showingFullHelp {
		height -= 4
	} else {
		height -= 2
	}
	a.width, a.height = width, height
	// Update status bar
	s, cmd := a.status.Update(tea.WindowSizeMsg{Width: width, Height: height})
	a.status = s.(status.StatusCmp)
	cmds = append(cmds, cmd)

	// Update the current page
	for p, page := range a.pages {
		updated, pageCmd := page.Update(tea.WindowSizeMsg{Width: width, Height: height})
		a.pages[p] = updated.(util.Model)
		cmds = append(cmds, pageCmd)
	}

	// Update the dialogs
	dialog, cmd := a.dialog.Update(tea.WindowSizeMsg{Width: width, Height: height})
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
		// help
	case key.Matches(msg, a.keyMap.Help):
		a.status.ToggleFullHelp()
		a.showingFullHelp = !a.showingFullHelp
		return a.handleWindowResize(a.wWidth, a.wHeight)
	// dialogs
	case key.Matches(msg, a.keyMap.Quit):
		if a.dialog.ActiveDialogID() == quit.QuitDialogID {
			// if the quit dialog is already open, close the app
			return tea.Quit
		}
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: quit.NewQuitDialog(),
		})

	case key.Matches(msg, a.keyMap.Commands):
		if a.dialog.ActiveDialogID() == commands.CommandsDialogID {
			// If the commands dialog is already open, close it
			return util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: commands.NewCommandDialog(a.selectedSessionID),
		})
	case key.Matches(msg, a.keyMap.Sessions):
		if a.dialog.ActiveDialogID() == sessions.SessionsDialogID {
			// If the sessions dialog is already open, close it
			return util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		var cmds []tea.Cmd
		if a.dialog.ActiveDialogID() == commands.CommandsDialogID {
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
		return a.moveToPage(logs.LogsPage)

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
		// TODO: maybe remove this :  For now we don't move to any page if the agent is busy
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
	page := a.pages[a.currentPage]
	if withHelp, ok := page.(layout.Help); ok {
		a.keyMap.pageBindings = withHelp.Bindings()
	}
	a.status.SetKeyMap(a.keyMap)
	pageView := page.View()
	components := []string{
		pageView.String(),
	}
	components = append(components, a.status.View().String())

	appView := lipgloss.JoinVertical(lipgloss.Top, components...)
	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(appView),
	}
	if a.dialog.HasDialogs() {
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
	chatPage := chat.NewChatPage(app)
	keyMap := DefaultKeyMap()
	keyMap.pageBindings = chatPage.Bindings()

	model := &appModel{
		currentPage: chat.ChatPageID,
		app:         app,
		status:      status.NewStatusCmp(keyMap),
		loadedPages: make(map[page.PageID]bool),
		keyMap:      keyMap,

		pages: map[page.PageID]util.Model{
			chat.ChatPageID: chatPage,
			logs.LogsPage:   logs.NewLogsPage(),
		},

		dialog:      dialogs.NewDialogCmp(),
		completions: completions.New(),
	}

	return model
}
