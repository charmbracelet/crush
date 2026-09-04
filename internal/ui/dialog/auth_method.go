package dialog

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// AuthMethodID is the identifier of the authentication method dialog.
const AuthMethodID = "auth_method"

// AuthMethod lets providers with both OAuth and API-key credentials choose
// their authentication method explicitly.
type AuthMethod struct {
	com       *common.Common
	provider  catwalk.Provider
	model     config.SelectedModel
	modelType config.SelectedModelType
	selected  int
}

// NewAuthMethod creates a dialog to choose between OAuth and API key.
func NewAuthMethod(
	com *common.Common,
	provider catwalk.Provider,
	model config.SelectedModel,
	modelType config.SelectedModelType,
) *AuthMethod {
	return &AuthMethod{
		com:       com,
		provider:  provider,
		model:     model,
		modelType: modelType,
	}
}

func (m *AuthMethod) ID() string {
	return AuthMethodID
}

func (m *AuthMethod) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}

	switch {
	case key.Matches(keyMsg, CloseKey):
		return ActionClose{}
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("up", "down", "tab"))):
		m.selected = 1 - m.selected
		return nil
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter", "ctrl+y"))):
		return ActionSelectAuthMethod{
			Provider:  m.provider,
			Model:     m.model,
			ModelType: m.modelType,
			UseOAuth:  m.selected == 0,
		}
	}
	return nil
}

func (m *AuthMethod) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	title := common.DialogTitle(
		t,
		t.Dialog.Title.Render(fmt.Sprintf("Authenticate with %s", m.provider.Name)),
		52,
		t.Dialog.TitleGradFromColor,
		t.Dialog.TitleGradToColor,
	)

	oauthLabel := "Sign in via OAuth (Subscription)"
	if p := oauth.Get(string(m.provider.ID)); p != nil {
		oauthLabel = fmt.Sprintf("Sign in via %s (OAuth)", p.Name())
	}
	apiKeyLabel := fmt.Sprintf("Enter %s API key", m.provider.Name)

	first := "  " + oauthLabel
	second := "  " + apiKeyLabel

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Dialog.TitleGradToColor)
	if m.selected == 0 {
		first = selectedStyle.Render("> " + oauthLabel)
	} else {
		second = selectedStyle.Render("> " + apiKeyLabel)
	}

	helpText := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("↑/↓ choose • enter select • esc cancel")
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", first, second, "", helpText)
	width := min(64, area.Dx())
	DrawCenter(scr, area, t.Dialog.View.Width(width).Render(content))
	return nil
}

func (m *AuthMethod) FullHelp() [][]key.Binding { return nil }
func (m *AuthMethod) ShortHelp() []key.Binding  { return nil }
