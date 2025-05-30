package tui

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/tui/components/completions"
	"github.com/opencode-ai/opencode/internal/tui/components/core"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/commands"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/quit"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/page"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type appModel struct {
	width, height int
	keyMap        KeyMap

	currentPage  page.PageID
	previousPage page.PageID
	pages        map[page.PageID]util.Model
	loadedPages  map[page.PageID]bool

	status core.StatusCmp

	app *app.App

	dialog      dialogs.DialogCmp
	completions completions.Completions
}

func (a appModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmd := a.pages[a.currentPage].Init()
	cmds = append(cmds, cmd)
	a.loadedPages[a.currentPage] = true

	cmd = a.status.Init()
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

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
		a.status = s.(core.StatusCmp)
		cmds = append(cmds, statusCmd)
		return a, tea.Batch(cmds...)

	// Logs
	case pubsub.Event[logging.LogMessage]:
		// Send to the status component
		s, statusCmd := a.status.Update(msg)
		a.status = s.(core.StatusCmp)
		cmds = append(cmds, statusCmd)

		// If the current page is logs, update the logs view
		if a.currentPage == page.LogsPage {
			updated, pageCmd := a.pages[a.currentPage].Update(msg)
			a.pages[a.currentPage] = updated.(util.Model)
			cmds = append(cmds, pageCmd)
		}
		return a, tea.Batch(cmds...)
	case tea.KeyPressMsg:
		return a, a.handleKeyPressMsg(msg)
	}
	s, _ := a.status.Update(msg)
	a.status = s.(core.StatusCmp)
	updated, cmd := a.pages[a.currentPage].Update(msg)
	a.pages[a.currentPage] = updated.(util.Model)
	cmds = append(cmds, cmd)
	return a, tea.Batch(cmds...)
}

func (a *appModel) handleWindowResize(msg tea.WindowSizeMsg) tea.Cmd {
	var cmds []tea.Cmd
	msg.Height -= 1 // Make space for the status bar
	a.width, a.height = msg.Width, msg.Height

	// Update status bar
	s, cmd := a.status.Update(msg)
	a.status = s.(core.StatusCmp)
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
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: quit.NewQuitDialog(),
		})

	case key.Matches(msg, a.keyMap.Commands):
		return util.CmdHandler(dialogs.OpenDialogMsg{
			Model: commands.NewCommandDialog(),
		})

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

// RegisterCommand adds a command to the command dialog
// func (a *appModel) RegisterCommand(cmd dialog.Command) {
// 	a.commands = append(a.commands, cmd)
// }

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
	t := theme.CurrentTheme()
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
	view := tea.NewView(canvas.Render())
	view.SetBackgroundColor(t.Background())
	view.SetCursor(cursor)
	return view
}

func New(app *app.App) tea.Model {
	startPage := page.ChatPage
	model := &appModel{
		currentPage: startPage,
		app:         app,
		status:      core.NewStatusCmp(app.LSPClients),
		loadedPages: make(map[page.PageID]bool),
		keyMap:      DefaultKeyMap(),

		pages: map[page.PageID]util.Model{
			page.ChatPage: page.NewChatPage(app),
			page.LogsPage: page.NewLogsPage(),
		},

		dialog:      dialogs.NewDialogCmp(),
		completions: completions.New(),
	}

	return model
}
