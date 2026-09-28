package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/antigravity"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// GoogleSecretInputID is the identifier for the dialog that collects the
// OAuth client secret a Google AI subscription login needs.
const GoogleSecretInputID = "google_secret_input"

// ActionGoogleSecretSaved means the client secret reached the configuration,
// so the sign-in that needs it can finally start.
type ActionGoogleSecretSaved struct {
	Provider  catwalk.Provider
	Model     config.SelectedModel
	ModelType config.SelectedModelType
}

// GoogleSecretInput asks for the OAuth client secret Google's token endpoint
// demands alongside its public client id.
//
// The value is not personal — Google ships it inside the Antigravity and
// Gemini CLIs — but it cannot be committed, so it is collected once and kept
// in the global configuration like any other credential.
type GoogleSecretInput struct {
	com          *common.Common
	isOnboarding bool

	provider  catwalk.Provider
	model     config.SelectedModel
	modelType config.SelectedModelType

	width   int
	errText string

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}
	input textinput.Model
	help  help.Model
}

var _ Dialog = (*GoogleSecretInput)(nil)

// NewGoogleSecretInput creates the client secret prompt.
func NewGoogleSecretInput(
	com *common.Common,
	isOnboarding bool,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) (*GoogleSecretInput, tea.Cmd) {
	t := com.Styles

	m := &GoogleSecretInput{
		com:          com,
		isOnboarding: isOnboarding,
		provider:     provider,
		model:        model,
		modelType:    modelType,
	}

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = "Enter the client secret..."
	// A credential the config file stores in plain text still should not
	// sit on screen while it is typed.
	m.input.EchoMode = textinput.EchoPassword
	m.input.SetStyles(t.TextInput)
	m.input.Focus()

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "submit"),
	)
	m.keyMap.Close = CloseKey

	return m, nil
}

// ID implements [Dialog].
func (m *GoogleSecretInput) ID() string {
	return GoogleSecretInputID
}

// HandleMsg implements [Dialog].
func (m *GoogleSecretInput) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Submit):
			return m.save()
		default:
			m.errText = ""
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.PasteMsg:
		m.errText = ""
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	}
	return nil
}

// save persists the entered secret and hands control back so the caller can
// restart the sign-in that needed it.
func (m *GoogleSecretInput) save() Action {
	secret := strings.TrimSpace(m.input.Value())
	if secret == "" {
		m.errText = "A client secret is required: Google rejects both sign-in and refresh without it."
		return nil
	}

	field := "providers." + antigravity.ProviderID + ".oauth_client_secret"
	if err := m.com.Workspace.SetConfigField(config.ScopeGlobal, field, secret); err != nil {
		m.errText = fmt.Sprintf("Could not save the client secret: %v", err)
		return nil
	}

	return ActionGoogleSecretSaved{
		Provider:  m.provider,
		Model:     m.model,
		ModelType: m.modelType,
	}
}

// Draw implements [Dialog].
func (m *GoogleSecretInput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles

	m.width = max(0, min(70, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize() - 2
	m.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1)) // (1) cursor padding

	textStyle := t.Dialog.SecondaryText
	dialogStyle := t.Dialog.View.Width(m.width)
	inputStyle := t.Dialog.InputPrompt
	helpView := renderDialogHelp(t, &m.help, m, m.width-dialogStyle.GetHorizontalFrameSize())

	explainer := []string{
		"The subscription signs in as Google's Antigravity CLI, whose client",
		"secret is not one crush can ship, so you supply it once: paste it",
		"below, set providers.gemini-sub.oauth_client_secret in crush.json,",
		"or export CRUSH_ANTIGRAVITY_CLIENT_SECRET.",
	}
	if m.errText != "" {
		explainer = append(explainer, "", t.Dialog.TitleError.Render(m.errText))
	}

	content := strings.Join(append([]string{
		m.headerView(),
		inputStyle.Render(m.input.View()),
		textStyle.Render("This will be written in your global configuration:"),
		textStyle.Render(config.GlobalConfigData()),
		textStyle.Render("You can also set " + antigravity.ClientSecretEnv + " instead."),
		"",
		strings.Join(explainer, "\n"),
		"",
		helpView,
	}, "\n"), "\n")

	cur := m.Cursor()

	if m.isOnboarding {
		cur = adjustOnboardingInputCursor(t, cur)
		DrawOnboardingCursor(scr, area, content, cur)
	} else {
		DrawCenterCursor(scr, area, dialogStyle.Render(content), cur)
	}
	return cur
}

func (m *GoogleSecretInput) headerView() string {
	var (
		t           = m.com.Styles
		titleStyle  = t.Dialog.Title
		textStyle   = t.Dialog.PrimaryText
		dialogStyle = t.Dialog.View.Width(m.width)
	)
	title := textStyle.Render("Enter the ") +
		t.Dialog.TitleAccent.Render("Google Subscription") +
		textStyle.Render(" client secret.")
	if m.isOnboarding {
		return textStyle.Render(strings.TrimSuffix(title, "\n"))
	}
	headerOffset := titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
	return common.DialogTitle(t, titleStyle.Render(title), m.width-headerOffset, m.com.Styles.Dialog.TitleGradFromColor, m.com.Styles.Dialog.TitleGradToColor)
}

// Cursor returns the cursor position relative to the dialog.
func (m *GoogleSecretInput) Cursor() *tea.Cursor {
	return InputCursor(m.com.Styles, m.input.Cursor())
}

// FullHelp returns the full help view.
func (m *GoogleSecretInput) FullHelp() [][]key.Binding {
	return [][]key.Binding{{m.keyMap.Submit, m.keyMap.Close}}
}

// ShortHelp returns the short help view.
func (m *GoogleSecretInput) ShortHelp() []key.Binding {
	return []key.Binding{m.keyMap.Submit, m.keyMap.Close}
}
