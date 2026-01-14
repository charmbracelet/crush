package dialog

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/uiutil"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/exp/charmtone"
)

type APIKeyInputState int

const (
	APIKeyInputStateInitial APIKeyInputState = iota
	APIKeyInputStateVerifying
	APIKeyInputStateVerified
	APIKeyInputStateError
)

// APIKeyInputID is the identifier for the model selection dialog.
const APIKeyInputID = "api_key_input"

// APIKeyInput represents a model selection dialog.
type APIKeyInput struct {
	com *common.Common

	provider  catwalk.Provider
	model     config.SelectedModel
	modelType config.SelectedModelType

	width int
	state APIKeyInputState

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}
	input   textinput.Model
	spinner spinner.Model
	help    help.Model
}

var _ Dialog = (*APIKeyInput)(nil)

// NewAPIKeyInput creates a new Models dialog.
func NewAPIKeyInput(com *common.Common, provider catwalk.Provider, model config.SelectedModel, modelType config.SelectedModelType) (*APIKeyInput, error) {
	t := com.Styles

	m := APIKeyInput{}
	m.com = com
	m.provider = provider
	m.model = model
	m.modelType = modelType
	m.width = 60

	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Enter you API key..."
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()
	m.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1) // (1) cursor padding

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Base.Foreground(t.Green)),
	)

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "submit"),
	)
	m.keyMap.Close = CloseKey

	return &m, nil
}

// ID implements Dialog.
func (m *APIKeyInput) ID() string {
	return APIKeyInputID
}

// HandleMsg implements [Dialog].
func (m *APIKeyInput) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case ActionChangeAPIKeyState:
		m.state = msg.State
		switch m.state {
		case APIKeyInputStateVerifying:
			cmd := tea.Batch(m.spinner.Tick, m.verifyAPIKey)
			return ActionCmd{cmd}
		}
	case spinner.TickMsg:
		switch m.state {
		case APIKeyInputStateVerifying:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.KeyPressMsg:
		switch {
		case m.state == APIKeyInputStateVerifying:
			// do nothing
		case key.Matches(msg, m.keyMap.Close):
			switch m.state {
			case APIKeyInputStateVerified:
				return m.saveKeyAndContinue()
			default:
				return ActionClose{}
			}
		case key.Matches(msg, m.keyMap.Submit):
			switch m.state {
			case APIKeyInputStateInitial, APIKeyInputStateError:
				return ActionChangeAPIKeyState{State: APIKeyInputStateVerifying}
			case APIKeyInputStateVerified:
				return m.saveKeyAndContinue()
			}
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Draw implements [Dialog].
func (m *APIKeyInput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles

	textStyle := t.Dialog.SecondaryText
	helpStyle := t.Dialog.HelpView
	dialogStyle := t.Dialog.View.Width(m.width)
	inputStyle := t.Dialog.InputPrompt
	helpStyle = helpStyle.Width(m.width - dialogStyle.GetHorizontalFrameSize())

	m.input.Prompt = m.spinner.View()

	content := strings.Join([]string{
		m.headerView(),
		inputStyle.Render(m.inputView()),
		textStyle.Render("This will be written in your global configuration:"),
		textStyle.Render(config.GlobalConfigData()),
		"",
		helpStyle.Render(m.help.View(m)),
	}, "\n")

	view := dialogStyle.Render(content)

	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (m *APIKeyInput) headerView() string {
	t := m.com.Styles
	titleStyle := t.Dialog.Title
	dialogStyle := t.Dialog.View.Width(m.width)

	headerOffset := titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
	return common.DialogTitle(t, titleStyle.Render(m.dialogTitle()), m.width-headerOffset)
}

func (m *APIKeyInput) dialogTitle() string {
	t := m.com.Styles
	textStyle := t.Dialog.TitleText
	errorStyle := t.Dialog.TitleError
	accentStyle := t.Dialog.TitleAccent

	switch m.state {
	case APIKeyInputStateInitial:
		return textStyle.Render("Enter your ") + accentStyle.Render(fmt.Sprintf("%s Key", m.provider.Name)) + textStyle.Render(".")
	case APIKeyInputStateVerifying:
		return textStyle.Render("Verifying your ") + accentStyle.Render(fmt.Sprintf("%s Key", m.provider.Name)) + textStyle.Render("...")
	case APIKeyInputStateVerified:
		return accentStyle.Render(fmt.Sprintf("%s Key", m.provider.Name)) + textStyle.Render(" validated.")
	case APIKeyInputStateError:
		return errorStyle.Render("Invalid ") + accentStyle.Render(fmt.Sprintf("%s Key", m.provider.Name)) + errorStyle.Render(". Try again?")
	}
	return ""
}

func (m *APIKeyInput) inputView() string {
	t := m.com.Styles

	switch m.state {
	case APIKeyInputStateInitial:
		m.input.Prompt = "> "
		m.input.SetStyles(t.TextInput)
		m.input.Focus()
	case APIKeyInputStateVerifying:
		ts := t.TextInput
		ts.Blurred.Prompt = ts.Focused.Prompt

		m.input.Prompt = m.spinner.View()
		m.input.SetStyles(ts)
		m.input.Blur()
	case APIKeyInputStateVerified:
		ts := t.TextInput
		ts.Blurred.Prompt = ts.Focused.Prompt

		m.input.Prompt = styles.CheckIcon + " "
		m.input.SetStyles(ts)
		m.input.Blur()
	case APIKeyInputStateError:
		ts := t.TextInput
		ts.Focused.Prompt = ts.Focused.Prompt.Foreground(charmtone.Cherry)

		m.input.Prompt = styles.ErrorIcon + " "
		m.input.SetStyles(ts)
		m.input.Focus()
	}
	return m.input.View()
}

// Cursor returns the cursor position relative to the dialog.
func (m *APIKeyInput) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

// FullHelp returns the full help view.
func (m *APIKeyInput) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			m.keyMap.Submit,
			m.keyMap.Close,
		},
	}
}

// ShortHelp returns the full help view.
func (m *APIKeyInput) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.Submit,
		m.keyMap.Close,
	}
}

func (m *APIKeyInput) verifyAPIKey() tea.Msg {
	start := time.Now()

	providerConfig := config.ProviderConfig{
		ID:      string(m.provider.ID),
		Name:    m.provider.Name,
		APIKey:  m.input.Value(),
		Type:    m.provider.Type,
		BaseURL: m.provider.APIEndpoint,
	}
	err := providerConfig.TestConnection(config.Get().Resolver())

	// intentionally wait for at least 750ms to make sure the user sees the spinner
	elapsed := time.Since(start)
	minimum := 750 * time.Millisecond
	if elapsed < minimum {
		time.Sleep(minimum - elapsed)
	}

	if err == nil {
		return ActionChangeAPIKeyState{APIKeyInputStateVerified}
	}
	return ActionChangeAPIKeyState{APIKeyInputStateError}
}

func (m *APIKeyInput) saveKeyAndContinue() Action {
	cfg := m.com.Config()

	err := cfg.SetProviderAPIKey(string(m.provider.ID), m.input.Value())
	if err != nil {
		return ActionCmd{uiutil.ReportError(fmt.Errorf("failed to save API key: %w", err))}
	}

	return ActionSelectModel{
		Provider:  m.provider,
		Model:     m.model,
		ModelType: m.modelType,
	}
}
