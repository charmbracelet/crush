// Package models provides the model selection dialog for the TUI.
package models

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	hyperp "github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/claude"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/copilot"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/hyper"
	"github.com/charmbracelet/crush/internal/tui/exp/list"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	ModelsDialogID dialogs.DialogID = "models"

	defaultWidth = 60
)

const (
	LargeModelType int = iota
	SmallModelType

	largeModelInputPlaceholder = "Choose a model for large, complex tasks"
	smallModelInputPlaceholder = "Choose a model for small, simple tasks"
)

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model     config.SelectedModel
	ModelType config.SelectedModelType
}

// CloseModelDialogMsg is sent when a model is selected
type CloseModelDialogMsg struct{}

// ModelDialog interface for the model selection dialog
type ModelDialog interface {
	dialogs.DialogModel
}

type ModelOption struct {
	Provider catwalk.Provider
	Model    catwalk.Model
}

type modelDialogCmp struct {
	width   int
	wWidth  int
	wHeight int

	modelList *ModelListComponent
	keyMap    KeyMap
	help      help.Model

	// API key state
	needsAPIKey       bool
	apiKeyInput       *APIKeyInput
	selectedModel     *ModelOption
	selectedModelType config.SelectedModelType
	isAPIKeyValid     bool
	apiKeyValue       string

	// Hyper device flow state
	hyperDeviceFlow     *hyper.DeviceFlow
	showHyperDeviceFlow bool

	// Copilot device flow state
	copilotDeviceFlow     *copilot.DeviceFlow
	showCopilotDeviceFlow bool

	// Claude state
	claudeAuthMethodChooser     *claude.AuthMethodChooser
	claudeOAuth2                *claude.OAuth2
	showClaudeAuthMethodChooser bool
	showClaudeOAuth2            bool
}

func NewModelDialogCmp() ModelDialog {
	keyMap := DefaultKeyMap()

	listKeyMap := list.DefaultKeyMap()
	listKeyMap.Down.SetEnabled(false)
	listKeyMap.Up.SetEnabled(false)
	listKeyMap.DownOneItem = keyMap.Next
	listKeyMap.UpOneItem = keyMap.Previous

	t := styles.CurrentTheme()
	modelList := NewModelListComponent(listKeyMap, largeModelInputPlaceholder, true)
	apiKeyInput := NewAPIKeyInput()
	apiKeyInput.SetShowTitle(false)
	help := help.New()
	help.Styles = t.S().Help

	return &modelDialogCmp{
		modelList:   modelList,
		apiKeyInput: apiKeyInput,
		width:       defaultWidth,
		keyMap:      DefaultKeyMap(),
		help:        help,

		claudeAuthMethodChooser: claude.NewAuthMethodChooser(),
		claudeOAuth2:            claude.NewOAuth2(),
	}
}

func (m *modelDialogCmp) Init() tea.Cmd {
	return tea.Batch(
		m.modelList.Init(),
		m.apiKeyInput.Init(),
		m.claudeAuthMethodChooser.Init(),
		m.claudeOAuth2.Init(),
	)
}

func (m *modelDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.wWidth = msg.Width
		m.wHeight = msg.Height
		m.apiKeyInput.SetWidth(m.width - 2)
		m.help.SetWidth(m.width - 2)
		m.claudeAuthMethodChooser.SetWidth(m.width - 2)
		return m, m.modelList.SetSize(m.listWidth(), m.listHeight())
	case APIKeyStateChangeMsg:
		u, cmd := m.apiKeyInput.Update(msg)
		m.apiKeyInput = u.(*APIKeyInput)
		return m, cmd
	case hyper.DeviceFlowCompletedMsg:
		return m, m.saveOauthTokenAndContinue(msg.Token, true)
	case hyper.DeviceAuthInitiatedMsg, hyper.DeviceFlowErrorMsg:
		if m.hyperDeviceFlow != nil {
			u, cmd := m.hyperDeviceFlow.Update(msg)
			m.hyperDeviceFlow = u.(*hyper.DeviceFlow)
			return m, cmd
		}
		return m, nil
	case copilot.DeviceAuthInitiatedMsg, copilot.DeviceFlowErrorMsg:
		if m.copilotDeviceFlow != nil {
			u, cmd := m.copilotDeviceFlow.Update(msg)
			m.copilotDeviceFlow = u.(*copilot.DeviceFlow)
			return m, cmd
		}
		return m, nil
	case copilot.DeviceFlowCompletedMsg:
		return m, m.saveOauthTokenAndContinue(msg.Token, true)
	case claude.ValidationCompletedMsg:
		var cmds []tea.Cmd
		u, cmd := m.claudeOAuth2.Update(msg)
		m.claudeOAuth2 = u.(*claude.OAuth2)
		cmds = append(cmds, cmd)

		if msg.State == claude.OAuthValidationStateValid {
			cmds = append(cmds, m.saveOauthTokenAndContinue(msg.Token, false))
			m.keyMap.isClaudeOAuthHelpComplete = true
		}

		return m, tea.Batch(cmds...)
	case claude.AuthenticationCompleteMsg:
		return m, util.CmdHandler(dialogs.CloseDialogMsg{})
	case tea.KeyPressMsg:
		switch {
		// Handle Hyper device flow keys
		case key.Matches(msg, key.NewBinding(key.WithKeys("c", "C"))) && m.showHyperDeviceFlow:
			return m, m.hyperDeviceFlow.CopyCode()
		case key.Matches(msg, key.NewBinding(key.WithKeys("c", "C"))) && m.showCopilotDeviceFlow:
			return m, m.copilotDeviceFlow.CopyCode()
		case key.Matches(msg, key.NewBinding(key.WithKeys("c", "C"))) && m.showClaudeOAuth2 && m.claudeOAuth2.State == claude.OAuthStateURL:
			return m, tea.Sequence(
				tea.SetClipboard(m.claudeOAuth2.URL),
				func() tea.Msg {
					_ = clipboard.WriteAll(m.claudeOAuth2.URL)
					return nil
				},
				util.ReportInfo("URL copied to clipboard"),
			)
		case key.Matches(msg, m.keyMap.Choose) && m.showClaudeAuthMethodChooser:
			m.claudeAuthMethodChooser.ToggleChoice()
			return m, nil
		case key.Matches(msg, m.keyMap.Select):
			// If showing device flow, enter copies code and opens URL
			if m.showHyperDeviceFlow && m.hyperDeviceFlow != nil {
				return m, m.hyperDeviceFlow.CopyCodeAndOpenURL()
			}
			if m.showCopilotDeviceFlow && m.copilotDeviceFlow != nil {
				return m, m.copilotDeviceFlow.CopyCodeAndOpenURL()
			}
			selectedItem := m.modelList.SelectedModel()
			if selectedItem == nil {
				return m, nil
			}

			modelType := config.SelectedModelTypeLarge
			if m.modelList.GetModelType() == SmallModelType {
				modelType = config.SelectedModelTypeSmall
			}

			askForApiKey := func() {
				m.keyMap.isClaudeAuthChoiceHelp = false
				m.keyMap.isClaudeOAuthHelp = false
				m.keyMap.isAPIKeyHelp = true
				m.showHyperDeviceFlow = false
				m.showCopilotDeviceFlow = false
				m.showClaudeAuthMethodChooser = false
				m.needsAPIKey = true
				m.selectedModel = selectedItem
				m.selectedModelType = modelType
				m.apiKeyInput.SetProviderName(selectedItem.Provider.Name)
			}

			if m.showClaudeAuthMethodChooser {
				switch m.claudeAuthMethodChooser.State {
				case claude.AuthMethodAPIKey:
					askForApiKey()
				case claude.AuthMethodOAuth2:
					m.selectedModel = selectedItem
					m.selectedModelType = modelType
					m.showClaudeAuthMethodChooser = false
					m.showClaudeOAuth2 = true
					m.keyMap.isClaudeAuthChoiceHelp = false
					m.keyMap.isClaudeOAuthHelp = true
				}
				return m, nil
			}
			if m.showClaudeOAuth2 {
				m2, cmd2 := m.claudeOAuth2.ValidationConfirm()
				m.claudeOAuth2 = m2.(*claude.OAuth2)
				return m, cmd2
			}
			if m.isAPIKeyValid {
				return m, m.saveOauthTokenAndContinue(m.apiKeyValue, true)
			}
			if m.needsAPIKey {
				// Handle API key submission
				m.apiKeyValue = m.apiKeyInput.Value()
				provider, err := m.getProvider(m.selectedModel.Provider.ID)
				if err != nil || provider == nil {
					return m, util.ReportError(fmt.Errorf("provider %s not found", m.selectedModel.Provider.ID))
				}
				providerConfig := config.ProviderConfig{
					ID:      string(m.selectedModel.Provider.ID),
					Name:    m.selectedModel.Provider.Name,
					APIKey:  m.apiKeyValue,
					Type:    provider.Type,
					BaseURL: provider.APIEndpoint,
				}
				return m, tea.Sequence(
					util.CmdHandler(APIKeyStateChangeMsg{
						State: APIKeyInputStateVerifying,
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
							m.isAPIKeyValid = true
							return APIKeyStateChangeMsg{
								State: APIKeyInputStateVerified,
							}
						}
						return APIKeyStateChangeMsg{
							State: APIKeyInputStateError,
						}
					},
				)
			}

			// Check if provider is configured
			if m.isProviderConfigured(string(selectedItem.Provider.ID)) {
				return m, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(ModelSelectedMsg{
						Model: config.SelectedModel{
							Model:           selectedItem.Model.ID,
							Provider:        string(selectedItem.Provider.ID),
							ReasoningEffort: selectedItem.Model.DefaultReasoningEffort,
							MaxTokens:       selectedItem.Model.DefaultMaxTokens,
						},
						ModelType: modelType,
					}),
				)
			}
			switch selectedItem.Provider.ID {
			case catwalk.InferenceProviderAnthropic:
				m.showClaudeAuthMethodChooser = true
				m.keyMap.isClaudeAuthChoiceHelp = true
				return m, nil
			case hyperp.Name:
				m.showHyperDeviceFlow = true
				m.selectedModel = selectedItem
				m.selectedModelType = modelType
				m.hyperDeviceFlow = hyper.NewDeviceFlow()
				m.hyperDeviceFlow.SetWidth(m.width - 2)
				return m, m.hyperDeviceFlow.Init()
			case catwalk.InferenceProviderCopilot:
				if token, ok := config.Get().ImportCopilot(); ok {
					m.selectedModel = selectedItem
					m.selectedModelType = modelType
					return m, m.saveOauthTokenAndContinue(token, true)
				}
				m.showCopilotDeviceFlow = true
				m.selectedModel = selectedItem
				m.selectedModelType = modelType
				m.copilotDeviceFlow = copilot.NewDeviceFlow()
				m.copilotDeviceFlow.SetWidth(m.width - 2)
				return m, m.copilotDeviceFlow.Init()
			}
			// For other providers, show API key input
			askForApiKey()
			return m, nil
		case key.Matches(msg, m.keyMap.Tab):
			switch {
			case m.showClaudeAuthMethodChooser:
				m.claudeAuthMethodChooser.ToggleChoice()
				return m, nil
			case m.needsAPIKey:
				u, cmd := m.apiKeyInput.Update(msg)
				m.apiKeyInput = u.(*APIKeyInput)
				return m, cmd
			case m.modelList.GetModelType() == LargeModelType:
				m.modelList.SetInputPlaceholder(smallModelInputPlaceholder)
				return m, m.modelList.SetModelType(SmallModelType)
			default:
				m.modelList.SetInputPlaceholder(largeModelInputPlaceholder)
				return m, m.modelList.SetModelType(LargeModelType)
			}
		case key.Matches(msg, m.keyMap.Close):
			switch {
			case m.showHyperDeviceFlow:
				if m.hyperDeviceFlow != nil {
					m.hyperDeviceFlow.Cancel()
				}
				m.showHyperDeviceFlow = false
				m.selectedModel = nil
			case m.showCopilotDeviceFlow:
				if m.copilotDeviceFlow != nil {
					m.copilotDeviceFlow.Cancel()
				}
				m.showCopilotDeviceFlow = false
				m.selectedModel = nil
			case m.showClaudeAuthMethodChooser:
				m.claudeAuthMethodChooser.SetDefaults()
				m.showClaudeAuthMethodChooser = false
				m.keyMap.isClaudeAuthChoiceHelp = false
				m.keyMap.isClaudeOAuthHelp = false
				return m, nil
			case m.needsAPIKey:
				if m.isAPIKeyValid {
					return m, nil
				}
				// Go back to model selection
				m.needsAPIKey = false
				m.selectedModel = nil
				m.isAPIKeyValid = false
				m.apiKeyValue = ""
				m.apiKeyInput.Reset()
				return m, nil
			default:
				return m, util.CmdHandler(dialogs.CloseDialogMsg{})
			}
		default:
			switch {
			case m.showClaudeAuthMethodChooser:
				u, cmd := m.claudeAuthMethodChooser.Update(msg)
				m.claudeAuthMethodChooser = u.(*claude.AuthMethodChooser)
				return m, cmd
			case m.showClaudeOAuth2:
				u, cmd := m.claudeOAuth2.Update(msg)
				m.claudeOAuth2 = u.(*claude.OAuth2)
				return m, cmd
			case m.needsAPIKey:
				u, cmd := m.apiKeyInput.Update(msg)
				m.apiKeyInput = u.(*APIKeyInput)
				return m, cmd
			default:
				u, cmd := m.modelList.Update(msg)
				m.modelList = u
				return m, cmd
			}
		}
	case tea.PasteMsg:
		switch {
		case m.showClaudeOAuth2:
			u, cmd := m.claudeOAuth2.Update(msg)
			m.claudeOAuth2 = u.(*claude.OAuth2)
			return m, cmd
		case m.needsAPIKey:
			u, cmd := m.apiKeyInput.Update(msg)
			m.apiKeyInput = u.(*APIKeyInput)
			return m, cmd
		default:
			var cmd tea.Cmd
			m.modelList, cmd = m.modelList.Update(msg)
			return m, cmd
		}
	case spinner.TickMsg:
		u, cmd := m.apiKeyInput.Update(msg)
		m.apiKeyInput = u.(*APIKeyInput)
		if m.showHyperDeviceFlow && m.hyperDeviceFlow != nil {
			u, cmd = m.hyperDeviceFlow.Update(msg)
			m.hyperDeviceFlow = u.(*hyper.DeviceFlow)
		}
		if m.showCopilotDeviceFlow && m.copilotDeviceFlow != nil {
			u, cmd = m.copilotDeviceFlow.Update(msg)
			m.copilotDeviceFlow = u.(*copilot.DeviceFlow)
		}
		return m, cmd
	default:
		// Pass all other messages to the device flow for spinner animation
		switch {
		case m.showHyperDeviceFlow && m.hyperDeviceFlow != nil:
			u, cmd := m.hyperDeviceFlow.Update(msg)
			m.hyperDeviceFlow = u.(*hyper.DeviceFlow)
			return m, cmd
		case m.showCopilotDeviceFlow && m.copilotDeviceFlow != nil:
			u, cmd := m.copilotDeviceFlow.Update(msg)
			m.copilotDeviceFlow = u.(*copilot.DeviceFlow)
			return m, cmd
		case m.showClaudeOAuth2:
			u, cmd := m.claudeOAuth2.Update(msg)
			m.claudeOAuth2 = u.(*claude.OAuth2)
			return m, cmd
		default:
			u, cmd := m.apiKeyInput.Update(msg)
			m.apiKeyInput = u.(*APIKeyInput)
			return m, cmd
		}
	}
	return m, nil
}

func (m *modelDialogCmp) View() string {
	t := styles.CurrentTheme()

	if m.showHyperDeviceFlow && m.hyperDeviceFlow != nil {
		// Show Hyper device flow
		m.keyMap.isHyperDeviceFlow = true
		deviceFlowView := m.hyperDeviceFlow.View()
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Authenticate with Hyper", m.width-4)),
			deviceFlowView,
			"",
			t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
		)
		return m.style().Render(content)
	}
	if m.showCopilotDeviceFlow && m.copilotDeviceFlow != nil {
		// Show Hyper device flow
		m.keyMap.isCopilotDeviceFlow = m.copilotDeviceFlow.State != copilot.DeviceFlowStateUnavailable
		m.keyMap.isCopilotUnavailable = m.copilotDeviceFlow.State == copilot.DeviceFlowStateUnavailable
		deviceFlowView := m.copilotDeviceFlow.View()
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Authenticate with GitHub Copilot", m.width-4)),
			deviceFlowView,
			"",
			t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
		)
		return m.style().Render(content)
	}

	// Reset the flags when not showing device flow
	m.keyMap.isHyperDeviceFlow = false
	m.keyMap.isCopilotDeviceFlow = false
	m.keyMap.isCopilotUnavailable = false

	switch {
	case m.showClaudeAuthMethodChooser:
		chooserView := m.claudeAuthMethodChooser.View()
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Let's Auth Anthropic", m.width-4)),
			chooserView,
			"",
			t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
		)
		return m.style().Render(content)
	case m.showClaudeOAuth2:
		m.keyMap.isClaudeOAuthURLState = m.claudeOAuth2.State == claude.OAuthStateURL
		oauth2View := m.claudeOAuth2.View()
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Let's Auth Anthropic", m.width-4)),
			oauth2View,
			"",
			t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
		)
		return m.style().Render(content)
	case m.needsAPIKey:
		// Show API key input
		m.keyMap.isAPIKeyHelp = true
		m.keyMap.isAPIKeyValid = m.isAPIKeyValid
		apiKeyView := m.apiKeyInput.View()
		apiKeyView = t.S().Base.Width(m.width - 3).Height(lipgloss.Height(apiKeyView)).PaddingLeft(1).Render(apiKeyView)
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			t.S().Base.Padding(0, 1, 1, 1).Render(core.Title(m.apiKeyInput.GetTitle(), m.width-4)),
			apiKeyView,
			"",
			t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
		)
		return m.style().Render(content)
	}

	// Show model selection
	listView := m.modelList.View()
	radio := m.modelTypeRadio()
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		t.S().Base.Padding(0, 1, 1, 1).Render(core.Title("Switch Model", m.width-lipgloss.Width(radio)-5)+" "+radio),
		listView,
		"",
		t.S().Base.Width(m.width-2).PaddingLeft(1).AlignHorizontal(lipgloss.Left).Render(m.help.View(m.keyMap)),
	)
	return m.style().Render(content)
}

func (m *modelDialogCmp) Cursor() *tea.Cursor {
	if m.showHyperDeviceFlow && m.hyperDeviceFlow != nil {
		return m.hyperDeviceFlow.Cursor()
	}
	if m.showCopilotDeviceFlow && m.copilotDeviceFlow != nil {
		return m.copilotDeviceFlow.Cursor()
	}
	if m.showClaudeAuthMethodChooser {
		return nil
	}
	if m.showClaudeOAuth2 {
		if cursor := m.claudeOAuth2.CodeInput.Cursor(); cursor != nil {
			cursor.Y += 2 // FIXME(@andreynering): Why do we need this?
			return m.moveCursor(cursor)
		}
		return nil
	}
	if m.needsAPIKey {
		cursor := m.apiKeyInput.Cursor()
		if cursor != nil {
			cursor = m.moveCursor(cursor)
			return cursor
		}
	} else {
		cursor := m.modelList.Cursor()
		if cursor != nil {
			cursor = m.moveCursor(cursor)
			return cursor
		}
	}
	return nil
}

func (m *modelDialogCmp) style() lipgloss.Style {
	t := styles.CurrentTheme()
	return t.S().Base.
		Width(m.width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
}

func (m *modelDialogCmp) listWidth() int {
	return m.width - 2
}

func (m *modelDialogCmp) listHeight() int {
	return m.wHeight / 2
}

func (m *modelDialogCmp) Position() (int, int) {
	row := m.wHeight/4 - 2 // just a bit above the center
	col := m.wWidth / 2
	col -= m.width / 2
	return row, col
}

func (m *modelDialogCmp) moveCursor(cursor *tea.Cursor) *tea.Cursor {
	row, col := m.Position()
	if m.needsAPIKey {
		offset := row + 3 // Border + title + API key input offset
		cursor.Y += offset
		cursor.X = cursor.X + col + 2
	} else {
		offset := row + 3 // Border + title
		cursor.Y += offset
		cursor.X = cursor.X + col + 2
	}
	return cursor
}

func (m *modelDialogCmp) ID() dialogs.DialogID {
	return ModelsDialogID
}

func (m *modelDialogCmp) modelTypeRadio() string {
	t := styles.CurrentTheme()
	choices := []string{"Large Task", "Small Task"}
	iconSelected := "◉"
	iconUnselected := "○"
	if m.modelList.GetModelType() == LargeModelType {
		return t.S().Base.Foreground(t.FgHalfMuted).Render(iconSelected + " " + choices[0] + "  " + iconUnselected + " " + choices[1])
	}
	return t.S().Base.Foreground(t.FgHalfMuted).Render(iconUnselected + " " + choices[0] + "  " + iconSelected + " " + choices[1])
}

func (m *modelDialogCmp) isProviderConfigured(providerID string) bool {
	cfg := config.Get()
	_, ok := cfg.Providers.Get(providerID)
	return ok
}

func (m *modelDialogCmp) getProvider(providerID catwalk.InferenceProvider) (*catwalk.Provider, error) {
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

func (m *modelDialogCmp) saveOauthTokenAndContinue(apiKey any, close bool) tea.Cmd {
	if m.selectedModel == nil {
		return util.ReportError(fmt.Errorf("no model selected"))
	}

	cfg := config.Get()
	err := cfg.SetProviderAPIKey(string(m.selectedModel.Provider.ID), apiKey)
	if err != nil {
		return util.ReportError(fmt.Errorf("failed to save API key: %w", err))
	}

	// Reset API key state and continue with model selection
	selectedModel := *m.selectedModel
	var cmds []tea.Cmd
	if close {
		cmds = append(cmds, util.CmdHandler(dialogs.CloseDialogMsg{}))
	}
	cmds = append(
		cmds,
		util.CmdHandler(ModelSelectedMsg{
			Model: config.SelectedModel{
				Model:           selectedModel.Model.ID,
				Provider:        string(selectedModel.Provider.ID),
				ReasoningEffort: selectedModel.Model.DefaultReasoningEffort,
				MaxTokens:       selectedModel.Model.DefaultMaxTokens,
			},
			ModelType: m.selectedModelType,
		}),
	)
	return tea.Sequence(cmds...)
}
