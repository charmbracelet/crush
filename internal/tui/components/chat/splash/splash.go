package splash

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/core/layout"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/models"
	"github.com/charmbracelet/crush/internal/tui/components/logo"
	lspcomponent "github.com/charmbracelet/crush/internal/tui/components/lsp"
	"github.com/charmbracelet/crush/internal/tui/components/mcp"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/crush/internal/version"
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

	// IsAPIKeyValid returns whether the API key is valid
	IsAPIKeyValid() bool
}

const (
	SplashScreenPaddingY = 1 // Padding Y for the splash screen

	LogoGap = 6
)

// OnboardingCompleteMsg is sent when onboarding is complete
type (
	OnboardingCompleteMsg struct{}
	SubmitAPIKeyMsg       struct{}
)

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
	isAPIKeyValid bool
	apiKeyValue   string
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

	modelList := models.NewModelListComponent(listKeyMap, "Find your fave", false)
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
	wasSmallScreen := s.isSmallScreen()
	rerenderLogo := width != s.width
	s.height = height
	s.width = width
	if rerenderLogo || wasSmallScreen != s.isSmallScreen() {
		s.logoRendered = s.logoBlock()
	}
	// remove padding, logo height, gap, title space
	s.listHeight = s.height - lipgloss.Height(s.logoRendered) - (SplashScreenPaddingY * 2) - s.logoGap() - 2
	listWidth := min(60, width)
	s.apiKeyInput.SetWidth(width - 2)
	return s.modelList.SetSize(listWidth, s.listHeight)
}

// Update implements SplashPage.
func (s *splashCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return s, s.SetSize(msg.Width, msg.Height)
	case models.APIKeyStateChangeMsg:
		u, cmd := s.apiKeyInput.Update(msg)
		s.apiKeyInput = u.(*models.APIKeyInput)
		if msg.State == models.APIKeyInputStateVerified {
			return s, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
				return SubmitAPIKeyMsg{}
			})
		}
		return s, cmd
	case SubmitAPIKeyMsg:
		if s.isAPIKeyValid {
			return s, s.saveAPIKeyAndContinue(s.apiKeyValue)
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Back):
			if s.isAPIKeyValid {
				return s, nil
			}
			if s.needsAPIKey {
				// Go back to model selection
				s.needsAPIKey = false
				s.selectedModel = nil
				s.isAPIKeyValid = false
				s.apiKeyValue = ""
				s.apiKeyInput.Reset()
				return s, nil
			}
		case key.Matches(msg, s.keyMap.Select):
			if s.isAPIKeyValid {
				return s, s.saveAPIKeyAndContinue(s.apiKeyValue)
			}
			if s.isOnboarding && !s.needsAPIKey {
				selectedItem := s.modelList.SelectedModel()
				if selectedItem == nil {
					return s, nil
				}
				if s.isProviderConfigured(string(selectedItem.Provider.ID)) {
					cmd := s.setPreferredModel(*selectedItem)
					s.isOnboarding = false
					return s, tea.Batch(cmd, util.CmdHandler(OnboardingCompleteMsg{}))
				} else {
					// Provider not configured, show API key input
					s.needsAPIKey = true
					s.selectedModel = selectedItem
					s.apiKeyInput.SetProviderName(selectedItem.Provider.Name)
					return s, nil
				}
			} else if s.needsAPIKey {
				// Handle API key submission
				s.apiKeyValue = strings.TrimSpace(s.apiKeyInput.Value())
				if s.apiKeyValue == "" {
					return s, nil
				}

				provider, err := s.getProvider(s.selectedModel.Provider.ID)
				if err != nil || provider == nil {
					return s, util.ReportError(fmt.Errorf("provider %s not found", s.selectedModel.Provider.ID))
				}
				providerConfig := config.ProviderConfig{
					ID:      string(s.selectedModel.Provider.ID),
					Name:    s.selectedModel.Provider.Name,
					APIKey:  s.apiKeyValue,
					Type:    provider.Type,
					BaseURL: provider.APIEndpoint,
				}
				return s, tea.Sequence(
					util.CmdHandler(models.APIKeyStateChangeMsg{
						State: models.APIKeyInputStateVerifying,
					}),
					func() tea.Msg {
						start := time.Now()
						err := providerConfig.TestConnection(config.Get().Resolver())
						// intentionally wait for at least 750ms to make sure the user sees the spinner
						elapsed := time.Since(start)
						if elapsed < 750*time.Millisecond {
							time.Sleep(750*time.Millisecond - elapsed)
						}
						if err == nil {
							s.isAPIKeyValid = true
							return models.APIKeyStateChangeMsg{
								State: models.APIKeyInputStateVerified,
							}
						}
						return models.APIKeyStateChangeMsg{
							State: models.APIKeyInputStateError,
						}
					},
				)
			} else if s.needsProjectInit {
				return s, s.initializeProject()
			}
		case key.Matches(msg, s.keyMap.Tab, s.keyMap.LeftRight):
			if s.needsAPIKey {
				u, cmd := s.apiKeyInput.Update(msg)
				s.apiKeyInput = u.(*models.APIKeyInput)
				return s, cmd
			}
			if s.needsProjectInit {
				s.selectedNo = !s.selectedNo
				return s, nil
			}
		case key.Matches(msg, s.keyMap.Yes):
			if s.needsAPIKey {
				u, cmd := s.apiKeyInput.Update(msg)
				s.apiKeyInput = u.(*models.APIKeyInput)
				return s, cmd
			}
			if s.isOnboarding {
				u, cmd := s.modelList.Update(msg)
				s.modelList = u
				return s, cmd
			}
			if s.needsProjectInit {
				s.selectedNo = false
				return s, s.initializeProject()
			}
		case key.Matches(msg, s.keyMap.No):
			if s.needsAPIKey {
				u, cmd := s.apiKeyInput.Update(msg)
				s.apiKeyInput = u.(*models.APIKeyInput)
				return s, cmd
			}
			if s.isOnboarding {
				u, cmd := s.modelList.Update(msg)
				s.modelList = u
				return s, cmd
			}
			if s.needsProjectInit {
				s.selectedNo = true
				return s, s.initializeProject()
			}
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
	case spinner.TickMsg:
		u, cmd := s.apiKeyInput.Update(msg)
		s.apiKeyInput = u.(*models.APIKeyInput)
		return s, cmd
	}
	return s, nil
}

func (s *splashCmp) saveAPIKeyAndContinue(apiKey string) tea.Cmd {
	if s.selectedModel == nil {
		return nil
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
	s.isAPIKeyValid = false

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
				Text: agent.InitializePrompt(),
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

func (s *splashCmp) getProvider(providerID catwalk.InferenceProvider) (*catwalk.Provider, error) {
	cfg := config.Get()
	providers, err := config.Providers(cfg)
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
	if _, ok := cfg.Providers.Get(providerID); ok {
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
		pathStyle := t.S().Base.Foreground(t.Success).PaddingLeft(2)
		bodyStyle := t.S().Base.Foreground(t.FgMuted)
		shortcutStyle := t.S().Base.Foreground(t.Success)

		initText := lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render("Would you like to initialize this project?"),
			"",
			pathStyle.Render(s.cwd()),
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
		remainingHeight := s.height - lipgloss.Height(s.logoRendered) - (SplashScreenPaddingY * 2)

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
			"",
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

func (s *splashCmp) isSmallScreen() bool {
	// Consider a screen small if either the width is less than 40 or if the
	// height is less than 20
	return s.width < 55 || s.height < 20
}

func (s *splashCmp) infoSection() string {
	t := styles.CurrentTheme()
	infoStyle := t.S().Base.PaddingLeft(2)
	if s.isSmallScreen() {
		infoStyle = infoStyle.MarginTop(1)
	}
	return infoStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			s.cwdPart(),
			"",
			s.currentModelBlock(),
			"",
			lipgloss.JoinHorizontal(lipgloss.Left, s.lspBlock(), s.mcpBlock()),
			"",
		),
	)
}

func (s *splashCmp) logoBlock() string {
	t := styles.CurrentTheme()
	logoStyle := t.S().Base.Padding(0, 2).Width(s.width)
	if s.isSmallScreen() {
		// If the width is too small, render a smaller version of the logo
		// NOTE: 20 is not correct because [splashCmp.height] is not the
		// *actual* window height, instead, it is the height of the splash
		// component and that depends on other variables like compact mode and
		// the height of the editor.
		return logoStyle.Render(
			logo.SmallRender(s.width - logoStyle.GetHorizontalFrameSize()),
		)
	}
	return logoStyle.Render(
		logo.Render(version.Version, false, logo.Opts{
			FieldColor:   t.Primary,
			TitleColorA:  t.Secondary,
			TitleColorB:  t.Primary,
			CharmColor:   t.Secondary,
			VersionColor: t.Primary,
			Width:        s.width - logoStyle.GetHorizontalFrameSize(),
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
		offset := logoHeight + SplashScreenPaddingY + s.logoGap() + 2
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
	return min(s.width-2, 90) // 2 for left padding
}

func (s *splashCmp) cwdPart() string {
	t := styles.CurrentTheme()
	maxWidth := s.getMaxInfoWidth()
	return t.S().Muted.Width(maxWidth).Render(s.cwd())
}

func (s *splashCmp) cwd() string {
	return home.Short(config.Get().WorkingDir())
}

func LSPList(maxWidth int) []string {
	return lspcomponent.RenderLSPList(nil, lspcomponent.RenderOptions{
		MaxWidth:    maxWidth,
		ShowSection: false,
	})
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
	return mcp.RenderMCPList(mcp.RenderOptions{
		MaxWidth:    maxWidth,
		ShowSection: false,
	})
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

func (s *splashCmp) currentModelBlock() string {
	cfg := config.Get()
	agentCfg := cfg.Agents[config.AgentCoder]
	model := config.Get().GetModelByType(agentCfg.Model)
	if model == nil {
		return ""
	}
	t := styles.CurrentTheme()
	modelIcon := t.S().Base.Foreground(t.FgSubtle).Render(styles.ModelIcon)
	modelName := t.S().Text.Render(model.Name)
	modelInfo := fmt.Sprintf("%s %s", modelIcon, modelName)
	parts := []string{
		modelInfo,
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		parts...,
	)
}

func (s *splashCmp) IsShowingAPIKey() bool {
	return s.needsAPIKey
}

func (s *splashCmp) IsAPIKeyValid() bool {
	return s.isAPIKeyValid
}
