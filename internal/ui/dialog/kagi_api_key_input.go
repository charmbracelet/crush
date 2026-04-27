package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

const KagiAPIKeyInputID = "kagi_api_key_input"

type KagiAPIKeyInput struct {
	com   *common.Common
	width int

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}
	input textinput.Model
	help  help.Model
}

var _ Dialog = (*KagiAPIKeyInput)(nil)

func NewKagiAPIKeyInput(com *common.Common) (*KagiAPIKeyInput, tea.Cmd) {
	t := com.Styles

	k := KagiAPIKeyInput{}
	k.com = com
	k.width = 60

	innerWidth := k.width - t.Dialog.View.GetHorizontalFrameSize() - 2

	k.input = textinput.New()
	k.input.SetVirtualCursor(false)
	k.input.Placeholder = "Enter your Kagi API key..."
	k.input.SetStyles(com.Styles.TextInput)
	k.input.Focus()
	k.input.SetWidth(max(0, innerWidth-t.Dialog.InputPrompt.GetHorizontalFrameSize()-1))

	cfg := com.Config()
	// Only prefill environment references to avoid displaying saved secrets.
	if cfg != nil && strings.HasPrefix(cfg.Tools.WebSearch.KagiAPIKey, "$") {
		k.input.SetValue(cfg.Tools.WebSearch.KagiAPIKey)
	}

	k.help = help.New()
	k.help.Styles = t.DialogHelpStyles()

	k.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "save"),
	)
	k.keyMap.Close = CloseKey

	return &k, nil
}

func (k *KagiAPIKeyInput) ID() string {
	return KagiAPIKeyInputID
}

func (k *KagiAPIKeyInput) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, k.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, k.keyMap.Submit):
			apiKey := strings.TrimSpace(k.input.Value())
			if apiKey == "" {
				return nil
			}
			return ActionSaveKagiAPIKey{APIKey: apiKey}
		default:
			var cmd tea.Cmd
			k.input, cmd = k.input.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		k.input, cmd = k.input.Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	}
	return nil
}

func (k *KagiAPIKeyInput) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := k.com.Styles

	textStyle := t.Dialog.SecondaryText
	helpStyle := t.Dialog.HelpView
	dialogStyle := t.Dialog.View.Width(k.width)
	inputStyle := t.Dialog.InputPrompt
	helpStyle = helpStyle.Width(k.width - dialogStyle.GetHorizontalFrameSize())

	content := strings.Join([]string{
		k.headerView(),
		inputStyle.Render(k.input.View()),
		textStyle.Render("This will be written in your global configuration:"),
		textStyle.Render(config.GlobalConfigData()),
		textStyle.Render("Environment references like $KAGI_API_KEY are supported."),
		"",
		helpStyle.Render(k.help.View(k)),
	}, "\n")

	cur := k.Cursor()
	view := dialogStyle.Render(content)
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (k *KagiAPIKeyInput) headerView() string {
	t := k.com.Styles
	titleStyle := t.Dialog.Title
	dialogStyle := t.Dialog.View.Width(k.width)
	headerOffset := titleStyle.GetHorizontalFrameSize() + dialogStyle.GetHorizontalFrameSize()
	return common.DialogTitle(t, titleStyle.Render(t.Dialog.TitleText.Render("Enter your ")+t.Dialog.TitleAccent.Render("Kagi API Key")+t.Dialog.TitleText.Render(".")), k.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}

func (k *KagiAPIKeyInput) Cursor() *tea.Cursor {
	return InputCursor(k.com.Styles, k.input.Cursor())
}

func (k *KagiAPIKeyInput) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			k.keyMap.Submit,
			k.keyMap.Close,
		},
	}
}

func (k *KagiAPIKeyInput) ShortHelp() []key.Binding {
	return []key.Binding{
		k.keyMap.Submit,
		k.keyMap.Close,
	}
}
