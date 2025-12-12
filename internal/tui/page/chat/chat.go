package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/chat/editor"
	"github.com/charmbracelet/crush/internal/tui/components/chat/header"
	"github.com/charmbracelet/crush/internal/tui/components/chat/messages"
	"github.com/charmbracelet/crush/internal/tui/components/chat/sidebar"
	"github.com/charmbracelet/crush/internal/tui/components/chat/splash"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/claude"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/commands"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/filepicker"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/models"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/reasoning"
	"github.com/charmbracelet/crush/internal/tui/page"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
)

var ChatPageID page.PageID = "chat"

type (
	ChatFocusedMsg struct {
		Focused bool
	}
	CancelTimerExpiredMsg struct{}
)

type PanelType string

const (
	PanelTypeChat   PanelType = "chat"
	PanelTypeEditor PanelType = "editor"
	PanelTypeSplash PanelType = "splash"
)

// PillSection represents which pill section is focused when in pills panel.
type PillSection int

const (
	PillSectionTodos PillSection = iota
	PillSectionQueue
)

const (
	CompactModeWidthBreakpoint  = 120 // Width at which the chat page switches to compact mode
	CompactModeHeightBreakpoint = 30  // Height at which the chat page switches to compact mode
	EditorHeight                = 5   // Height of the editor input area including padding
	SideBarWidth                = 31  // Width of the sidebar
	SideBarDetailsPadding       = 1   // Padding for the sidebar details section
	HeaderHeight                = 1   // Height of the header

	// Layout constants for borders and padding
	BorderWidth        = 1 // Width of component borders
	LeftRightBorders   = 2 // Left + right border width (1 + 1)
	TopBottomBorders   = 2 // Top + bottom border width (1 + 1)
	DetailsPositioning = 2 // Positioning adjustment for details panel

	// Timing constants
	CancelTimerDuration = 2 * time.Second // Duration before cancel timer expires
)

type ChatPage interface {
	util.Model
	layout.Help
	IsChatFocused() bool
}

// cancelTimerCmd creates a command that expires the cancel timer
func cancelTimerCmd() tea.Cmd {
	return tea.Tick(CancelTimerDuration, func(time.Time) tea.Msg {
		return CancelTimerExpiredMsg{}
	})
}

type chatPage struct {
	width, height               int
	detailsWidth, detailsHeight int
	app                         *app.App
	keyboardEnhancements        tea.KeyboardEnhancementsMsg

	// Layout state
	compact      bool
	forceCompact bool
	focusedPane  PanelType

	// Session
	session session.Session
	keyMap  KeyMap

	// Components
	header  header.Header
	sidebar sidebar.Sidebar
	chat    chat.MessageListCmp
	editor  editor.Editor
	splash  splash.Splash

	// Simple state flags
	showingDetails   bool
	isCanceling      bool
	splashFullScreen bool
	isOnboarding     bool
	isProjectInit    bool
	promptQueue      int

	// Pills state
	pillsExpanded      bool
	focusedPillSection PillSection

	// Todo spinner
	todoSpinner spinner.Model
}

func New(app *app.App) ChatPage {
	t := styles.CurrentTheme()
	return &chatPage{
		app:         app,
		keyMap:      DefaultKeyMap(),
		header:      header.New(app.LSPClients),
		sidebar:     sidebar.New(app.History, app.LSPClients, false),
		chat:        chat.New(app),
		editor:      editor.New(app),
		splash:      splash.New(),
		focusedPane: PanelTypeSplash,
		todoSpinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(t.S().Base.Foreground(t.GreenDark)),
		),
	}
}

func (p *chatPage) Init() tea.Cmd {
	cfg := config.Get()
	compact := cfg.Options.TUI.CompactMode
	p.compact = compact
	p.forceCompact = compact
	p.sidebar.SetCompactMode(p.compact)

	// Set splash state based on config
	if !config.HasInitialDataConfig() {
		// First-time setup: show model selection
		p.splash.SetOnboarding(true)
		p.isOnboarding = true
		p.splashFullScreen = true
	} else if b, _ := config.ProjectNeedsInitialization(); b {
		// Project needs context initialization
		p.splash.SetProjectInit(true)
		p.isProjectInit = true
		p.splashFullScreen = true
	} else {
		// Ready to chat: focus editor, splash in background
		p.focusedPane = PanelTypeEditor
		p.splashFullScreen = false
	}

	return tea.Batch(
		p.header.Init(),
		p.sidebar.Init(),
		p.chat.Init(),
		p.editor.Init(),
		p.splash.Init(),
	)
}

func (p *chatPage) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if p.session.ID != "" && p.app.AgentCoordinator != nil {
		queueSize := p.app.AgentCoordinator.QueuedPrompts(p.session.ID)
		if queueSize != p.promptQueue {
			p.promptQueue = queueSize
			cmds = append(cmds, p.SetSize(p.width, p.height))
		}
	}
	switch msg := msg.(type) {
	case tea.KeyboardEnhancementsMsg:
		p.keyboardEnhancements = msg
		return p, nil
	case tea.MouseWheelMsg:
		if p.compact {
			msg.Y -= 1
		}
		if p.isMouseOverChat(msg.X, msg.Y) {
			u, cmd := p.chat.Update(msg)
			p.chat = u.(chat.MessageListCmp)
			return p, cmd
		}
		return p, nil
	case tea.MouseClickMsg:
		if p.isOnboarding || p.isProjectInit {
			return p, nil
		}
		if p.compact {
			msg.Y -= 1
		}
		if p.isMouseOverChat(msg.X, msg.Y) {
			p.focusedPane = PanelTypeChat
			p.chat.Focus()
			p.editor.Blur()
		} else {
			p.focusedPane = PanelTypeEditor
			p.editor.Focus()
			p.chat.Blur()
		}
		u, cmd := p.chat.Update(msg)
		p.chat = u.(chat.MessageListCmp)
		return p, cmd
	case tea.MouseMotionMsg:
		if p.compact {
			msg.Y -= 1
		}
		if msg.Button == tea.MouseLeft {
			u, cmd := p.chat.Update(msg)
			p.chat = u.(chat.MessageListCmp)
			return p, cmd
		}
		return p, nil
	case tea.MouseReleaseMsg:
		if p.isOnboarding || p.isProjectInit {
			return p, nil
		}
		if p.compact {
			msg.Y -= 1
		}
		if msg.Button == tea.MouseLeft {
			u, cmd := p.chat.Update(msg)
			p.chat = u.(chat.MessageListCmp)
			return p, cmd
		}
		return p, nil
	case chat.SelectionCopyMsg:
		u, cmd := p.chat.Update(msg)
		p.chat = u.(chat.MessageListCmp)
		return p, cmd
	case tea.WindowSizeMsg:
		u, cmd := p.editor.Update(msg)
		p.editor = u.(editor.Editor)
		return p, tea.Batch(p.SetSize(msg.Width, msg.Height), cmd)
	case CancelTimerExpiredMsg:
		p.isCanceling = false
		return p, nil
	case editor.OpenEditorMsg:
		u, cmd := p.editor.Update(msg)
		p.editor = u.(editor.Editor)
		return p, cmd
	case chat.SendMsg:
		return p, p.sendMessage(msg.Text, msg.Attachments)
	case chat.SessionSelectedMsg:
		return p, p.setSession(msg)
	case splash.SubmitAPIKeyMsg:
		u, cmd := p.splash.Update(msg)
		p.splash = u.(splash.Splash)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)
	case commands.ToggleCompactModeMsg:
		p.forceCompact = !p.forceCompact
		var cmd tea.Cmd
		if p.forceCompact {
			p.setCompactMode(true)
			cmd = p.updateCompactConfig(true)
		} else if p.width >= CompactModeWidthBreakpoint && p.height >= CompactModeHeightBreakpoint {
			p.setCompactMode(false)
			cmd = p.updateCompactConfig(false)
		}
		return p, tea.Batch(p.SetSize(p.width, p.height), cmd)
	case commands.ToggleThinkingMsg:
		return p, p.toggleThinking()
	case commands.OpenReasoningDialogMsg:
		return p, p.openReasoningDialog()
	case reasoning.ReasoningEffortSelectedMsg:
		return p, p.handleReasoningEffortSelected(msg.Effort)
	case commands.OpenExternalEditorMsg:
		u, cmd := p.editor.Update(msg)
		p.editor = u.(editor.Editor)
		return p, cmd
	case pubsub.Event[session.Session]:
		if msg.Payload.ID == p.session.ID {
			prevHasIncompleteTodos := hasIncompleteTodos(p.session.Todos)
			prevHasInProgress := p.hasInProgressTodo()
			p.session = msg.Payload
			newHasIncompleteTodos := hasIncompleteTodos(p.session.Todos)
			newHasInProgress := p.hasInProgressTodo()
			if prevHasIncompleteTodos != newHasIncompleteTodos {
				cmds = append(cmds, p.SetSize(p.width, p.height))
			}
			if !prevHasInProgress && newHasInProgress {
				cmds = append(cmds, p.todoSpinner.Tick)
			}
		}
		u, cmd := p.header.Update(msg)
		p.header = u.(header.Header)
		cmds = append(cmds, cmd)
		u, cmd = p.sidebar.Update(msg)
		p.sidebar = u.(sidebar.Sidebar)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)
	case chat.SessionClearedMsg:
		u, cmd := p.header.Update(msg)
		p.header = u.(header.Header)
		cmds = append(cmds, cmd)
		u, cmd = p.sidebar.Update(msg)
		p.sidebar = u.(sidebar.Sidebar)
		cmds = append(cmds, cmd)
		u, cmd = p.chat.Update(msg)
		p.chat = u.(chat.MessageListCmp)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)
	case filepicker.FilePickedMsg,
		completions.CompletionsClosedMsg,
		completions.SelectCompletionMsg:
		u, cmd := p.editor.Update(msg)
		p.editor = u.(editor.Editor)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)

	case claude.ValidationCompletedMsg, claude.AuthenticationCompleteMsg:
		if p.focusedPane == PanelTypeSplash {
			u, cmd := p.splash.Update(msg)
			p.splash = u.(splash.Splash)
			cmds = append(cmds, cmd)
		}
		return p, tea.Batch(cmds...)
	case models.APIKeyStateChangeMsg:
		if p.focusedPane == PanelTypeSplash {
			u, cmd := p.splash.Update(msg)
			p.splash = u.(splash.Splash)
			cmds = append(cmds, cmd)
		}
		return p, tea.Batch(cmds...)
	case pubsub.Event[message.Message],
		anim.StepMsg,
		spinner.TickMsg:
		// Update todo spinner if agent is busy and we have in-progress todos
		agentBusy := p.app.AgentCoordinator != nil && p.app.AgentCoordinator.IsBusy()
		if _, ok := msg.(spinner.TickMsg); ok && p.hasInProgressTodo() && agentBusy {
			var cmd tea.Cmd
			p.todoSpinner, cmd = p.todoSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		// Start spinner when agent becomes busy and we have in-progress todos
		if _, ok := msg.(pubsub.Event[message.Message]); ok && p.hasInProgressTodo() && agentBusy {
			cmds = append(cmds, p.todoSpinner.Tick)
		}
		if p.focusedPane == PanelTypeSplash {
			u, cmd := p.splash.Update(msg)
			p.splash = u.(splash.Splash)
			cmds = append(cmds, cmd)
		} else {
			u, cmd := p.chat.Update(msg)
			p.chat = u.(chat.MessageListCmp)
			cmds = append(cmds, cmd)
		}

		return p, tea.Batch(cmds...)
	case commands.ToggleYoloModeMsg:
		// update the editor style
		u, cmd := p.editor.Update(msg)
		p.editor = u.(editor.Editor)
		return p, cmd
	case pubsub.Event[history.File], sidebar.SessionFilesMsg:
		u, cmd := p.sidebar.Update(msg)
		p.sidebar = u.(sidebar.Sidebar)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)
	case pubsub.Event[permission.PermissionNotification]:
		u, cmd := p.chat.Update(msg)
		p.chat = u.(chat.MessageListCmp)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)

	case commands.CommandRunCustomMsg:
		if p.app.AgentCoordinator.IsBusy() {
			return p, util.ReportWarn("Agent is busy, please wait before executing a command...")
		}

		cmd := p.sendMessage(msg.Content, nil)
		if cmd != nil {
			return p, cmd
		}
	case splash.OnboardingCompleteMsg:
		p.splashFullScreen = false
		if b, _ := config.ProjectNeedsInitialization(); b {
			p.splash.SetProjectInit(true)
			p.splashFullScreen = true
			return p, p.SetSize(p.width, p.height)
		}
		err := p.app.InitCoderAgent(context.TODO())
		if err != nil {
			return p, util.ReportError(err)
		}
		p.isOnboarding = false
		p.isProjectInit = false
		p.focusedPane = PanelTypeEditor
		return p, p.SetSize(p.width, p.height)
	case commands.NewSessionsMsg:
		if p.app.AgentCoordinator.IsBusy() {
			return p, util.ReportWarn("Agent is busy, please wait before starting a new session...")
		}
		return p, p.newSession()
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.NewSession):
			// if we have no agent do nothing
			if p.app.AgentCoordinator == nil {
				return p, nil
			}
			if p.app.AgentCoordinator.IsBusy() {
				return p, util.ReportWarn("Agent is busy, please wait before starting a new session...")
			}
			return p, p.newSession()
		case key.Matches(msg, p.keyMap.AddAttachment):
			// Skip attachment handling during onboarding/splash screen
			if p.focusedPane == PanelTypeSplash || p.isOnboarding {
				u, cmd := p.splash.Update(msg)
				p.splash = u.(splash.Splash)
				return p, cmd
			}
			agentCfg := config.Get().Agents[config.AgentCoder]
			model := config.Get().GetModelByType(agentCfg.Model)
			if model == nil {
				return p, util.ReportWarn("No model configured yet")
			}
			if model.SupportsImages {
				return p, util.CmdHandler(commands.OpenFilePickerMsg{})
			} else {
				return p, util.ReportWarn("File attachments are not supported by the current model: " + model.Name)
			}
		case key.Matches(msg, p.keyMap.Tab):
			if p.session.ID == "" {
				u, cmd := p.splash.Update(msg)
				p.splash = u.(splash.Splash)
				return p, cmd
			}
			return p, p.changeFocus()
		case key.Matches(msg, p.keyMap.Cancel):
			if p.session.ID != "" && p.app.AgentCoordinator.IsBusy() {
				return p, p.cancel()
			}
		case key.Matches(msg, p.keyMap.Details):
			p.toggleDetails()
			return p, nil
		case key.Matches(msg, p.keyMap.TogglePills):
			if p.session.ID != "" {
				return p, p.togglePillsExpanded()
			}
		case key.Matches(msg, p.keyMap.PillLeft):
			if p.session.ID != "" && p.pillsExpanded {
				return p, p.switchPillSection(-1)
			}
		case key.Matches(msg, p.keyMap.PillRight):
			if p.session.ID != "" && p.pillsExpanded {
				return p, p.switchPillSection(1)
			}
		}

		switch p.focusedPane {
		case PanelTypeChat:
			u, cmd := p.chat.Update(msg)
			p.chat = u.(chat.MessageListCmp)
			cmds = append(cmds, cmd)
		case PanelTypeEditor:
			u, cmd := p.editor.Update(msg)
			p.editor = u.(editor.Editor)
			cmds = append(cmds, cmd)
		case PanelTypeSplash:
			u, cmd := p.splash.Update(msg)
			p.splash = u.(splash.Splash)
			cmds = append(cmds, cmd)
		}
	case tea.PasteMsg:
		switch p.focusedPane {
		case PanelTypeEditor:
			u, cmd := p.editor.Update(msg)
			p.editor = u.(editor.Editor)
			cmds = append(cmds, cmd)
			return p, tea.Batch(cmds...)
		case PanelTypeChat:
			u, cmd := p.chat.Update(msg)
			p.chat = u.(chat.MessageListCmp)
			cmds = append(cmds, cmd)
			return p, tea.Batch(cmds...)
		case PanelTypeSplash:
			u, cmd := p.splash.Update(msg)
			p.splash = u.(splash.Splash)
			cmds = append(cmds, cmd)
			return p, tea.Batch(cmds...)
		}
	}
	return p, tea.Batch(cmds...)
}

func (p *chatPage) Cursor() *tea.Cursor {
	if p.header.ShowingDetails() {
		return nil
	}
	switch p.focusedPane {
	case PanelTypeEditor:
		return p.editor.Cursor()
	case PanelTypeSplash:
		return p.splash.Cursor()
	default:
		return nil
	}
}

func (p *chatPage) View() string {
	var chatView string
	t := styles.CurrentTheme()

	if p.session.ID == "" {
		splashView := p.splash.View()
		// Full screen during onboarding or project initialization
		if p.splashFullScreen {
			chatView = splashView
		} else {
			// Show splash + editor for new message state
			editorView := p.editor.View()
			chatView = lipgloss.JoinVertical(
				lipgloss.Left,
				t.S().Base.Render(splashView),
				editorView,
			)
		}
	} else {
		messagesView := p.chat.View()
		editorView := p.editor.View()

		hasIncompleteTodos := hasIncompleteTodos(p.session.Todos)
		hasQueue := p.promptQueue > 0
		todosFocused := p.pillsExpanded && p.focusedPillSection == PillSectionTodos
		queueFocused := p.pillsExpanded && p.focusedPillSection == PillSectionQueue

		// Use spinner when agent is busy, otherwise show static icon
		agentBusy := p.app.AgentCoordinator != nil && p.app.AgentCoordinator.IsBusy()
		inProgressIcon := t.S().Base.Foreground(t.GreenDark).Render(styles.CenterSpinnerIcon)
		if agentBusy {
			inProgressIcon = p.todoSpinner.View()
		}

		var pills []string
		if hasIncompleteTodos {
			pills = append(pills, todoPill(p.session.Todos, inProgressIcon, todosFocused, p.pillsExpanded, t))
		}
		if hasQueue {
			pills = append(pills, queuePill(p.promptQueue, queueFocused, p.pillsExpanded, t))
		}

		var expandedList string
		if p.pillsExpanded {
			if todosFocused && hasIncompleteTodos {
				expandedList = todoList(p.session.Todos, inProgressIcon, t, p.width-SideBarWidth)
			} else if queueFocused && hasQueue {
				queueItems := p.app.AgentCoordinator.QueuedPromptsList(p.session.ID)
				expandedList = queueList(queueItems, t)
			}
		}

		var pillsArea string
		if len(pills) > 0 {
			pillsRow := lipgloss.JoinHorizontal(lipgloss.Top, pills...)

			// Add help hint for expanding/collapsing pills based on state.
			var helpDesc string
			if p.pillsExpanded {
				helpDesc = "close"
			} else {
				helpDesc = "open"
			}
			// Style to match help section: keys in FgMuted, description in FgSubtle
			helpKey := t.S().Base.Foreground(t.FgMuted).Render("ctrl+space")
			helpText := t.S().Base.Foreground(t.FgSubtle).Render(helpDesc)
			helpHint := lipgloss.JoinHorizontal(lipgloss.Center, helpKey, " ", helpText)
			pillsRow = lipgloss.JoinHorizontal(lipgloss.Center, pillsRow, " ", helpHint)

			if expandedList != "" {
				pillsArea = lipgloss.JoinVertical(
					lipgloss.Left,
					pillsRow,
					expandedList,
				)
			} else {
				pillsArea = pillsRow
			}

			style := t.S().Base.MarginTop(1).PaddingLeft(3)
			pillsArea = style.Render(pillsArea)
		}

		if p.compact {
			headerView := p.header.View()
			views := []string{headerView, messagesView}
			if pillsArea != "" {
				views = append(views, pillsArea)
			}
			views = append(views, editorView)
			chatView = lipgloss.JoinVertical(lipgloss.Left, views...)
		} else {
			sidebarView := p.sidebar.View()
			var messagesColumn string
			if pillsArea != "" {
				messagesColumn = lipgloss.JoinVertical(
					lipgloss.Left,
					messagesView,
					pillsArea,
				)
			} else {
				messagesColumn = messagesView
			}
			messages := lipgloss.JoinHorizontal(
				lipgloss.Left,
				messagesColumn,
				sidebarView,
			)
			chatView = lipgloss.JoinVertical(
				lipgloss.Left,
				messages,
				p.editor.View(),
			)
		}
	}

	layers := []*lipgloss.Layer{
		lipgloss.NewLayer(chatView).X(0).Y(0),
	}

	if p.showingDetails {
		style := t.S().Base.
			Width(p.detailsWidth).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus)
		version := t.S().Base.Foreground(t.Border).Width(p.detailsWidth - 4).AlignHorizontal(lipgloss.Right).Render(version.Version)
		details := style.Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				p.sidebar.View(),
				version,
			),
		)
		layers = append(layers, lipgloss.NewLayer(details).X(1).Y(1))
	}
	canvas := lipgloss.NewCompositor(layers...)
	return canvas.Render()
}

func (p *chatPage) updateCompactConfig(compact bool) tea.Cmd {
	return func() tea.Msg {
		err := config.Get().SetCompactMode(compact)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "Failed to update compact mode configuration: " + err.Error(),
			}
		}
		return nil
	}
}

func (p *chatPage) toggleThinking() tea.Cmd {
	return func() tea.Msg {
		cfg := config.Get()
		agentCfg := cfg.Agents[config.AgentCoder]
		currentModel := cfg.Models[agentCfg.Model]

		// Toggle the thinking mode
		currentModel.Think = !currentModel.Think
		if err := cfg.UpdatePreferredModel(agentCfg.Model, currentModel); err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "Failed to update thinking mode: " + err.Error(),
			}
		}

		// Update the agent with the new configuration
		go p.app.UpdateAgentModel(context.TODO())

		status := "disabled"
		if currentModel.Think {
			status = "enabled"
		}
		return util.InfoMsg{
			Type: util.InfoTypeInfo,
			Msg:  "Thinking mode " + status,
		}
	}
}

func (p *chatPage) openReasoningDialog() tea.Cmd {
	return func() tea.Msg {
		cfg := config.Get()
		agentCfg := cfg.Agents[config.AgentCoder]
		model := cfg.GetModelByType(agentCfg.Model)
		providerCfg := cfg.GetProviderForModel(agentCfg.Model)

		if providerCfg != nil && model != nil && len(model.ReasoningLevels) > 0 {
			// Return the OpenDialogMsg directly so it bubbles up to the main TUI
			return dialogs.OpenDialogMsg{
				Model: reasoning.NewReasoningDialog(),
			}
		}
		return nil
	}
}

func (p *chatPage) handleReasoningEffortSelected(effort string) tea.Cmd {
	return func() tea.Msg {
		cfg := config.Get()
		agentCfg := cfg.Agents[config.AgentCoder]
		currentModel := cfg.Models[agentCfg.Model]

		// Update the model configuration
		currentModel.ReasoningEffort = effort
		if err := cfg.UpdatePreferredModel(agentCfg.Model, currentModel); err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "Failed to update reasoning effort: " + err.Error(),
			}
		}

		// Update the agent with the new configuration
		if err := p.app.UpdateAgentModel(context.TODO()); err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "Failed to update reasoning effort: " + err.Error(),
			}
		}

		return util.InfoMsg{
			Type: util.InfoTypeInfo,
			Msg:  "Reasoning effort set to " + effort,
		}
	}
}

func (p *chatPage) setCompactMode(compact bool) {
	if p.compact == compact {
		return
	}
	p.compact = compact
	if compact {
		p.sidebar.SetCompactMode(true)
	} else {
		p.setShowDetails(false)
	}
}

func (p *chatPage) handleCompactMode(newWidth int, newHeight int) {
	if p.forceCompact {
		return
	}
	if (newWidth < CompactModeWidthBreakpoint || newHeight < CompactModeHeightBreakpoint) && !p.compact {
		p.setCompactMode(true)
	}
	if (newWidth >= CompactModeWidthBreakpoint && newHeight >= CompactModeHeightBreakpoint) && p.compact {
		p.setCompactMode(false)
	}
}

func (p *chatPage) SetSize(width, height int) tea.Cmd {
	p.handleCompactMode(width, height)
	p.width = width
	p.height = height
	var cmds []tea.Cmd

	if p.session.ID == "" {
		if p.splashFullScreen {
			cmds = append(cmds, p.splash.SetSize(width, height))
		} else {
			cmds = append(cmds, p.splash.SetSize(width, height-EditorHeight))
			cmds = append(cmds, p.editor.SetSize(width, EditorHeight))
			cmds = append(cmds, p.editor.SetPosition(0, height-EditorHeight))
		}
	} else {
		hasIncompleteTodos := hasIncompleteTodos(p.session.Todos)
		hasQueue := p.promptQueue > 0
		hasPills := hasIncompleteTodos || hasQueue

		pillsAreaHeight := 0
		if hasPills {
			pillsAreaHeight = pillHeightWithBorder + 1 // +1 for padding top
			if p.pillsExpanded {
				if p.focusedPillSection == PillSectionTodos && hasIncompleteTodos {
					pillsAreaHeight += len(p.session.Todos)
				} else if p.focusedPillSection == PillSectionQueue && hasQueue {
					pillsAreaHeight += p.promptQueue
				}
			}
		}

		if p.compact {
			cmds = append(cmds, p.chat.SetSize(width, height-EditorHeight-HeaderHeight-pillsAreaHeight))
			p.detailsWidth = width - DetailsPositioning
			cmds = append(cmds, p.sidebar.SetSize(p.detailsWidth-LeftRightBorders, p.detailsHeight-TopBottomBorders))
			cmds = append(cmds, p.editor.SetSize(width, EditorHeight))
			cmds = append(cmds, p.header.SetWidth(width-BorderWidth))
		} else {
			cmds = append(cmds, p.chat.SetSize(width-SideBarWidth, height-EditorHeight-pillsAreaHeight))
			cmds = append(cmds, p.editor.SetSize(width, EditorHeight))
			cmds = append(cmds, p.sidebar.SetSize(SideBarWidth, height-EditorHeight))
		}
		cmds = append(cmds, p.editor.SetPosition(0, height-EditorHeight))
	}
	return tea.Batch(cmds...)
}

func (p *chatPage) newSession() tea.Cmd {
	if p.session.ID == "" {
		return nil
	}

	p.session = session.Session{}
	p.focusedPane = PanelTypeEditor
	p.editor.Focus()
	p.chat.Blur()
	p.isCanceling = false
	return tea.Batch(
		util.CmdHandler(chat.SessionClearedMsg{}),
		p.SetSize(p.width, p.height),
	)
}

func (p *chatPage) setSession(sess session.Session) tea.Cmd {
	if p.session.ID == sess.ID {
		return nil
	}

	var cmds []tea.Cmd
	p.session = sess

	if p.hasInProgressTodo() {
		cmds = append(cmds, p.todoSpinner.Tick)
	}

	cmds = append(cmds, p.SetSize(p.width, p.height))
	cmds = append(cmds, p.chat.SetSession(sess))
	cmds = append(cmds, p.sidebar.SetSession(sess))
	cmds = append(cmds, p.header.SetSession(sess))
	cmds = append(cmds, p.editor.SetSession(sess))

	return tea.Sequence(cmds...)
}

func (p *chatPage) changeFocus() tea.Cmd {
	if p.session.ID == "" {
		return nil
	}

	switch p.focusedPane {
	case PanelTypeEditor:
		p.focusedPane = PanelTypeChat
		p.chat.Focus()
		p.editor.Blur()
	case PanelTypeChat:
		p.focusedPane = PanelTypeEditor
		p.editor.Focus()
		p.chat.Blur()
	}
	return nil
}

func (p *chatPage) togglePillsExpanded() tea.Cmd {
	hasPills := hasIncompleteTodos(p.session.Todos) || p.promptQueue > 0
	if !hasPills {
		return nil
	}
	p.pillsExpanded = !p.pillsExpanded
	if p.pillsExpanded {
		if hasIncompleteTodos(p.session.Todos) {
			p.focusedPillSection = PillSectionTodos
		} else {
			p.focusedPillSection = PillSectionQueue
		}
	}
	return p.SetSize(p.width, p.height)
}

func (p *chatPage) switchPillSection(dir int) tea.Cmd {
	if !p.pillsExpanded {
		return nil
	}
	hasIncompleteTodos := hasIncompleteTodos(p.session.Todos)
	hasQueue := p.promptQueue > 0

	if dir < 0 && p.focusedPillSection == PillSectionQueue && hasIncompleteTodos {
		p.focusedPillSection = PillSectionTodos
		return p.SetSize(p.width, p.height)
	}
	if dir > 0 && p.focusedPillSection == PillSectionTodos && hasQueue {
		p.focusedPillSection = PillSectionQueue
		return p.SetSize(p.width, p.height)
	}
	return nil
}

func (p *chatPage) cancel() tea.Cmd {
	if p.isCanceling {
		p.isCanceling = false
		if p.app.AgentCoordinator != nil {
			p.app.AgentCoordinator.Cancel(p.session.ID)
		}
		return nil
	}

	if p.app.AgentCoordinator != nil && p.app.AgentCoordinator.QueuedPrompts(p.session.ID) > 0 {
		p.app.AgentCoordinator.ClearQueue(p.session.ID)
		return nil
	}
	p.isCanceling = true
	return cancelTimerCmd()
}

func (p *chatPage) setShowDetails(show bool) {
	p.showingDetails = show
	p.header.SetDetailsOpen(p.showingDetails)
	if !p.compact {
		p.sidebar.SetCompactMode(false)
	}
}

func (p *chatPage) toggleDetails() {
	if p.session.ID == "" || !p.compact {
		return
	}
	p.setShowDetails(!p.showingDetails)
}

func (p *chatPage) sendMessage(text string, attachments []message.Attachment) tea.Cmd {
	session := p.session
	var cmds []tea.Cmd
	if p.session.ID == "" {
		newSession, err := p.app.Sessions.Create(context.Background(), "New Session")
		if err != nil {
			return util.ReportError(err)
		}
		session = newSession
		cmds = append(cmds, util.CmdHandler(chat.SessionSelectedMsg(session)))
	}
	if p.app.AgentCoordinator == nil {
		return util.ReportError(fmt.Errorf("coder agent is not initialized"))
	}
	cmds = append(cmds, p.chat.GoToBottom())
	cmds = append(cmds, func() tea.Msg {
		_, err := p.app.AgentCoordinator.Run(context.Background(), session.ID, text, attachments...)
		if err != nil {
			isCancelErr := errors.Is(err, context.Canceled)
			isPermissionErr := errors.Is(err, permission.ErrorPermissionDenied)
			if isCancelErr || isPermissionErr {
				return nil
			}
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  err.Error(),
			}
		}
		return nil
	})
	return tea.Batch(cmds...)
}

func (p *chatPage) Bindings() []key.Binding {
	bindings := []key.Binding{
		p.keyMap.NewSession,
		p.keyMap.AddAttachment,
	}
	if p.app.AgentCoordinator != nil && p.app.AgentCoordinator.IsBusy() {
		cancelBinding := p.keyMap.Cancel
		if p.isCanceling {
			cancelBinding = key.NewBinding(
				key.WithKeys("esc", "alt+esc"),
				key.WithHelp("esc", "press again to cancel"),
			)
		}
		bindings = append([]key.Binding{cancelBinding}, bindings...)
	}

	switch p.focusedPane {
	case PanelTypeChat:
		bindings = append([]key.Binding{
			key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "focus editor"),
			),
		}, bindings...)
		bindings = append(bindings, p.chat.Bindings()...)
	case PanelTypeEditor:
		bindings = append([]key.Binding{
			key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "focus chat"),
			),
		}, bindings...)
		bindings = append(bindings, p.editor.Bindings()...)
	case PanelTypeSplash:
		bindings = append(bindings, p.splash.Bindings()...)
	}

	return bindings
}

func (p *chatPage) Help() help.KeyMap {
	var shortList []key.Binding
	var fullList [][]key.Binding
	switch {
	case p.isOnboarding && p.splash.IsShowingClaudeAuthMethodChooser():
		shortList = append(shortList,
			// Choose auth method
			key.NewBinding(
				key.WithKeys("left", "right", "tab"),
				key.WithHelp("←→/tab", "choose"),
			),
			// Accept selection
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "accept"),
			),
			// Go back
			key.NewBinding(
				key.WithKeys("esc", "alt+esc"),
				key.WithHelp("esc", "back"),
			),
			// Quit
			key.NewBinding(
				key.WithKeys("ctrl+c"),
				key.WithHelp("ctrl+c", "quit"),
			),
		)
		// keep them the same
		for _, v := range shortList {
			fullList = append(fullList, []key.Binding{v})
		}
	case p.isOnboarding && p.splash.IsShowingClaudeOAuth2():
		if p.splash.IsClaudeOAuthURLState() {
			shortList = append(shortList,
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "open"),
				),
				key.NewBinding(
					key.WithKeys("c"),
					key.WithHelp("c", "copy url"),
				),
			)
		} else if p.splash.IsClaudeOAuthComplete() {
			shortList = append(shortList,
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "continue"),
				),
			)
		} else {
			shortList = append(shortList,
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "submit"),
				),
			)
		}
		shortList = append(shortList,
			// Quit
			key.NewBinding(
				key.WithKeys("ctrl+c"),
				key.WithHelp("ctrl+c", "quit"),
			),
		)
		// keep them the same
		for _, v := range shortList {
			fullList = append(fullList, []key.Binding{v})
		}
	case p.isOnboarding && !p.splash.IsShowingAPIKey():
		shortList = append(shortList,
			// Choose model
			key.NewBinding(
				key.WithKeys("up", "down"),
				key.WithHelp("↑/↓", "choose"),
			),
			// Accept selection
			key.NewBinding(
				key.WithKeys("enter", "ctrl+y"),
				key.WithHelp("enter", "accept"),
			),
			// Quit
			key.NewBinding(
				key.WithKeys("ctrl+c"),
				key.WithHelp("ctrl+c", "quit"),
			),
		)
		// keep them the same
		for _, v := range shortList {
			fullList = append(fullList, []key.Binding{v})
		}
	case p.isOnboarding && p.splash.IsShowingAPIKey():
		if p.splash.IsAPIKeyValid() {
			shortList = append(shortList,
				key.NewBinding(
					key.WithKeys("enter"),
					key.WithHelp("enter", "continue"),
				),
			)
		} else {
			shortList = append(shortList,
				// Go back
				key.NewBinding(
					key.WithKeys("esc", "alt+esc"),
					key.WithHelp("esc", "back"),
				),
			)
		}
		shortList = append(shortList,
			// Quit
			key.NewBinding(
				key.WithKeys("ctrl+c"),
				key.WithHelp("ctrl+c", "quit"),
			),
		)
		// keep them the same
		for _, v := range shortList {
			fullList = append(fullList, []key.Binding{v})
		}
	case p.isProjectInit:
		shortList = append(shortList,
			key.NewBinding(
				key.WithKeys("ctrl+c"),
				key.WithHelp("ctrl+c", "quit"),
			),
		)
		// keep them the same
		for _, v := range shortList {
			fullList = append(fullList, []key.Binding{v})
		}
	default:
		if p.editor.IsCompletionsOpen() {
			shortList = append(shortList,
				key.NewBinding(
					key.WithKeys("tab", "enter"),
					key.WithHelp("tab/enter", "complete"),
				),
				key.NewBinding(
					key.WithKeys("esc", "alt+esc"),
					key.WithHelp("esc", "cancel"),
				),
				key.NewBinding(
					key.WithKeys("up", "down"),
					key.WithHelp("↑/↓", "choose"),
				),
			)
			for _, v := range shortList {
				fullList = append(fullList, []key.Binding{v})
			}
			return core.NewSimpleHelp(shortList, fullList)
		}
		if p.app.AgentCoordinator != nil && p.app.AgentCoordinator.IsBusy() {
			cancelBinding := key.NewBinding(
				key.WithKeys("esc", "alt+esc"),
				key.WithHelp("esc", "cancel"),
			)
			if p.isCanceling {
				cancelBinding = key.NewBinding(
					key.WithKeys("esc", "alt+esc"),
					key.WithHelp("esc", "press again to cancel"),
				)
			}
			if p.app.AgentCoordinator != nil && p.app.AgentCoordinator.QueuedPrompts(p.session.ID) > 0 {
				cancelBinding = key.NewBinding(
					key.WithKeys("esc", "alt+esc"),
					key.WithHelp("esc", "clear queue"),
				)
			}
			shortList = append(shortList, cancelBinding)
			fullList = append(fullList,
				[]key.Binding{
					cancelBinding,
				},
			)
		}
		globalBindings := []key.Binding{}
		// we are in a session
		if p.session.ID != "" {
			var tabKey key.Binding
			switch p.focusedPane {
			case PanelTypeEditor:
				tabKey = key.NewBinding(
					key.WithKeys("tab"),
					key.WithHelp("tab", "focus chat"),
				)
			case PanelTypeChat:
				tabKey = key.NewBinding(
					key.WithKeys("tab"),
					key.WithHelp("tab", "focus editor"),
				)
			default:
				tabKey = key.NewBinding(
					key.WithKeys("tab"),
					key.WithHelp("tab", "focus chat"),
				)
			}
			shortList = append(shortList, tabKey)
			globalBindings = append(globalBindings, tabKey)

			// Show left/right to switch sections when expanded and both exist
			hasTodos := hasIncompleteTodos(p.session.Todos)
			hasQueue := p.promptQueue > 0
			if p.pillsExpanded && hasTodos && hasQueue {
				shortList = append(shortList, p.keyMap.PillLeft)
				globalBindings = append(globalBindings, p.keyMap.PillLeft)
			}
		}
		commandsBinding := key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("ctrl+p", "commands"),
		)
		if p.focusedPane == PanelTypeEditor && p.editor.IsEmpty() {
			commandsBinding.SetHelp("/ or ctrl+p", "commands")
		}
		modelsBinding := key.NewBinding(
			key.WithKeys("ctrl+m", "ctrl+l"),
			key.WithHelp("ctrl+l", "models"),
		)
		if p.keyboardEnhancements.Flags > 0 {
			// non-zero flags mean we have at least key disambiguation
			modelsBinding.SetHelp("ctrl+m", "models")
		}
		helpBinding := key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "more"),
		)
		globalBindings = append(globalBindings, commandsBinding, modelsBinding)
		globalBindings = append(globalBindings,
			key.NewBinding(
				key.WithKeys("ctrl+s"),
				key.WithHelp("ctrl+s", "sessions"),
			),
		)
		if p.session.ID != "" {
			globalBindings = append(globalBindings,
				key.NewBinding(
					key.WithKeys("ctrl+n"),
					key.WithHelp("ctrl+n", "new sessions"),
				))
		}
		shortList = append(shortList,
			// Commands
			commandsBinding,
			modelsBinding,
		)
		fullList = append(fullList, globalBindings)

		switch p.focusedPane {
		case PanelTypeChat:
			shortList = append(shortList,
				key.NewBinding(
					key.WithKeys("up", "down"),
					key.WithHelp("↑↓", "scroll"),
				),
				messages.CopyKey,
			)
			fullList = append(fullList,
				[]key.Binding{
					key.NewBinding(
						key.WithKeys("up", "down"),
						key.WithHelp("↑↓", "scroll"),
					),
					key.NewBinding(
						key.WithKeys("shift+up", "shift+down"),
						key.WithHelp("shift+↑↓", "next/prev item"),
					),
					key.NewBinding(
						key.WithKeys("pgup", "b"),
						key.WithHelp("b/pgup", "page up"),
					),
					key.NewBinding(
						key.WithKeys("pgdown", " ", "f"),
						key.WithHelp("f/pgdn", "page down"),
					),
				},
				[]key.Binding{
					key.NewBinding(
						key.WithKeys("u"),
						key.WithHelp("u", "half page up"),
					),
					key.NewBinding(
						key.WithKeys("d"),
						key.WithHelp("d", "half page down"),
					),
					key.NewBinding(
						key.WithKeys("g", "home"),
						key.WithHelp("g", "home"),
					),
					key.NewBinding(
						key.WithKeys("G", "end"),
						key.WithHelp("G", "end"),
					),
				},
				[]key.Binding{
					messages.CopyKey,
					messages.ClearSelectionKey,
				},
			)
		case PanelTypeEditor:
			newLineBinding := key.NewBinding(
				key.WithKeys("shift+enter", "ctrl+j"),
				// "ctrl+j" is a common keybinding for newline in many editors. If
				// the terminal supports "shift+enter", we substitute the help text
				// to reflect that.
				key.WithHelp("ctrl+j", "newline"),
			)
			if p.keyboardEnhancements.Flags > 0 {
				// Non-zero flags mean we have at least key disambiguation.
				newLineBinding.SetHelp("shift+enter", newLineBinding.Help().Desc)
			}
			shortList = append(shortList, newLineBinding)
			fullList = append(fullList,
				[]key.Binding{
					newLineBinding,
					key.NewBinding(
						key.WithKeys("ctrl+f"),
						key.WithHelp("ctrl+f", "add image"),
					),
					key.NewBinding(
						key.WithKeys("@"),
						key.WithHelp("@", "mention file"),
					),
					key.NewBinding(
						key.WithKeys("ctrl+o"),
						key.WithHelp("ctrl+o", "open editor"),
					),
				})

			if p.editor.HasAttachments() {
				fullList = append(fullList, []key.Binding{
					key.NewBinding(
						key.WithKeys("ctrl+r"),
						key.WithHelp("ctrl+r+{i}", "delete attachment at index i"),
					),
					key.NewBinding(
						key.WithKeys("ctrl+r", "r"),
						key.WithHelp("ctrl+r+r", "delete all attachments"),
					),
					key.NewBinding(
						key.WithKeys("esc", "alt+esc"),
						key.WithHelp("esc", "cancel delete mode"),
					),
				})
			}
		}
		shortList = append(shortList,
			// Quit
			key.NewBinding(
				key.WithKeys("ctrl+c"),
				key.WithHelp("ctrl+c", "quit"),
			),
			// Help
			helpBinding,
		)
		fullList = append(fullList, []key.Binding{
			key.NewBinding(
				key.WithKeys("ctrl+g"),
				key.WithHelp("ctrl+g", "less"),
			),
		})
	}

	return core.NewSimpleHelp(shortList, fullList)
}

func (p *chatPage) IsChatFocused() bool {
	return p.focusedPane == PanelTypeChat
}

// isMouseOverChat checks if the given mouse coordinates are within the chat area bounds.
// Returns true if the mouse is over the chat area, false otherwise.
func (p *chatPage) isMouseOverChat(x, y int) bool {
	// No session means no chat area
	if p.session.ID == "" {
		return false
	}

	var chatX, chatY, chatWidth, chatHeight int

	if p.compact {
		// In compact mode: chat area starts after header and spans full width
		chatX = 0
		chatY = HeaderHeight
		chatWidth = p.width
		chatHeight = p.height - EditorHeight - HeaderHeight
	} else {
		// In non-compact mode: chat area spans from left edge to sidebar
		chatX = 0
		chatY = 0
		chatWidth = p.width - SideBarWidth
		chatHeight = p.height - EditorHeight
	}

	// Check if mouse coordinates are within chat bounds
	return x >= chatX && x < chatX+chatWidth && y >= chatY && y < chatY+chatHeight
}

func (p *chatPage) hasInProgressTodo() bool {
	for _, todo := range p.session.Todos {
		if todo.Status == session.TodoStatusInProgress {
			return true
		}
	}
	return false
}
