package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/llm/agent"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	cmpChat "github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/chat/splash"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/core/status"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/compact"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/filepicker"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/models"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/permissions"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/quit"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/sessions"
	"github.com/charmbracelet/crush/internal/tui/page"
	"github.com/charmbracelet/crush/internal/tui/page/chat"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

var lastMouseEvent time.Time

func MouseEventFilter(m tea.Model, msg tea.Msg) tea.Msg {
	switch msg.(type) {
	case tea.MouseWheelMsg, tea.MouseMotionMsg:
		now := time.Now()
		// trackpad is sending too many requests
		if now.Sub(lastMouseEvent) < 15*time.Millisecond {
			return nil
		}
		lastMouseEvent = now
	}
	return msg
}

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

	c   *client.Client
	ins *proto.Instance

	dialog       dialogs.DialogCmp
	completions  completions.Completions
	isConfigured bool

	// Chat Page Specific
	selectedSessionID string // The ID of the currently selected session
}

// Init initializes the application model and returns initial commands.
func (a appModel) Init() tea.Cmd {
	item, ok := a.pages[a.currentPage]
	if !ok {
		return nil
	}

	var cmds []tea.Cmd
	cmd := item.Init()
	cmds = append(cmds, cmd)
	a.loadedPages[a.currentPage] = true

	cmd = a.status.Init()
	cmds = append(cmds, cmd)

	cmds = append(cmds, tea.EnableMouseAllMotion)

	return tea.Batch(cmds...)
}

// Update handles incoming messages and updates the application state.
func (a *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	a.isConfigured = config.HasInitialDataConfig(a.ins.Config)

	switch msg := msg.(type) {
	case tea.KeyboardEnhancementsMsg:
		for id, page := range a.pages {
			m, pageCmd := page.Update(msg)
			if model, ok := m.(util.Model); ok {
				a.pages[id] = model
			}

			if pageCmd != nil {
				cmds = append(cmds, pageCmd)
			}
		}
		return a, tea.Batch(cmds...)
	case tea.WindowSizeMsg:
		a.wWidth, a.wHeight = msg.Width, msg.Height
		a.completions.Update(msg)
		return a, a.handleWindowResize(msg.Width, msg.Height)

	// Completions messages
	case completions.OpenCompletionsMsg, completions.FilterCompletionsMsg,
		completions.CloseCompletionsMsg, completions.RepositionCompletionsMsg:
		u, completionCmd := a.completions.Update(msg)
		if model, ok := u.(completions.Completions); ok {
			a.completions = model
		}

		return a, completionCmd

	// Dialog messages
	case dialogs.OpenDialogMsg, dialogs.CloseDialogMsg:
		u, completionCmd := a.completions.Update(completions.CloseCompletionsMsg{})
		a.completions = u.(completions.Completions)
		u, dialogCmd := a.dialog.Update(msg)
		a.dialog = u.(dialogs.DialogCmp)
		return a, tea.Batch(completionCmd, dialogCmd)
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
	// Commands
	case commands.SwitchSessionsMsg:
		return a, func() tea.Msg {
			allSessions, _ := a.c.ListSessions(context.Background(), a.ins.ID)
			return dialogs.OpenDialogMsg{
				Model: sessions.NewSessionDialogCmp(allSessions, a.selectedSessionID),
			}
		}

	case commands.SwitchModelMsg:
		return a, util.CmdHandler(
			dialogs.OpenDialogMsg{
				Model: models.NewModelDialogCmp(a.ins),
			},
		)
	// Compact
	case commands.CompactMsg:
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: compact.NewCompactDialogCmp(a.c, a.ins, msg.SessionID, true),
		})
	case commands.QuitMsg:
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: quit.NewQuitDialog(),
		})
	case commands.ToggleYoloModeMsg:
		skip, err := a.c.GetPermissionsSkipRequests(context.TODO(), a.ins.ID)
		if err != nil {
			return a, util.ReportError(fmt.Errorf("failed to get permissions skip requests: %v", err))
		}
		if err := a.c.SetPermissionsSkipRequests(context.TODO(), a.ins.ID, !skip); err != nil {
			return a, util.ReportError(fmt.Errorf("failed to toggle YOLO mode: %v", err))
		}
	case commands.ToggleHelpMsg:
		a.status.ToggleFullHelp()
		a.showingFullHelp = !a.showingFullHelp
		return a, a.handleWindowResize(a.wWidth, a.wHeight)
	// Model Switch
	case models.ModelSelectedMsg:
		info, err := a.c.GetAgentInfo(context.TODO(), a.ins.ID)
		if err != nil {
			return a, util.ReportError(fmt.Errorf("failed to check if agent is busy: %v", err))
		}
		if info.IsBusy {
			return a, util.ReportWarn("Agent is busy, please wait...")
		}
		a.ins.Config.UpdatePreferredModel(msg.ModelType, msg.Model)

		// Update the agent with the new model/provider configuration
		if err := a.c.UpdateAgent(context.TODO(), a.ins.ID); err != nil {
			return a, util.ReportError(fmt.Errorf("model changed to %s but failed to update agent: %v", msg.Model.Model, err))
		}

		modelTypeName := "large"
		if msg.ModelType == config.SelectedModelTypeSmall {
			modelTypeName = "small"
		}
		return a, util.ReportInfo(fmt.Sprintf("%s model changed to %s", modelTypeName, msg.Model.Model))

	// File Picker
	case commands.OpenFilePickerMsg:
		event.FilePickerOpened()

		if a.dialog.ActiveDialogID() == filepicker.FilePickerID {
			// If the commands dialog is already open, close it
			return a, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: filepicker.NewFilePickerCmp(a.ins.Config.WorkingDir()),
		})
	// Permissions
	case pubsub.Event[permission.PermissionNotification]:
		item, ok := a.pages[a.currentPage]
		if !ok {
			return a, nil
		}

		// Forward to view.
		updated, itemCmd := item.Update(msg)
		if model, ok := updated.(util.Model); ok {
			a.pages[a.currentPage] = model
		}

		return a, itemCmd
	case pubsub.Event[permission.PermissionRequest]:
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: permissions.NewPermissionDialogCmp(msg.Payload, &permissions.Options{
				DiffMode: a.ins.Config.Options.TUI.DiffMode,
			}),
		})
	case permissions.PermissionResponseMsg:
		if err := a.c.GrantPermission(context.TODO(), a.ins.ID, proto.PermissionGrant(msg)); err != nil {
			return a, util.ReportError(fmt.Errorf("failed to grant permission: %v", err))
		}
		return a, nil
	// Agent Events
	case pubsub.Event[agent.AgentEvent]:
		payload := msg.Payload

		if payload.Type == agent.AgentEventTypeError {
			return a, util.ReportError(fmt.Errorf("agent error: %v", payload.Error))
		}

		// Forward agent events to dialogs
		if a.dialog.HasDialogs() && a.dialog.ActiveDialogID() == compact.CompactDialogID {
			u, dialogCmd := a.dialog.Update(payload)
			if model, ok := u.(dialogs.DialogCmp); ok {
				a.dialog = model
			}

			cmds = append(cmds, dialogCmd)
		}

		// Handle auto-compact logic
		if payload.Done && payload.Type == agent.AgentEventTypeResponse && a.selectedSessionID != "" {
			// Get current session to check token usage
			session, err := a.c.GetSession(context.Background(), a.ins.ID, a.selectedSessionID)
			if err == nil {
				info, err := a.c.GetAgentInfo(context.Background(), a.ins.ID)
				if err != nil {
					return a, util.ReportError(fmt.Errorf("failed to check if agent is busy: %v", err))
				}
				model := info.Model
				contextWindow := model.ContextWindow
				tokens := session.CompletionTokens + session.PromptTokens
				if (tokens >= int64(float64(contextWindow)*0.95)) && !a.ins.Config.Options.DisableAutoSummarize { // Show compact confirmation dialog
					cmds = append(cmds, util.CmdHandler(dialogs.OpenDialogMsg{
						Model: compact.NewCompactDialogCmp(a.c, a.ins, a.selectedSessionID, false),
					}))
				}
			}
		}

		return a, tea.Batch(cmds...)
	case splash.OnboardingCompleteMsg:
		item, ok := a.pages[a.currentPage]
		if !ok {
			return a, nil
		}

		a.isConfigured = config.HasInitialDataConfig(a.ins.Config)
		updated, pageCmd := item.Update(msg)
		if model, ok := updated.(util.Model); ok {
			a.pages[a.currentPage] = model
		}

		cmds = append(cmds, pageCmd)
		return a, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		return a, a.handleKeyPressMsg(msg)

	case tea.MouseWheelMsg:
		if a.dialog.HasDialogs() {
			u, dialogCmd := a.dialog.Update(msg)
			a.dialog = u.(dialogs.DialogCmp)
			cmds = append(cmds, dialogCmd)
		} else {
			item, ok := a.pages[a.currentPage]
			if !ok {
				return a, nil
			}

			updated, pageCmd := item.Update(msg)
			if model, ok := updated.(util.Model); ok {
				a.pages[a.currentPage] = model
			}

			cmds = append(cmds, pageCmd)
		}
		return a, tea.Batch(cmds...)
	case tea.PasteMsg:
		if a.dialog.HasDialogs() {
			u, dialogCmd := a.dialog.Update(msg)
			if model, ok := u.(dialogs.DialogCmp); ok {
				a.dialog = model
			}

			cmds = append(cmds, dialogCmd)
		} else {
			item, ok := a.pages[a.currentPage]
			if !ok {
				return a, nil
			}

			updated, pageCmd := item.Update(msg)
			if model, ok := updated.(util.Model); ok {
				a.pages[a.currentPage] = model
			}

			cmds = append(cmds, pageCmd)
		}
		return a, tea.Batch(cmds...)
	}
	s, _ := a.status.Update(msg)
	a.status = s.(status.StatusCmp)

	item, ok := a.pages[a.currentPage]
	if !ok {
		return a, nil
	}

	updated, cmd := item.Update(msg)
	if model, ok := updated.(util.Model); ok {
		a.pages[a.currentPage] = model
	}

	if a.dialog.HasDialogs() {
		u, dialogCmd := a.dialog.Update(msg)
		if model, ok := u.(dialogs.DialogCmp); ok {
			a.dialog = model
		}

		cmds = append(cmds, dialogCmd)
	}
	cmds = append(cmds, cmd)
	return a, tea.Batch(cmds...)
}

// handleWindowResize processes window resize events and updates all components.
func (a *appModel) handleWindowResize(width, height int) tea.Cmd {
	var cmds []tea.Cmd

	// TODO: clean up these magic numbers.
	if a.showingFullHelp {
		height -= 5
	} else {
		height -= 2
	}

	a.width, a.height = width, height
	// Update status bar
	s, cmd := a.status.Update(tea.WindowSizeMsg{Width: width, Height: height})
	if model, ok := s.(status.StatusCmp); ok {
		a.status = model
	}
	cmds = append(cmds, cmd)

	// Update the current view.
	for p, page := range a.pages {
		updated, pageCmd := page.Update(tea.WindowSizeMsg{Width: width, Height: height})
		if model, ok := updated.(util.Model); ok {
			a.pages[p] = model
		}

		cmds = append(cmds, pageCmd)
	}

	// Update the dialogs
	dialog, cmd := a.dialog.Update(tea.WindowSizeMsg{Width: width, Height: height})
	if model, ok := dialog.(dialogs.DialogCmp); ok {
		a.dialog = model
	}

	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

// handleKeyPressMsg processes keyboard input and routes to appropriate handlers.
func (a *appModel) handleKeyPressMsg(msg tea.KeyPressMsg) tea.Cmd {
	// Check this first as the user should be able to quit no matter what.
	if key.Matches(msg, a.keyMap.Quit) {
		if a.dialog.ActiveDialogID() == quit.QuitDialogID {
			return tea.Quit
		}
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: quit.NewQuitDialog(),
		})
	}

	if a.completions.Open() {
		// completions
		keyMap := a.completions.KeyMap()
		switch {
		case key.Matches(msg, keyMap.Up), key.Matches(msg, keyMap.Down),
			key.Matches(msg, keyMap.Select), key.Matches(msg, keyMap.Cancel),
			key.Matches(msg, keyMap.UpInsert), key.Matches(msg, keyMap.DownInsert):
			u, cmd := a.completions.Update(msg)
			a.completions = u.(completions.Completions)
			return cmd
		}
	}
	if a.dialog.HasDialogs() {
		u, dialogCmd := a.dialog.Update(msg)
		a.dialog = u.(dialogs.DialogCmp)
		return dialogCmd
	}
	switch {
	// help
	case key.Matches(msg, a.keyMap.Help):
		a.status.ToggleFullHelp()
		a.showingFullHelp = !a.showingFullHelp
		return a.handleWindowResize(a.wWidth, a.wHeight)
	// dialogs
	case key.Matches(msg, a.keyMap.Commands):
		// if the app is not configured show no commands
		if !a.isConfigured {
			return nil
		}
		if a.dialog.ActiveDialogID() == commands.CommandsDialogID {
			return util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		if a.dialog.HasDialogs() {
			return nil
		}
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: commands.NewCommandDialog(a.ins.Config, a.selectedSessionID),
		})
	case key.Matches(msg, a.keyMap.Sessions):
		// if the app is not configured show no sessions
		if !a.isConfigured {
			return nil
		}
		if a.dialog.ActiveDialogID() == sessions.SessionsDialogID {
			return util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		if a.dialog.HasDialogs() && a.dialog.ActiveDialogID() != commands.CommandsDialogID {
			return nil
		}
		var cmds []tea.Cmd
		if a.dialog.ActiveDialogID() == commands.CommandsDialogID {
			// If the commands dialog is open, close it first
			cmds = append(cmds, util.CmdHandler(dialogs.CloseDialogMsg{}))
		}
		cmds = append(cmds,
			func() tea.Msg {
				allSessions, _ := a.c.ListSessions(context.Background(), a.ins.ID)
				return dialogs.OpenDialogMsg{
					Model: sessions.NewSessionDialogCmp(allSessions, a.selectedSessionID),
				}
			},
		)
		return tea.Sequence(cmds...)
	case key.Matches(msg, a.keyMap.Suspend):
		info, err := a.c.GetAgentInfo(context.TODO(), a.ins.ID)
		if err != nil || info.IsBusy {
			return util.ReportWarn("Agent is busy, please wait...")
		}
		return tea.Suspend
	default:
		item, ok := a.pages[a.currentPage]
		if !ok {
			return nil
		}

		updated, cmd := item.Update(msg)
		if model, ok := updated.(util.Model); ok {
			a.pages[a.currentPage] = model
		}
		return cmd
	}
}

// moveToPage handles navigation between different pages in the application.
func (a *appModel) moveToPage(pageID page.PageID) tea.Cmd {
	info, err := a.c.GetAgentInfo(context.TODO(), a.ins.ID)
	if err != nil {
		return util.ReportError(fmt.Errorf("failed to check if agent is busy: %v", err))
	}
	if info.IsBusy {
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
	var view tea.View
	t := styles.CurrentTheme()
	view.BackgroundColor = t.BgBase
	if a.wWidth < 25 || a.wHeight < 15 {
		view.Layer = lipgloss.NewCanvas(
			lipgloss.NewLayer(
				t.S().Base.Width(a.wWidth).Height(a.wHeight).
					Align(lipgloss.Center, lipgloss.Center).
					Render(
						t.S().Base.
							Padding(1, 4).
							Foreground(t.White).
							BorderStyle(lipgloss.RoundedBorder()).
							BorderForeground(t.Primary).
							Render("Window too small!"),
					),
			),
		)
		return view
	}

	page := a.pages[a.currentPage]
	if withHelp, ok := page.(core.KeyMapHelp); ok {
		a.status.SetKeyMap(withHelp.Help())
	}
	pageView := page.View()
	components := []string{
		pageView,
	}
	components = append(components, a.status.View())

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

	var cursor *tea.Cursor
	if v, ok := page.(util.Cursor); ok {
		cursor = v.Cursor()
		// Hide the cursor if it's positioned outside the textarea
		statusHeight := a.height - strings.Count(pageView, "\n") + 1
		if cursor != nil && cursor.Y+statusHeight+chat.EditorHeight-2 <= a.height { // 2 for the top and bottom app padding
			cursor = nil
		}
	}
	activeView := a.dialog.ActiveModel()
	if activeView != nil {
		cursor = nil // Reset cursor if a dialog is active unless it implements util.Cursor
		if v, ok := activeView.(util.Cursor); ok {
			cursor = v.Cursor()
		}
	}

	if a.completions.Open() && cursor != nil {
		cmp := a.completions.View()
		x, y := a.completions.Position()
		layers = append(
			layers,
			lipgloss.NewLayer(cmp).X(x).Y(y),
		)
	}

	canvas := lipgloss.NewCanvas(
		layers...,
	)

	view.Layer = canvas
	view.Cursor = cursor
	return view
}

// New creates and initializes a new TUI application model.
func New(c *client.Client, ins *proto.Instance) (tea.Model, error) {
	// Setup logs
	log.Setup(
		filepath.Join(ins.Config.Options.DataDirectory, "logs", "tui.log"),
		ins.Config.Options.Debug,
	)

	chatPage := chat.New(c, ins)
	keyMap := DefaultKeyMap()
	keyMap.pageBindings = chatPage.Bindings()
	model := &appModel{
		ins:         ins,
		currentPage: chat.ChatPageID,
		c:           c,
		status:      status.NewStatusCmp(),
		loadedPages: make(map[page.PageID]bool),
		keyMap:      keyMap,

		pages: map[page.PageID]util.Model{
			chat.ChatPageID: chatPage,
		},

		dialog:      dialogs.NewDialogCmp(),
		completions: completions.New(),
	}

	return model, nil
}
