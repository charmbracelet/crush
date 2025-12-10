package tui

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/stringext"
	cmpChat "github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/chat/splash"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/core/status"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/filepicker"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/models"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/permissions"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/quit"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/sessions"
	"github.com/charmbracelet/crush/internal/tui/page"
	"github.com/charmbracelet/crush/internal/tui/page/chat"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"golang.org/x/mod/semver"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

	app *app.App

	dialog       dialogs.DialogCmp
	completions  completions.Completions
	isConfigured bool

	// Chat Page Specific
	selectedSessionID string // The ID of the currently selected session

	// sendProgressBar instructs the TUI to send progress bar updates to the
	// terminal.
	sendProgressBar bool

	// QueryVersion instructs the TUI to query for the terminal version when it
	// starts.
	QueryVersion bool
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
	if a.QueryVersion {
		cmds = append(cmds, tea.RequestTerminalVersion)
	}

	return tea.Batch(cmds...)
}

// Update handles incoming messages and updates the application state.
func (a *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	a.isConfigured = config.HasInitialDataConfig()

	switch msg := msg.(type) {
	case tea.EnvMsg:
		// Is this Windows Terminal?
		if !a.sendProgressBar {
			a.sendProgressBar = slices.Contains(msg, "WT_SESSION")
		}
	case tea.TerminalVersionMsg:
		if a.sendProgressBar {
			return a, nil
		}
		termVersion := strings.ToLower(msg.Name)
		switch {
		case stringext.ContainsAny(termVersion, "ghostty", "rio"):
			a.sendProgressBar = true
		case strings.Contains(termVersion, "iterm2"):
			// iTerm2 supports progress bars from version v3.6.6
			matches := regexp.MustCompile(`^iterm2 (\d+\.\d+\.\d+)$`).FindStringSubmatch(termVersion)
			if len(matches) == 2 && semver.Compare("v"+matches[1], "v3.6.6") >= 0 {
				a.sendProgressBar = true
			}
		}
		return a, nil
	case tea.KeyboardEnhancementsMsg:
		// A non-zero value means we have key disambiguation support.
		if msg.Flags > 0 {
			a.keyMap.Models.SetHelp("ctrl+m", "models")
		}
		for id, page := range a.pages {
			m, pageCmd := page.Update(msg)
			a.pages[id] = m

			if pageCmd != nil {
				cmds = append(cmds, pageCmd)
			}
		}
		return a, tea.Batch(cmds...)
	case tea.WindowSizeMsg:
		a.wWidth, a.wHeight = msg.Width, msg.Height
		a.completions.Update(msg)
		return a, a.handleWindowResize(msg.Width, msg.Height)

	case pubsub.Event[mcp.Event]:
		switch msg.Payload.Type {
		case mcp.EventStateChanged:
			return a, a.handleStateChanged(context.Background())
		case mcp.EventPromptsListChanged:
			return a, handleMCPPromptsEvent(context.Background(), msg.Payload.Name)
		case mcp.EventToolsListChanged:
			return a, handleMCPToolsEvent(context.Background(), msg.Payload.Name)
		}

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
		var args []commands.Argument
		for _, arg := range msg.ArgNames {
			args = append(args, commands.Argument{
				Name:     arg,
				Title:    cases.Title(language.English).String(arg),
				Required: true,
			})
		}
		return a, util.CmdHandler(
			dialogs.OpenDialogMsg{
				Model: commands.NewCommandArgumentsDialog(
					msg.CommandID,
					msg.CommandID,
					msg.CommandID,
					msg.Description,
					args,
					msg.OnSubmit,
				),
			},
		)
	case commands.ShowMCPPromptArgumentsDialogMsg:
		args := make([]commands.Argument, 0, len(msg.Prompt.Arguments))
		for _, arg := range msg.Prompt.Arguments {
			args = append(args, commands.Argument(*arg))
		}
		dialog := commands.NewCommandArgumentsDialog(
			msg.Prompt.Name,
			msg.Prompt.Title,
			msg.Prompt.Name,
			msg.Prompt.Description,
			args,
			msg.OnSubmit,
		)
		return a, util.CmdHandler(
			dialogs.OpenDialogMsg{
				Model: dialog,
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
		return a, func() tea.Msg {
			err := a.app.AgentCoordinator.Summarize(context.Background(), msg.SessionID)
			if err != nil {
				return util.ReportError(err)()
			}
			return nil
		}
	case commands.QuitMsg:
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: quit.NewQuitDialog(),
		})
	case commands.ToggleYoloModeMsg:
		a.app.Permissions.SetSkipRequests(!a.app.Permissions.SkipRequests())
	case commands.ToggleHelpMsg:
		a.status.ToggleFullHelp()
		a.showingFullHelp = !a.showingFullHelp
		return a, a.handleWindowResize(a.wWidth, a.wHeight)
	// Model Switch
	case models.ModelSelectedMsg:
		if a.app.AgentCoordinator.IsBusy() {
			return a, util.ReportWarn("Agent is busy, please wait...")
		}

		cfg := config.Get()
		if err := cfg.UpdatePreferredModel(msg.ModelType, msg.Model); err != nil {
			return a, util.ReportError(err)
		}

		go a.app.UpdateAgentModel(context.TODO())

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
			Model: filepicker.NewFilePickerCmp(a.app.Config().WorkingDir()),
		})
	// Permissions
	case pubsub.Event[permission.PermissionNotification]:
		item, ok := a.pages[a.currentPage]
		if !ok {
			return a, nil
		}

		// Forward to view.
		updated, itemCmd := item.Update(msg)
		a.pages[a.currentPage] = updated

		return a, itemCmd
	case pubsub.Event[permission.PermissionRequest]:
		return a, util.CmdHandler(dialogs.OpenDialogMsg{
			Model: permissions.NewPermissionDialogCmp(msg.Payload, &permissions.Options{
				DiffMode: config.Get().Options.TUI.DiffMode,
			}),
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
	case splash.OnboardingCompleteMsg:
		item, ok := a.pages[a.currentPage]
		if !ok {
			return a, nil
		}

		a.isConfigured = config.HasInitialDataConfig()
		updated, pageCmd := item.Update(msg)
		a.pages[a.currentPage] = updated

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
			a.pages[a.currentPage] = updated

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
			a.pages[a.currentPage] = updated

			cmds = append(cmds, pageCmd)
		}
		return a, tea.Batch(cmds...)
	// Update Available
	case pubsub.UpdateAvailableMsg:
		// Show update notification in status bar
		statusMsg := fmt.Sprintf("Crush update available: v%s → v%s.", msg.CurrentVersion, msg.LatestVersion)
		if msg.IsDevelopment {
			statusMsg = fmt.Sprintf("This is a development version of Crush. The latest version is v%s.", msg.LatestVersion)
		}
		s, statusCmd := a.status.Update(util.InfoMsg{
			Type: util.InfoTypeUpdate,
			Msg:  statusMsg,
			TTL:  10 * time.Second,
		})
		a.status = s.(status.StatusCmp)
		return a, statusCmd
	}
	s, _ := a.status.Update(msg)
	a.status = s.(status.StatusCmp)

	item, ok := a.pages[a.currentPage]
	if !ok {
		return a, nil
	}

	updated, cmd := item.Update(msg)
	a.pages[a.currentPage] = updated

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
		a.pages[p] = updated

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
			Model: commands.NewCommandDialog(a.selectedSessionID),
		})
	case key.Matches(msg, a.keyMap.Models):
		// if the app is not configured show no models
		if !a.isConfigured {
			return nil
		}
		if a.dialog.ActiveDialogID() == models.ModelsDialogID {
			return util.CmdHandler(dialogs.CloseDialogMsg{})
		}
		if a.dialog.HasDialogs() {
			return nil
		}
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: models.NewModelDialogCmp(),
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
		cmds = append(cmds,
			func() tea.Msg {
				allSessions, _ := a.app.Sessions.List(context.Background())
				return dialogs.OpenDialogMsg{
					Model: sessions.NewSessionDialogCmp(allSessions, a.selectedSessionID),
				}
			},
		)
		return tea.Sequence(cmds...)
	case key.Matches(msg, a.keyMap.Suspend):
		if a.app.AgentCoordinator != nil && a.app.AgentCoordinator.IsBusy() {
			return util.ReportWarn("Agent is busy, please wait...")
		}
		return tea.Suspend
	default:
		item, ok := a.pages[a.currentPage]
		if !ok {
			return nil
		}

		updated, cmd := item.Update(msg)
		a.pages[a.currentPage] = updated
		return cmd
	}
}

// moveToPage handles navigation between different pages in the application.
func (a *appModel) moveToPage(pageID page.PageID) tea.Cmd {
	if a.app.AgentCoordinator.IsBusy() {
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
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.BackgroundColor = t.BgBase
	if a.wWidth < 25 || a.wHeight < 15 {
		view.Content = t.S().Base.Width(a.wWidth).Height(a.wHeight).
			Align(lipgloss.Center, lipgloss.Center).
			Render(t.S().Base.
				Padding(1, 4).
				Foreground(t.White).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(t.Primary).
				Render("Window too small!"),
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

	comp := lipgloss.NewCompositor(layers...)
	view.Content = comp.Render()
	view.Cursor = cursor

	if a.sendProgressBar && a.app != nil && a.app.AgentCoordinator != nil && a.app.AgentCoordinator.IsBusy() {
		// HACK: use a random percentage to prevent ghostty from hiding it
		// after a timeout.
		view.ProgressBar = tea.NewProgressBar(tea.ProgressBarIndeterminate, rand.Intn(100))
	}
	return view
}

func (a *appModel) handleStateChanged(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		a.app.UpdateAgentModel(ctx)
		return nil
	}
}

func handleMCPPromptsEvent(ctx context.Context, name string) tea.Cmd {
	return func() tea.Msg {
		mcp.RefreshPrompts(ctx, name)
		return nil
	}
}

func handleMCPToolsEvent(ctx context.Context, name string) tea.Cmd {
	return func() tea.Msg {
		mcp.RefreshTools(ctx, name)
		return nil
	}
}

// New creates and initializes a new TUI application model.
func New(app *app.App) *appModel {
	chatPage := chat.New(app)
	keyMap := DefaultKeyMap()
	keyMap.pageBindings = chatPage.Bindings()

	model := &appModel{
		currentPage: chat.ChatPageID,
		app:         app,
		status:      status.NewStatusCmp(),
		loadedPages: make(map[page.PageID]bool),
		keyMap:      keyMap,

		pages: map[page.PageID]util.Model{
			chat.ChatPageID: chatPage,
		},

		dialog:      dialogs.NewDialogCmp(),
		completions: completions.New(),
	}

	return model
}
