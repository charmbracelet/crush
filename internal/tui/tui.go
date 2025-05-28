package tui

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/pubsub"
	"github.com/opencode-ai/opencode/internal/tui/components/core"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/commands"
	"github.com/opencode-ai/opencode/internal/tui/components/dialogs/quit"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/page"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// type startCompactSessionMsg struct{}

type appModel struct {
	width, height int
	keyMap        KeyMap

	currentPage  page.PageID
	previousPage page.PageID
	pages        map[page.PageID]util.Model
	loadedPages  map[page.PageID]bool

	status core.StatusCmp

	app *app.App

	// selectedSession session.Session
	//
	// showPermissions bool
	// permissions     dialog.PermissionDialogCmp
	//
	// showHelp bool
	// help     dialog.HelpCmp
	//
	// showSessionDialog bool
	// sessionDialog     dialog.SessionDialog
	//
	// showCommandDialog bool
	// commandDialog     dialog.CommandDialog
	// commands          []dialog.Command
	//
	// showModelDialog bool
	// modelDialog     dialog.ModelDialog
	//
	// showInitDialog bool
	// initDialog     dialog.InitDialogCmp
	//
	// showFilepicker bool
	// filepicker     dialog.FilepickerCmp
	//
	// showThemeDialog bool
	// themeDialog     dialog.ThemeDialog
	//
	// showMultiArgumentsDialog bool
	// multiArgumentsDialog     dialog.MultiArgumentsDialogCmp
	//
	// isCompacting      bool
	// compactingMessage string

	// NEW DIALOG
	dialog dialogs.DialogCmp
}

func (a appModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmd := a.pages[a.currentPage].Init()
	cmds = append(cmds, cmd)
	a.loadedPages[a.currentPage] = true

	cmd = a.status.Init()
	cmds = append(cmds, cmd)
	// cmd = a.help.Init()
	// cmds = append(cmds, cmd)
	// cmd = a.sessionDialog.Init()
	// cmds = append(cmds, cmd)
	// cmd = a.commandDialog.Init()
	// cmds = append(cmds, cmd)
	// cmd = a.modelDialog.Init()
	// cmds = append(cmds, cmd)
	// cmd = a.initDialog.Init()
	// cmds = append(cmds, cmd)
	// cmd = a.filepicker.Init()
	// cmds = append(cmds, cmd)
	// cmd = a.themeDialog.Init()
	// cmds = append(cmds, cmd)

	// Check if we should show the init dialog
	// cmds = append(cmds, func() tea.Msg {
	// 	shouldShow, err := config.ShouldShowInitDialog()
	// 	if err != nil {
	// 		return util.InfoMsg{
	// 			Type: util.InfoTypeError,
	// 			Msg:  "Failed to check init status: " + err.Error(),
	// 		}
	// 	}
	// 	return dialog.ShowInitDialogMsg{Show: shouldShow}
	// })
	return tea.Batch(cmds...)
}

func (a *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return a, a.handleWindowResize(msg)
	// TODO: remove when refactor is done
	// msg.Height -= 1 // Make space for the status bar
	// a.width, a.height = msg.Width, msg.Height
	//
	// s, _ := a.status.Update(msg)
	// a.status = s.(core.StatusCmp)
	// updated, cmd := a.pages[a.currentPage].Update(msg)
	// a.pages[a.currentPage] = updated.(util.Model)
	// cmds = append(cmds, cmd)
	//
	// prm, permCmd := a.permissions.Update(msg)
	// a.permissions = prm.(dialog.PermissionDialogCmp)
	// cmds = append(cmds, permCmd)
	//
	// help, helpCmd := a.help.Update(msg)
	// a.help = help.(dialog.HelpCmp)
	// cmds = append(cmds, helpCmd)
	//
	// session, sessionCmd := a.sessionDialog.Update(msg)
	// a.sessionDialog = session.(dialog.SessionDialog)
	// cmds = append(cmds, sessionCmd)
	//
	// command, commandCmd := a.commandDialog.Update(msg)
	// a.commandDialog = command.(dialog.CommandDialog)
	// cmds = append(cmds, commandCmd)
	//
	// filepicker, filepickerCmd := a.filepicker.Update(msg)
	// a.filepicker = filepicker.(dialog.FilepickerCmp)
	// cmds = append(cmds, filepickerCmd)
	//
	// a.initDialog.SetSize(msg.Width, msg.Height)
	//
	// if a.showMultiArgumentsDialog {
	// 	a.multiArgumentsDialog.SetSize(msg.Width, msg.Height)
	// 	args, argsCmd := a.multiArgumentsDialog.Update(msg)
	// 	a.multiArgumentsDialog = args.(dialog.MultiArgumentsDialogCmp)
	// 	cmds = append(cmds, argsCmd, a.multiArgumentsDialog.Init())
	// }
	//
	// dialog, cmd := a.dialog.Update(msg)
	// a.dialog = dialog.(dialogs.DialogCmp)
	// cmds = append(cmds, cmd)
	//
	// return a, tea.Batch(cmds...)

	// Dialog messages
	case dialogs.OpenDialogMsg, dialogs.CloseDialogMsg:
		u, dialogCmd := a.dialog.Update(msg)
		a.dialog = u.(dialogs.DialogCmp)
		return a, dialogCmd

	// Page change messages
	case page.PageChangeMsg:
		return a, a.moveToPage(msg.ID)

	// Status Messages
	case util.InfoMsg, util.ClearStatusMsg:
		s, cmd := a.status.Update(msg)
		a.status = s.(core.StatusCmp)
		cmds = append(cmds, cmd)
		return a, tea.Batch(cmds...)
	// Logs
	case pubsub.Event[logging.LogMessage]:
		// Send to the status component
		s, cmd := a.status.Update(msg)
		a.status = s.(core.StatusCmp)
		cmds = append(cmds, cmd)

		// If the current page is logs, update the logs view
		if a.currentPage == page.LogsPage {
			updated, cmd := a.pages[a.currentPage].Update(msg)
			a.pages[a.currentPage] = updated.(util.Model)
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)

	// // Permission
	// case pubsub.Event[permission.PermissionRequest]:
	// 	a.showPermissions = true
	// 	return a, a.permissions.SetPermissions(msg.Payload)
	// case dialog.PermissionResponseMsg:
	// 	var cmd tea.Cmd
	// 	switch msg.Action {
	// 	case dialog.PermissionAllow:
	// 		a.app.Permissions.Grant(msg.Permission)
	// 	case dialog.PermissionAllowForSession:
	// 		a.app.Permissions.GrantPersistant(msg.Permission)
	// 	case dialog.PermissionDeny:
	// 		a.app.Permissions.Deny(msg.Permission)
	// 	}
	// 	a.showPermissions = false
	// 	return a, cmd
	//
	// 	// Theme changed
	// case dialog.ThemeChangedMsg:
	// 	updated, cmd := a.pages[a.currentPage].Update(msg)
	// 	a.pages[a.currentPage] = updated.(util.Model)
	// 	a.showThemeDialog = false
	// 	return a, tea.Batch(cmd, util.ReportInfo("Theme changed to: "+msg.ThemeName))
	//
	// case dialog.CloseSessionDialogMsg:
	// 	a.showSessionDialog = false
	// 	return a, nil
	//
	// case dialog.CloseCommandDialogMsg:
	// 	a.showCommandDialog = false
	// 	return a, nil
	//
	// case startCompactSessionMsg:
	// 	// Start compacting the current session
	// 	a.isCompacting = true
	// 	a.compactingMessage = "Starting summarization..."
	//
	// 	if a.selectedSession.ID == "" {
	// 		a.isCompacting = false
	// 		return a, util.ReportWarn("No active session to summarize")
	// 	}
	//
	// 	// Start the summarization process
	// 	return a, func() tea.Msg {
	// 		ctx := context.Background()
	// 		a.app.CoderAgent.Summarize(ctx, a.selectedSession.ID)
	// 		return nil
	// 	}
	//
	// case pubsub.Event[agent.AgentEvent]:
	// 	payload := msg.Payload
	// 	if payload.Error != nil {
	// 		a.isCompacting = false
	// 		return a, util.ReportError(payload.Error)
	// 	}
	//
	// 	a.compactingMessage = payload.Progress
	//
	// 	if payload.Done && payload.Type == agent.AgentEventTypeSummarize {
	// 		a.isCompacting = false
	// 		return a, util.ReportInfo("Session summarization complete")
	// 	} else if payload.Done && payload.Type == agent.AgentEventTypeResponse && a.selectedSession.ID != "" {
	// 		model := a.app.CoderAgent.Model()
	// 		contextWindow := model.ContextWindow
	// 		tokens := a.selectedSession.CompletionTokens + a.selectedSession.PromptTokens
	// 		if (tokens >= int64(float64(contextWindow)*0.95)) && config.Get().AutoCompact {
	// 			return a, util.CmdHandler(startCompactSessionMsg{})
	// 		}
	// 	}
	// 	// Continue listening for events
	// 	return a, nil
	//
	// case dialog.CloseThemeDialogMsg:
	// 	a.showThemeDialog = false
	// 	return a, nil
	//
	// case dialog.CloseModelDialogMsg:
	// 	a.showModelDialog = false
	// 	return a, nil
	//
	// case dialog.ModelSelectedMsg:
	// 	a.showModelDialog = false
	//
	// 	model, err := a.app.CoderAgent.Update(config.AgentCoder, msg.Model.ID)
	// 	if err != nil {
	// 		return a, util.ReportError(err)
	// 	}
	//
	// 	return a, util.ReportInfo(fmt.Sprintf("Model changed to %s", model.Name))
	//
	// case dialog.ShowInitDialogMsg:
	// 	a.showInitDialog = msg.Show
	// 	return a, nil
	//
	// case dialog.CloseInitDialogMsg:
	// 	a.showInitDialog = false
	// 	if msg.Initialize {
	// 		// Run the initialization command
	// 		for _, cmd := range a.commands {
	// 			if cmd.ID == "init" {
	// 				// Mark the project as initialized
	// 				if err := config.MarkProjectInitialized(); err != nil {
	// 					return a, util.ReportError(err)
	// 				}
	// 				return a, cmd.Handler(cmd)
	// 			}
	// 		}
	// 	} else {
	// 		// Mark the project as initialized without running the command
	// 		if err := config.MarkProjectInitialized(); err != nil {
	// 			return a, util.ReportError(err)
	// 		}
	// 	}
	// 	return a, nil
	//
	// case chat.SessionSelectedMsg:
	// 	a.selectedSession = msg
	// 	a.sessionDialog.SetSelectedSession(msg.ID)
	//
	// case pubsub.Event[session.Session]:
	// 	if msg.Type == pubsub.UpdatedEvent && msg.Payload.ID == a.selectedSession.ID {
	// 		a.selectedSession = msg.Payload
	// 	}
	// case dialog.SessionSelectedMsg:
	// 	a.showSessionDialog = false
	// 	if a.currentPage == page.ChatPage {
	// 		return a, util.CmdHandler(chat.SessionSelectedMsg(msg.Session))
	// 	}
	// 	return a, nil
	//
	// case dialog.CommandSelectedMsg:
	// 	a.showCommandDialog = false
	// 	// Execute the command handler if available
	// 	if msg.Command.Handler != nil {
	// 		return a, msg.Command.Handler(msg.Command)
	// 	}
	// 	return a, util.ReportInfo("Command selected: " + msg.Command.Title)
	//
	// case dialog.ShowMultiArgumentsDialogMsg:
	// 	// Show multi-arguments dialog
	// 	a.multiArgumentsDialog = dialog.NewMultiArgumentsDialogCmp(msg.CommandID, msg.Content, msg.ArgNames)
	// 	a.showMultiArgumentsDialog = true
	// 	return a, a.multiArgumentsDialog.Init()
	//
	// case dialog.CloseMultiArgumentsDialogMsg:
	// 	// Close multi-arguments dialog
	// 	a.showMultiArgumentsDialog = false
	//
	// 	// If submitted, replace all named arguments and run the command
	// 	if msg.Submit {
	// 		content := msg.Content
	//
	// 		// Replace each named argument with its value
	// 		for name, value := range msg.Args {
	// 			placeholder := "$" + name
	// 			content = strings.ReplaceAll(content, placeholder, value)
	// 		}
	//
	// 		// Execute the command with arguments
	// 		return a, util.CmdHandler(dialog.CommandRunCustomMsg{
	// 			Content: content,
	// 			Args:    msg.Args,
	// 		})
	// 	}
	// 	return a, nil
	//
	case tea.KeyPressMsg:
		return a, a.handleKeyPressMsg(msg)
		// if a.dialog.HasDialogs() {
		// 	u, dialogCmd := a.dialog.Update(msg)
		// 	a.dialog = u.(dialogs.DialogCmp)
		// 	return a, dialogCmd
		// }
		// // If multi-arguments dialog is open, let it handle the key press first
		// if a.showMultiArgumentsDialog {
		// 	args, cmd := a.multiArgumentsDialog.Update(msg)
		// 	a.multiArgumentsDialog = args.(dialog.MultiArgumentsDialogCmp)
		// 	return a, cmd
		// }
		//
		// switch {
		// case key.Matches(msg, keys.Quit):
		// 	// TODO: fix this after testing
		// 	// a.showQuit = !a.showQuit
		// 	// if a.showHelp {
		// 	// 	a.showHelp = false
		// 	// }
		// 	// if a.showSessionDialog {
		// 	// 	a.showSessionDialog = false
		// 	// }
		// 	// if a.showCommandDialog {
		// 	// 	a.showCommandDialog = false
		// 	// }
		// 	// if a.showFilepicker {
		// 	// 	a.showFilepicker = false
		// 	// 	a.filepicker.ToggleFilepicker(a.showFilepicker)
		// 	// }
		// 	// if a.showModelDialog {
		// 	// 	a.showModelDialog = false
		// 	// }
		// 	// if a.showMultiArgumentsDialog {
		// 	// 	a.showMultiArgumentsDialog = false
		// 	// }
		// 	return a, util.CmdHandler(dialogs.OpenDialogMsg{
		// 		Model: quit.NewQuitDialog(),
		// 	})
		// case key.Matches(msg, keys.SwitchSession):
		// 	if a.currentPage == page.ChatPage && !a.showPermissions && !a.showCommandDialog {
		// 		// Load sessions and show the dialog
		// 		sessions, err := a.app.Sessions.List(context.Background())
		// 		if err != nil {
		// 			return a, util.ReportError(err)
		// 		}
		// 		if len(sessions) == 0 {
		// 			return a, util.ReportWarn("No sessions available")
		// 		}
		// 		a.sessionDialog.SetSessions(sessions)
		// 		a.showSessionDialog = true
		// 		return a, nil
		// 	}
		// 	return a, nil
		// case key.Matches(msg, keys.Commands):
		// if a.currentPage == page.ChatPage && !a.showPermissions && !a.showSessionDialog && !a.showThemeDialog && !a.showFilepicker {
		// 	// Show commands dialog
		// 	if len(a.commands) == 0 {
		// 		return a, util.ReportWarn("No commands available")
		// 	}
		// 	a.commandDialog.SetCommands(a.commands)
		// 	a.showCommandDialog = true
		// 	return a, nil
		// }
		// 	return a, util.CmdHandler(dialogs.OpenDialogMsg{
		// 		Model: commands.NewCommandDialog(),
		// 	})
		// case key.Matches(msg, keys.Models):
		// 	if a.showModelDialog {
		// 		a.showModelDialog = false
		// 		return a, nil
		// 	}
		// 	if a.currentPage == page.ChatPage && !a.showPermissions && !a.showSessionDialog && !a.showCommandDialog {
		// 		a.showModelDialog = true
		// 		return a, nil
		// 	}
		// 	return a, nil
		// case key.Matches(msg, keys.SwitchTheme):
		// 	if !a.showPermissions && !a.showSessionDialog && !a.showCommandDialog {
		// 		// Show theme switcher dialog
		// 		a.showThemeDialog = true
		// 		// Theme list is dynamically loaded by the dialog component
		// 		return a, a.themeDialog.Init()
		// 	}
		// 	return a, nil
		// case key.Matches(msg, returnKey) || key.Matches(msg):
		// 	if msg.String() == quitKey {
		// 		if a.currentPage == page.LogsPage {
		// 			return a, a.moveToPage(page.ChatPage)
		// 		}
		// 	} else if !a.filepicker.IsCWDFocused() {
		// 		if a.showHelp {
		// 			a.showHelp = !a.showHelp
		// 			return a, nil
		// 		}
		// 		if a.showInitDialog {
		// 			a.showInitDialog = false
		// 			// Mark the project as initialized without running the command
		// 			if err := config.MarkProjectInitialized(); err != nil {
		// 				return a, util.ReportError(err)
		// 			}
		// 			return a, nil
		// 		}
		// 		if a.showFilepicker {
		// 			a.showFilepicker = false
		// 			a.filepicker.ToggleFilepicker(a.showFilepicker)
		// 			return a, nil
		// 		}
		// 		if a.currentPage == page.LogsPage {
		// 			return a, a.moveToPage(page.ChatPage)
		// 		}
		// 	}
		// case key.Matches(msg, keys.Logs):
		// 	return a, a.moveToPage(page.LogsPage)
		// case key.Matches(msg, keys.Help):
		// 	a.showHelp = !a.showHelp
		// 	return a, nil
		// case key.Matches(msg, helpEsc):
		// 	if a.app.CoderAgent.IsBusy() {
		// 		a.showHelp = !a.showHelp
		// 		return a, nil
		// 	}
		// case key.Matches(msg, keys.Filepicker):
		// 	a.showFilepicker = !a.showFilepicker
		// 	a.filepicker.ToggleFilepicker(a.showFilepicker)
		// 	return a, nil
		// }
		// default:
		// 	u, dialogCmd := a.dialog.Update(msg)
		// 	a.dialog = u.(dialogs.DialogCmp)
		// 	cmds = append(cmds, dialogCmd)
		// f, filepickerCmd := a.filepicker.Update(msg)
		// a.filepicker = f.(dialog.FilepickerCmp)
		// cmds = append(cmds, filepickerCmd)
		// }

		// if a.showFilepicker {
		// 	f, filepickerCmd := a.filepicker.Update(msg)
		// 	a.filepicker = f.(dialog.FilepickerCmp)
		// 	cmds = append(cmds, filepickerCmd)
		// 	// Only block key messages send all other messages down
		// 	if _, ok := msg.(tea.KeyPressMsg); ok {
		// 		return a, tea.Batch(cmds...)
		// 	}
		// }
		//
		// if a.showPermissions {
		// 	d, permissionsCmd := a.permissions.Update(msg)
		// 	a.permissions = d.(dialog.PermissionDialogCmp)
		// 	cmds = append(cmds, permissionsCmd)
		// 	// Only block key messages send all other messages down
		// 	if _, ok := msg.(tea.KeyPressMsg); ok {
		// 		return a, tea.Batch(cmds...)
		// 	}
		// }
		//
		// if a.showSessionDialog {
		// 	d, sessionCmd := a.sessionDialog.Update(msg)
		// 	a.sessionDialog = d.(dialog.SessionDialog)
		// 	cmds = append(cmds, sessionCmd)
		// 	// Only block key messages send all other messages down
		// 	if _, ok := msg.(tea.KeyPressMsg); ok {
		// 		return a, tea.Batch(cmds...)
		// 	}
		// }
		//
		// if a.showCommandDialog {
		// 	d, commandCmd := a.commandDialog.Update(msg)
		// 	a.commandDialog = d.(dialog.CommandDialog)
		// 	cmds = append(cmds, commandCmd)
		// 	// Only block key messages send all other messages down
		// 	if _, ok := msg.(tea.KeyPressMsg); ok {
		// 		return a, tea.Batch(cmds...)
		// 	}
		// }
		//
		// if a.showModelDialog {
		// 	d, modelCmd := a.modelDialog.Update(msg)
		// 	a.modelDialog = d.(dialog.ModelDialog)
		// 	cmds = append(cmds, modelCmd)
		// 	// Only block key messages send all other messages down
		// 	if _, ok := msg.(tea.KeyPressMsg); ok {
		// 		return a, tea.Batch(cmds...)
		// 	}
		// }
		//
		// if a.showInitDialog {
		// 	d, initCmd := a.initDialog.Update(msg)
		// 	a.initDialog = d.(dialog.InitDialogCmp)
		// 	cmds = append(cmds, initCmd)
		// 	// Only block key messages send all other messages down
		// 	if _, ok := msg.(tea.KeyPressMsg); ok {
		// 		return a, tea.Batch(cmds...)
		// 	}
		// }
		//
		// if a.showThemeDialog {
		// 	d, themeCmd := a.themeDialog.Update(msg)
		// 	a.themeDialog = d.(dialog.ThemeDialog)
		// 	cmds = append(cmds, themeCmd)
		// 	// Only block key messages send all other messages down
		// 	if _, ok := msg.(tea.KeyPressMsg); ok {
		// 		return a, tea.Batch(cmds...)
		// 	}
	}
	//
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

	// if a.showPermissions {
	// 	overlay := a.permissions.View().String()
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// if a.showFilepicker {
	// 	overlay := a.filepicker.View().String()
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// // Show compacting status overlay
	// if a.isCompacting {
	// 	t := theme.CurrentTheme()
	// 	style := lipgloss.NewStyle().
	// 		Border(lipgloss.RoundedBorder()).
	// 		BorderForeground(t.BorderFocused()).
	// 		BorderBackground(t.Background()).
	// 		Padding(1, 2).
	// 		Background(t.Background()).
	// 		Foreground(t.Text())
	//
	// 	overlay := style.Render("Summarizing\n" + a.compactingMessage)
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// if a.showHelp {
	// 	bindings := layout.KeyMapToSlice(a.keymap)
	// 	if p, ok := a.pages[a.currentPage].(layout.Bindings); ok {
	// 		bindings = append(bindings, p.BindingKeys()...)
	// 	}
	// 	if a.showPermissions {
	// 		bindings = append(bindings, a.permissions.BindingKeys()...)
	// 	}
	// 	if a.currentPage == page.LogsPage {
	// 		// bindings = append(bindings, logsKeyReturnKey)
	// 	}
	// 	if !a.app.CoderAgent.IsBusy() {
	// 		// bindings = append(bindings, helpEsc)
	// 	}
	//
	// 	a.help.SetBindings(bindings)
	//
	// 	overlay := a.help.View().String()
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// if a.showSessionDialog {
	// 	overlay := a.sessionDialog.View().String()
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// if a.showModelDialog {
	// 	overlay := a.modelDialog.View().String()
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// if a.showCommandDialog {
	// 	overlay := a.commandDialog.View().String()
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// if a.showInitDialog {
	// 	overlay := a.initDialog.View()
	// 	appView = layout.PlaceOverlay(
	// 		a.width/2-lipgloss.Width(overlay)/2,
	// 		a.height/2-lipgloss.Height(overlay)/2,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// if a.showThemeDialog {
	// 	overlay := a.themeDialog.View().String()
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	//
	// if a.showMultiArgumentsDialog {
	// 	overlay := a.multiArgumentsDialog.View()
	// 	row := lipgloss.Height(appView) / 2
	// 	row -= lipgloss.Height(overlay) / 2
	// 	col := lipgloss.Width(appView) / 2
	// 	col -= lipgloss.Width(overlay) / 2
	// 	appView = layout.PlaceOverlay(
	// 		col,
	// 		row,
	// 		overlay,
	// 		appView,
	// 		true,
	// 	)
	// }
	t := theme.CurrentTheme()
	if a.dialog.HasDialogs() {
		layers := append(
			[]*lipgloss.Layer{
				lipgloss.NewLayer(appView),
			},
			a.dialog.GetLayers()...,
		)
		canvas := lipgloss.NewCanvas(
			layers...,
		)
		view := tea.NewView(canvas.Render())
		activeView := a.dialog.ActiveView()
		view.SetBackgroundColor(t.Background())
		view.SetCursor(activeView.Cursor())
		return view
	}

	view := tea.NewView(appView)
	view.SetCursor(pageView.Cursor())
	view.SetBackgroundColor(t.Background())
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

		// help:          dialog.NewHelpCmp(),
		// sessionDialog: dialog.NewSessionDialogCmp(),
		// commandDialog: dialog.NewCommandDialogCmp(),
		// modelDialog:   dialog.NewModelDialogCmp(),
		// permissions:   dialog.NewPermissionDialogCmp(),
		// initDialog:    dialog.NewInitDialogCmp(),
		// themeDialog:   dialog.NewThemeDialogCmp(),
		// commands:      []dialog.Command{},
		pages: map[page.PageID]util.Model{
			page.ChatPage: page.NewChatPage(app),
			page.LogsPage: page.NewLogsPage(),
		},
		// filepicker: dialog.NewFilepickerCmp(app),

		// New dialog
		dialog: dialogs.NewDialogCmp(),
	}

	// 	model.RegisterCommand(dialog.Command{
	// 		ID:          "init",
	// 		Title:       "Initialize Project",
	// 		Description: "Create/Update the OpenCode.md memory file",
	// 		Handler: func(cmd dialog.Command) tea.Cmd {
	// 			prompt := `Please analyze this codebase and create a OpenCode.md file containing:
	// 1. Build/lint/test commands - especially for running a single test
	// 2. Code style guidelines including imports, formatting, types, naming conventions, error handling, etc.
	//
	// The file you create will be given to agentic coding agents (such as yourself) that operate in this repository. Make it about 20 lines long.
	// If there's already a opencode.md, improve it.
	// If there are Cursor rules (in .cursor/rules/ or .cursorrules) or Copilot rules (in .github/copilot-instructions.md), make sure to include them.`
	// 			return tea.Batch(
	// 				util.CmdHandler(chat.SendMsg{
	// 					Text: prompt,
	// 				}),
	// 			)
	// 		},
	// 	})
	//
	// 	model.RegisterCommand(dialog.Command{
	// 		ID:          "compact",
	// 		Title:       "Compact Session",
	// 		Description: "Summarize the current session and create a new one with the summary",
	// 		Handler: func(cmd dialog.Command) tea.Cmd {
	// 			return func() tea.Msg {
	// 				return startCompactSessionMsg{}
	// 			}
	// 		},
	// 	})
	// 	// Load custom commands
	// 	customCommands, err := dialog.LoadCustomCommands()
	// 	if err != nil {
	// 		logging.Warn("Failed to load custom commands", "error", err)
	// 	} else {
	// 		for _, cmd := range customCommands {
	// 			model.RegisterCommand(cmd)
	// 		}
	// 	}

	return model
}
