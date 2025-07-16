package splash

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fur/provider"
	"github.com/charmbracelet/crush/internal/llm/prompt"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/core/list"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/models"
	"github.com/charmbracelet/crush/internal/tui/components/logo"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/lipgloss/v2"
)

type Splash interface {
	util.Model
	layout.Sizeable
	layout.Help
	Cursor() *tea.Cursor
	// SetOnboarding controls whether the splash shows model selection UI
	SetOnboarding(bool)
	// SetProjectInit controls whether the splash shows project initialization prompt
	SetProjectInit(bool)

	// Showing API key input
	IsShowingAPIKey() bool
}

const (
	SplashScreenPaddingY = 1 // Padding Y for the splash screen

	LogoGap = 6
)

// OnboardingCompleteMsg is sent when onboarding is complete
type OnboardingCompleteMsg struct{}

type splashCmp struct {
	width, height int
	keyMap        KeyMap
	logoRendered  string

	// State
	isOnboarding     bool
	needsProjectInit bool
	needsAPIKey      bool
	selectedNo       bool

	listHeight    int
	modelList     *models.ModelListComponent
	apiKeyInput   *models.APIKeyInput
	selectedModel *models.ModelOption
}

func New() Splash {
	keyMap := DefaultKeyMap()
	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.HalfPageDown.SetEnabled(false)
	listKeyMap.HalfPageUp.SetEnabled(false)
	listKeyMap.Home.SetEnabled(false)
	listKeyMap.End.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()
	inputStyle := t.S().Base.Padding(0, 1, 0, 1)
	modelList := models.NewModelListComponent(listKeyMap, inputStyle, "Find your fave")
	apiKeyInput := models.NewAPIKeyInput()

	return &splashCmp{
		width:        0,
		height:       0,
		keyMap:       keyMap,
		logoRendered: "",
		modelList:    modelList,
		apiKeyInput:  apiKeyInput,
		selectedNo:   false,
	}
}

func (s *splashCmp) SetOnboarding(onboarding bool) {
	s.isOnboarding = onboarding
	if onboarding {
		providers, err := config.Providers()
		if err != nil {
			return
		}
		filteredProviders := []provider.Provider{}
		simpleProviders := []string{
			"anthropic",
			"openai",
			"gemini",
			"xai",
			"groq",
			"openrouter",
		}
		for _, p := range providers {
			if slices.Contains(simpleProviders, string(p.ID)) {
				filteredProviders = append(filteredProviders, p)
			}
		}
		s.modelList.SetProviders(filteredProviders)
	}
}

func (s *splashCmp) SetProjectInit(needsInit bool) {
	s.needsProjectInit = needsInit
}

// GetSize implements SplashPage.
func (s *splashCmp) GetSize() (int, int) {
	return s.width, s.height
}

// Init implements SplashPage.
func (s *splashCmp) Init() tea.Cmd {
	return tea.Batch(s.modelList.Init(), s.apiKeyInput.Init())
}

// SetSize implements SplashPage.
func (s *splashCmp) SetSize(width int, height int) tea.Cmd {
	s.height = height
	if width != s.width {
		s.width = width
		s.logoRendered = s.logoBlock()
	}
	// remove padding, logo height, gap, title space
	s.listHeight = s.height - lipgloss.Height(s.logoRendered) - (SplashScreenPaddingY * 2) - s.logoGap() - 2
	listWidth := min(60, width)
	return s.modelList.SetSize(listWidth, s.listHeight)
}

// Update implements SplashPage.
func (s *splashCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s, s.SetSize(msg.Width, msg.Height)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Back):
			if s.needsAPIKey {
				// Go back to model selection
				s.needsAPIKey = false
				s.selectedModel = nil
				return s, nil
			}
		case key.Matches(msg, s.keyMap.Select):
			if s.isOnboarding && !s.needsAPIKey {
				modelInx := s.modelList.SelectedIndex()
				items := s.modelList.Items()
				selectedItem := items[modelInx].(completions.CompletionItem).Value().(models.ModelOption)
				if s.isProviderConfigured(string(selectedItem.Provider.ID)) {
					cmd := s.setPreferredModel(selectedItem)
					s.isOnboarding = false
					return s, tea.Batch(cmd, util.CmdHandler(OnboardingCompleteMsg{}))
				} else {
					// Provider not configured, show API key input
					s.needsAPIKey = true
					s.selectedModel = &selectedItem
					s.apiKeyInput.SetProviderName(selectedItem.Provider.Name)
					return s, nil
				}
			} else if s.needsAPIKey {
				// Handle API key submission
				apiKey := s.apiKeyInput.Value()
				if apiKey != "" {
					return s, s.saveAPIKeyAndContinue(apiKey)
				}
			} else if s.needsProjectInit {
				return s, s.initializeProject()
			}
		case key.Matches(msg, s.keyMap.Tab, s.keyMap.LeftRight):
			if s.needsProjectInit {
				s.selectedNo = !s.selectedNo
				return s, nil
			}
		case key.Matches(msg, s.keyMap.Yes):
			if s.needsProjectInit {
				return s, s.initializeProject()
			}
		case key.Matches(msg, s.keyMap.No):
			s.selectedNo = true
			return s, s.initializeProject()
		default:
			if s.needsAPIKey {
				u, cmd := s.apiKeyInput.Update(msg)
				s.apiKeyInput = u.(*models.APIKeyInput)
				return s, cmd
			} else if s.isOnboarding {
				u, cmd := s.modelList.Update(msg)
				s.modelList = u
				return s, cmd
			}
		}
	case tea.PasteMsg:
		if s.needsAPIKey {
			u, cmd := s.apiKeyInput.Update(msg)
			s.apiKeyInput = u.(*models.APIKeyInput)
			return s, cmd
		} else if s.isOnboarding {
			var cmd tea.Cmd
			s.modelList, cmd = s.modelList.Update(msg)
			return s, cmd
		}
	}
	return s, nil
}

func (s *splashCmp) saveAPIKeyAndContinue(apiKey string) tea.Cmd {
	if s.selectedModel == nil {
		return util.ReportError(fmt.Errorf("no model selected"))
	}

	cfg := config.Get()
	err := cfg.SetProviderAPIKey(string(s.selectedModel.Provider.ID), apiKey)
	if err != nil {
		return util.ReportError(fmt.Errorf("failed to save API key: %w", err))
	}

	// Reset API key state and continue with model selection
	s.needsAPIKey = false
	cmd := s.setPreferredModel(*s.selectedModel)
	s.isOnboarding = false
	s.selectedModel = nil

	return tea.Batch(cmd, util.CmdHandler(OnboardingCompleteMsg{}))
}

func (s *splashCmp) initializeProject() tea.Cmd {
	s.needsProjectInit = false

	if err := config.MarkProjectInitialized(); err != nil {
		return util.ReportError(err)
	}
	var cmds []tea.Cmd

	cmds = append(cmds, util.CmdHandler(OnboardingCompleteMsg{}))
	if !s.selectedNo {
		cmds = append(cmds,
			util.CmdHandler(chat.SessionClearedMsg{}),
			util.CmdHandler(chat.SendMsg{
				Text: prompt.Initialize(),
			}),
		)
	}
	return tea.Sequence(cmds...)
}

func (s *splashCmp) setPreferredModel(selectedItem models.ModelOption) tea.Cmd {
	cfg := config.Get()
	model := cfg.GetModel(string(selectedItem.Provider.ID), selectedItem.Model.ID)
	if model == nil {
		return util.ReportError(fmt.Errorf("model %s not found for provider %s", selectedItem.Model.ID, selectedItem.Provider.ID))
	}

	selectedModel := config.SelectedModel{
		Model:           selectedItem.Model.ID,
		Provider:        string(selectedItem.Provider.ID),
		ReasoningEffort: model.DefaultReasoningEffort,
		MaxTokens:       model.DefaultMaxTokens,
	}

	err := cfg.UpdatePreferredModel(config.SelectedModelTypeLarge, selectedModel)
	if err != nil {
		return util.ReportError(err)
	}

	// Now lets automatically setup the small model
	knownProvider, err := s.getProvider(selectedItem.Provider.ID)
	if err != nil {
		return util.ReportError(err)
	}
	if knownProvider == nil {
		// for local provider we just use the same model
		err = cfg.UpdatePreferredModel(config.SelectedModelTypeSmall, selectedModel)
		if err != nil {
			return util.ReportError(err)
		}
	} else {
		smallModel := knownProvider.DefaultSmallModelID
		model := cfg.GetModel(string(selectedItem.Provider.ID), smallModel)
		// should never happen
		if model == nil {
			err = cfg.UpdatePreferredModel(config.SelectedModelTypeSmall, selectedModel)
			if err != nil {
				return util.ReportError(err)
			}
			return nil
		}
		smallSelectedModel := config.SelectedModel{
			Model:           smallModel,
			Provider:        string(selectedItem.Provider.ID),
			ReasoningEffort: model.DefaultReasoningEffort,
			MaxTokens:       model.DefaultMaxTokens,
		}
		err = cfg.UpdatePreferredModel(config.SelectedModelTypeSmall, smallSelectedModel)
		if err != nil {
			return util.ReportError(err)
		}
	}
	cfg.SetupAgents()
	return nil
}

func (s *splashCmp) getProvider(providerID provider.InferenceProvider) (*provider.Provider, error) {
	providers, err := config.Providers()
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		if p.ID == providerID {
			return &p, nil
		}
	}
	return nil, nil
}

func (s *splashCmp) isProviderConfigured(providerID string) bool {
	cfg := config.Get()
	if _, ok := cfg.Providers[providerID]; ok {
		return true
	}
	return false
}

func (s *splashCmp) View() string {
	t := styles.CurrentTheme()
	var content string
	if s.needsAPIKey {
		remainingHeight := s.height - lipgloss.Height(s.logoRendered) - (SplashScreenPaddingY * 2)
		apiKeyView := t.S().Base.PaddingLeft(1).Render(s.apiKeyInput.View())
		apiKeySelector := t.S().Base.AlignVertical(lipgloss.Bottom).Height(remainingHeight).Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				apiKeyView,
			),
		)
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			s.logoRendered,
			apiKeySelector,
		)
	} else if s.isOnboarding {
		modelListView := s.modelList.View()
		remainingHeight := s.height - lipgloss.Height(s.logoRendered) - (SplashScreenPaddingY * 2)
		modelSelector := t.S().Base.AlignVertical(lipgloss.Bottom).Height(remainingHeight).Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				t.S().Base.PaddingLeft(1).Foreground(t.Primary).Render("Choose a Model"),
				"",
				modelListView,
			),
		)
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			s.logoRendered,
			modelSelector,
		)
	} else if s.needsProjectInit {
		titleStyle := t.S().Base.Foreground(t.FgBase)
		bodyStyle := t.S().Base.Foreground(t.FgMuted)
		shortcutStyle := t.S().Base.Foreground(t.Success)

		initText := lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render("Would you like to initialize this project?"),
			"",
			bodyStyle.Render("When I initialize your codebase I examine the project and put the"),
			bodyStyle.Render("result into a CRUSH.md file which serves as general context."),
			"",
			bodyStyle.Render("You can also initialize anytime via ")+shortcutStyle.Render("ctrl+p")+bodyStyle.Render("."),
			"",
			bodyStyle.Render("Would you like to initialize now?"),
		)

		yesButton := core.SelectableButton(core.ButtonOpts{
			Text:           "Yep!",
			UnderlineIndex: 0,
			Selected:       !s.selectedNo,
		})

		noButton := core.SelectableButton(core.ButtonOpts{
			Text:           "Nope",
			UnderlineIndex: 0,
			Selected:       s.selectedNo,
		})

		buttons := lipgloss.JoinHorizontal(lipgloss.Left, yesButton, "  ", noButton)
		infoSection := s.infoSection()

		remainingHeight := s.height - lipgloss.Height(s.logoRendered) - (SplashScreenPaddingY * 2) - lipgloss.Height(infoSection)

		initContent := t.S().Base.AlignVertical(lipgloss.Bottom).PaddingLeft(1).Height(remainingHeight).Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				initText,
				"",
				buttons,
			),
		)

		content = lipgloss.JoinVertical(
			lipgloss.Left,
			s.logoRendered,
			infoSection,
			initContent,
		)
	} else {
		parts := []string{
			s.logoRendered,
			s.infoSection(),
		}
		content = lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	return t.S().Base.
		Width(s.width).
		Height(s.height).
		PaddingTop(SplashScreenPaddingY).
		PaddingBottom(SplashScreenPaddingY).
		Render(content)
}

func (s *splashCmp) Cursor() *tea.Cursor {
	if s.needsAPIKey {
		cursor := s.apiKeyInput.Cursor()
		if cursor != nil {
			return s.moveCursor(cursor)
		}
	} else if s.isOnboarding {
		cursor := s.modelList.Cursor()
		if cursor != nil {
			return s.moveCursor(cursor)
		}
	} else {
		return nil
	}
	return nil
}

func (s *splashCmp) infoSection() string {
	t := styles.CurrentTheme()
	return t.S().Base.PaddingLeft(2).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			s.cwd(),
			"",
			lipgloss.JoinHorizontal(lipgloss.Left, s.lspBlock(), s.mcpBlock()),
			"",
		),
	)
}

func (s *splashCmp) logoBlock() string {
	t := styles.CurrentTheme()
	return t.S().Base.Padding(0, 2).Width(s.width).Render(
		logo.Render(version.Version, false, logo.Opts{
			FieldColor:   t.Primary,
			TitleColorA:  t.Secondary,
			TitleColorB:  t.Primary,
			CharmColor:   t.Secondary,
			VersionColor: t.Primary,
			Width:        s.width - 4,
		}),
	)
}

func (s *splashCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	if cursor == nil {
		return nil
	}
	// Calculate the correct Y offset based on current state
	logoHeight := lipgloss.Height(s.logoRendered)
	if s.needsAPIKey {
		infoSectionHeight := lipgloss.Height(s.infoSection())
		baseOffset := logoHeight + SplashScreenPaddingY + infoSectionHeight
		remainingHeight := s.height - baseOffset - lipgloss.Height(s.apiKeyInput.View()) - SplashScreenPaddingY
		offset := baseOffset + remainingHeight
		cursor.Y += offset
		cursor.X = cursor.X + 1
	} else if s.isOnboarding {
		offset := logoHeight + SplashScreenPaddingY + s.logoGap() + 3
		cursor.Y += offset
		cursor.X = cursor.X + 1
	}

	return cursor
}

func (s *splashCmp) logoGap() int {
	if s.height > 35 {
		return LogoGap
	}
	return 0
}

// Bindings implements SplashPage.
func (s *splashCmp) Bindings() []key.Binding {
	if s.needsAPIKey {
		return []key.Binding{
			s.keyMap.Select,
			s.keyMap.Back,
		}
	} else if s.isOnboarding {
		return []key.Binding{
			s.keyMap.Select,
			s.keyMap.Next,
			s.keyMap.Previous,
		}
	} else if s.needsProjectInit {
		return []key.Binding{
			s.keyMap.Select,
			s.keyMap.Yes,
			s.keyMap.No,
			s.keyMap.Tab,
			s.keyMap.LeftRight,
		}
	}
	return []key.Binding{}
}

func (s *splashCmp) getMaxInfoWidth() int {
	return min(s.width-2, 40) // 2 for left padding
}

func (s *splashCmp) cwd() string {
	cwd := config.Get().WorkingDir()
	t := styles.CurrentTheme()
	homeDir, err := os.UserHomeDir()
	if err == nil && cwd != homeDir {
		cwd = strings.ReplaceAll(cwd, homeDir, "~")
	}
	maxWidth := s.getMaxInfoWidth()
	return t.S().Muted.Width(maxWidth).Render(cwd)
}

func LSPList(maxWidth int) []string {
	t := styles.CurrentTheme()
	lspList := []string{}
	lsp := config.Get().LSP.Sorted()
	if len(lsp) == 0 {
		return []string{t.S().Base.Foreground(t.Border).Render("None")}
	}
	for _, l := range lsp {
		iconColor := t.Success
		if l.LSP.Disabled {
			iconColor = t.FgMuted
		}
		lspList = append(lspList,
			core.Status(
				core.StatusOpts{
					IconColor:   iconColor,
					Title:       l.Name,
					Description: l.LSP.Command,
				},
				maxWidth,
			),
		)
	}
	return lspList
}

func (s *splashCmp) lspBlock() string {
	t := styles.CurrentTheme()
	maxWidth := s.getMaxInfoWidth() / 2
	section := t.S().Subtle.Render("LSPs")
	lspList := append([]string{section, ""}, LSPList(maxWidth-1)...)
	return t.S().Base.Width(maxWidth).PaddingRight(1).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			lspList...,
		),
	)
}

func MCPList(maxWidth int) []string {
	t := styles.CurrentTheme()
	mcpList := []string{}
	mcps := config.Get().MCP.Sorted()
	if len(mcps) == 0 {
		return []string{t.S().Base.Foreground(t.Border).Render("None")}
	}
	for _, l := range mcps {
		iconColor := t.Success
		if l.MCP.Disabled {
			iconColor = t.FgMuted
		}
		mcpList = append(mcpList,
			core.Status(
				core.StatusOpts{
					IconColor:   iconColor,
					Title:       l.Name,
					Description: l.MCP.Command,
				},
				maxWidth,
			),
		)
	}
	return mcpList
}

func (s *splashCmp) mcpBlock() string {
	t := styles.CurrentTheme()
	maxWidth := s.getMaxInfoWidth() / 2
	section := t.S().Subtle.Render("MCPs")
	mcpList := append([]string{section, ""}, MCPList(maxWidth-1)...)
	return t.S().Base.Width(maxWidth).PaddingRight(1).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			mcpList...,
		),
	)
}

func (s *splashCmp) IsShowingAPIKey() bool {
	return s.needsAPIKey
}
