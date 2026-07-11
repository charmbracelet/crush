package dialog

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/discover"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/exp/charmtone"
)

// LMStudioSetupID is the identifier for the LM Studio setup dialog.
const LMStudioSetupID = "lmstudio_setup"

const defaultLMStudioSetupWidth = 70

type LMStudioSetupState int

const (
	LMStudioSetupStateInitial LMStudioSetupState = iota
	LMStudioSetupStateVerifying
	LMStudioSetupStateError
)

type lmStudioSetupField int

const (
	lmStudioBaseURLField lmStudioSetupField = iota
	lmStudioAPIKeyField
)

// LMStudioSetup configures a custom LM Studio provider by discovering models
// before the provider is saved to the global config.
type LMStudioSetup struct {
	com          *common.Common
	isOnboarding bool

	width       int
	state       LMStudioSetupState
	activeField lmStudioSetupField
	errText     string

	keyMap struct {
		Submit key.Binding
		Next   key.Binding
		Close  key.Binding
	}
	baseURLInput textinput.Model
	apiKeyInput  textinput.Model
	spinner      spinner.Model
	help         help.Model
}

var _ Dialog = (*LMStudioSetup)(nil)

// NewLMStudioSetup creates the custom LM Studio setup dialog.
func NewLMStudioSetup(com *common.Common, isOnboarding bool) (*LMStudioSetup, tea.Cmd) {
	t := com.Styles
	m := LMStudioSetup{
		com:          com,
		isOnboarding: isOnboarding,
		width:        defaultLMStudioSetupWidth,
	}

	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2

	m.baseURLInput = textinput.New()
	m.baseURLInput.SetVirtualCursor(false)
	m.baseURLInput.Placeholder = "http://127.0.0.1:1234/v1"
	m.baseURLInput.SetStyles(com.Styles.TextInput)
	m.baseURLInput.SetWidth(max(0, innerWidth-1))
	m.baseURLInput.Focus()

	m.apiKeyInput = textinput.New()
	m.apiKeyInput.SetVirtualCursor(false)
	m.apiKeyInput.Placeholder = "API key (optional)"
	m.apiKeyInput.SetStyles(com.Styles.TextInput)
	m.apiKeyInput.SetWidth(max(0, innerWidth-1))
	m.apiKeyInput.Blur()

	if provider, ok := com.Config().Providers.Get(config.LMStudioProviderID); ok {
		m.baseURLInput.SetValue(provider.BaseURL)
		m.apiKeyInput.SetValue(provider.APIKey)
	} else if config.EmbeddedLMStudioBaseURL != "" {
		m.baseURLInput.SetValue(config.EmbeddedLMStudioBaseURL)
	}

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Dialog.APIKey.Spinner),
	)

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "save"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("tab", "up", "down"),
		key.WithHelp("tab", "field"),
	)
	m.keyMap.Close = CloseKey

	return &m, nil
}

// ID implements Dialog.
func (m *LMStudioSetup) ID() string {
	return LMStudioSetupID
}

// HandleMsg implements Dialog.
func (m *LMStudioSetup) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case ActionChangeLMStudioSetupState:
		if msg.State == LMStudioSetupStateVerifying {
			return m.startVerifying()
		}
		m.state = msg.State
	case ActionLMStudioSetupResult:
		if msg.Error != nil {
			m.state = LMStudioSetupStateError
			m.errText = msg.Error.Error()
			m.syncFocus()
			return nil
		}
		return ActionConfigureLMStudio{
			Provider: msg.Provider,
			Model:    msg.Model,
		}
	case spinner.TickMsg:
		if m.state == LMStudioSetupStateVerifying {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.KeyPressMsg:
		switch {
		case m.state == LMStudioSetupStateVerifying:
			return nil
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Next):
			m.toggleField()
		case key.Matches(msg, m.keyMap.Submit):
			if strings.TrimSpace(m.baseURLInput.Value()) == "" {
				m.state = LMStudioSetupStateError
				m.errText = "base URL is required"
				m.activeField = lmStudioBaseURLField
				m.syncFocus()
				return nil
			}
			return m.startVerifying()
		default:
			return ActionCmd{m.updateActiveInput(msg)}
		}
	case tea.PasteMsg:
		if m.state != LMStudioSetupStateVerifying {
			return ActionCmd{m.updateActiveInput(msg)}
		}
	}
	return nil
}

func (m *LMStudioSetup) startVerifying() Action {
	m.state = LMStudioSetupStateVerifying
	m.errText = ""
	m.baseURLInput.Blur()
	m.apiKeyInput.Blur()
	return ActionCmd{tea.Batch(m.spinner.Tick, m.verifyProvider)}
}

func (m *LMStudioSetup) updateActiveInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.activeField {
	case lmStudioAPIKeyField:
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	default:
		m.baseURLInput, cmd = m.baseURLInput.Update(msg)
	}
	return cmd
}

func (m *LMStudioSetup) toggleField() {
	if m.activeField == lmStudioBaseURLField {
		m.activeField = lmStudioAPIKeyField
	} else {
		m.activeField = lmStudioBaseURLField
	}
	m.syncFocus()
}

func (m *LMStudioSetup) syncFocus() {
	if m.state == LMStudioSetupStateVerifying {
		m.baseURLInput.Blur()
		m.apiKeyInput.Blur()
		return
	}
	if m.activeField == lmStudioAPIKeyField {
		m.baseURLInput.Blur()
		m.apiKeyInput.Focus()
		return
	}
	m.apiKeyInput.Blur()
	m.baseURLInput.Focus()
}

func (m *LMStudioSetup) verifyProvider() tea.Msg {
	baseURL := normalizeLMStudioBaseURL(m.baseURLInput.Value())
	apiKey := strings.TrimSpace(m.apiKeyInput.Value())

	discoverModels := true
	provider := config.ProviderConfig{
		ID:                 config.LMStudioProviderID,
		Name:               config.LMStudioProviderName,
		BaseURL:            baseURL,
		APIKey:             apiKey,
		Type:               catwalk.Type("lmstudio"),
		AutoDiscoverModels: &discoverModels,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := discover.DiscoverModels(ctx, discover.Config{
		ID:      provider.ID,
		BaseURL: provider.BaseURL,
		APIKey:  provider.APIKey,
	}, m.com.Workspace.Resolver())
	if err != nil {
		return ActionLMStudioSetupResult{Error: err}
	}

	if enricher := discover.GetEnricher(string(provider.Type)); enricher != nil {
		if enriched, enrichErr := enricher.EnrichModels(ctx, discover.Config{
			ID:             provider.ID,
			BaseURL:        provider.BaseURL,
			APIKey:         provider.APIKey,
			ExistingModels: models,
		}, m.com.Workspace.Resolver(), models); enrichErr == nil {
			models = enriched
		}
	}

	if len(models) == 0 {
		return ActionLMStudioSetupResult{Error: fmt.Errorf("LM Studio returned no models")}
	}

	provider.Models = models
	return ActionLMStudioSetupResult{
		Provider: provider,
		Model:    selectedLMStudioModel(models[0]),
	}
}

func selectedLMStudioModel(model catwalk.Model) config.SelectedModel {
	return config.SelectedModel{
		Provider:        config.LMStudioProviderID,
		Model:           model.ID,
		ReasoningEffort: model.DefaultReasoningEffort,
		Think:           model.CanReason && len(model.ReasoningLevels) == 0,
		MaxTokens:       model.DefaultMaxTokens,
	}
}

func normalizeLMStudioBaseURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}

	parsed, err := url.Parse(value)
	if err == nil && parsed.Host != "" {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
		if parsed.Path == "" {
			parsed.Path = "/v1"
		} else if !strings.HasSuffix(parsed.Path, "/v1") {
			parsed.Path += "/v1"
		}
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String()
	}

	value = strings.TrimRight(value, "/")
	if !strings.HasSuffix(value, "/v1") {
		value += "/v1"
	}
	return value
}

// Draw implements Dialog.
func (m *LMStudioSetup) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(m.width, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	m.baseURLInput.SetWidth(max(0, innerWidth-1))
	m.apiKeyInput.SetWidth(max(0, innerWidth-1))
	m.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "Configure LM Studio"
	rc.AddPart(m.introView())
	rc.AddPart(m.fieldView("Base URL", m.baseURLInput))
	rc.AddPart(m.fieldView("API Key", m.apiKeyInput))
	if m.errText != "" {
		rc.AddPart(t.Dialog.TitleError.Render(m.errText))
	}
	rc.Help = m.help.View(m)
	if m.state == LMStudioSetupStateVerifying {
		rc.Help = m.spinner.View() + " Checking LM Studio models..."
	}

	if m.isOnboarding {
		rc.Title = ""
		rc.IsOnboarding = true
	}

	view := rc.Render()
	cur := m.Cursor()
	if m.isOnboarding {
		cur = adjustOnboardingInputCursor(t, cur)
		DrawOnboardingCursor(scr, area, view, cur)
		return cur
	}
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *LMStudioSetup) introView() string {
	style := m.com.Styles.Dialog.SecondaryText
	if m.state == LMStudioSetupStateError {
		style = style.Foreground(charmtone.Cherry)
	}
	return style.Render("Paste the OpenAI-compatible LM Studio endpoint. The API key can be empty.")
}

func (m *LMStudioSetup) fieldView(label string, input textinput.Model) string {
	t := m.com.Styles
	labelView := t.Dialog.SecondaryText.Render(label)
	inputView := input.View()
	return lipgloss.JoinVertical(lipgloss.Left, labelView, inputView)
}

// Cursor returns the cursor position relative to the dialog.
func (m *LMStudioSetup) Cursor() *tea.Cursor {
	if m.state == LMStudioSetupStateVerifying {
		return nil
	}

	var cur *tea.Cursor
	if m.activeField == lmStudioAPIKeyField {
		cur = m.apiKeyInput.Cursor()
	} else {
		cur = m.baseURLInput.Cursor()
	}
	if cur == nil {
		return nil
	}

	cur = InputCursor(m.com.Styles, cur)
	cur.Y += lipgloss.Height(m.introView()) + 1
	if m.activeField == lmStudioAPIKeyField {
		cur.Y += lipgloss.Height(m.fieldView("Base URL", m.baseURLInput))
	}
	return cur
}

// ShortHelp implements help.KeyMap.
func (m *LMStudioSetup) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.Next,
		m.keyMap.Submit,
		m.keyMap.Close,
	}
}

// FullHelp implements help.KeyMap.
func (m *LMStudioSetup) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

func isLMStudioSetupModelItem(item *ModelItem) bool {
	return item != nil && string(item.prov.ID) == lmStudioSetupProviderID
}

const (
	lmStudioSetupProviderID = "__configure_lmstudio__"
	lmStudioSetupModelID    = "__configure_lmstudio__"
)

func newLMStudioSetupModelItem(t *styles.Styles) *ModelItem {
	return NewModelItem(t, catwalk.Provider{
		ID:   catwalk.InferenceProvider(lmStudioSetupProviderID),
		Name: "Local",
		Type: catwalk.Type("lmstudio"),
	}, catwalk.Model{
		ID:   lmStudioSetupModelID,
		Name: "Configure LM Studio",
	}, ModelTypeLarge, false)
}
