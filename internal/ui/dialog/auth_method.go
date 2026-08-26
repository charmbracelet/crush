package dialog

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// AuthMethodID is the identifier of the authentication-method dialog.
const AuthMethodID = "auth_method"

// AuthMethod lets providers with both OAuth and API-key credentials choose
// the authentication route explicitly.
type AuthMethod struct {
	com       *common.Common
	provider  catwalk.Provider
	model     config.SelectedModel
	modelType config.SelectedModelType
	selected  int
}

// NewAuthMethod creates a dialog that lets the user choose OAuth or an API key.
func NewAuthMethod(com *common.Common, provider catwalk.Provider, model config.SelectedModel, modelType config.SelectedModelType) *AuthMethod {
	return &AuthMethod{com: com, provider: provider, model: model, modelType: modelType}
}

func (m *AuthMethod) ID() string { return AuthMethodID }

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
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter", "ctrl+y"))):
		return ActionSelectAuthMethod{
			Provider: m.provider, Model: m.model, ModelType: m.modelType, OAuth: m.selected == 0,
		}
	}
	return nil
}

func (m *AuthMethod) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	title := common.DialogTitle(t, t.Dialog.Title.Render("Choose authentication method"), 52, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
	first, second := "  ChatGPT/Codex OAuth", "  OpenAI API key"
	if m.selected == 0 {
		first = "> ChatGPT/Codex OAuth"
	} else {
		second = "> OpenAI API key"
	}
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", first, second, "", "↑/↓ choose • enter select • esc cancel")
	width := min(60, area.Dx())
	DrawCenter(scr, area, t.Dialog.View.Width(width).Render(content))
	return nil
}

func (m *AuthMethod) FullHelp() [][]key.Binding { return nil }
func (m *AuthMethod) ShortHelp() []key.Binding  { return nil }
