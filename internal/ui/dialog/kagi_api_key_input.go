package dialog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/exp/charmtone"
)

const KagiAPIKeyInputID = "kagi_api_key_input"

type KagiAPIKeyInput struct {
	com   *common.Common
	width int
	state APIKeyInputState

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}
	input          textinput.Model
	spinner        spinner.Model
	help           help.Model
	pendingAPIKey  string
	verifiedAPIKey string
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

	k.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Dialog.APIKey.Spinner),
	)

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
	case ActionChangeAPIKeyState:
		k.state = msg.State
		switch k.state {
		case APIKeyInputStateVerifying:
			k.pendingAPIKey = msg.APIKey
			k.verifiedAPIKey = ""
			cmd := tea.Batch(k.spinner.Tick, k.verifyAPIKey(k.pendingAPIKey))
			return ActionCmd{cmd}
		case APIKeyInputStateVerified:
			k.verifiedAPIKey = msg.APIKey
		}
	case spinner.TickMsg:
		switch k.state {
		case APIKeyInputStateVerifying:
			var cmd tea.Cmd
			k.spinner, cmd = k.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.KeyPressMsg:
		switch {
		case k.state == APIKeyInputStateVerifying:
		case key.Matches(msg, k.keyMap.Close):
			switch k.state {
			case APIKeyInputStateVerified:
				return k.saveAPIKey()
			default:
				return ActionClose{}
			}
		case key.Matches(msg, k.keyMap.Submit):
			apiKey := strings.TrimSpace(k.input.Value())
			if apiKey == "" {
				return nil
			}
			switch k.state {
			case APIKeyInputStateInitial, APIKeyInputStateError:
				return ActionChangeAPIKeyState{State: APIKeyInputStateVerifying, APIKey: apiKey}
			case APIKeyInputStateVerified:
				return k.saveAPIKey()
			}
		default:
			var cmd tea.Cmd
			k.input, cmd = k.input.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.PasteMsg:
		switch k.state {
		case APIKeyInputStateVerifying, APIKeyInputStateVerified:
			return nil
		}
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
		inputStyle.Render(k.inputView()),
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
	return common.DialogTitle(t, titleStyle.Render(k.dialogTitle()), k.width-headerOffset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}

func (k *KagiAPIKeyInput) dialogTitle() string {
	var (
		t           = k.com.Styles
		textStyle   = t.Dialog.TitleText
		errorStyle  = t.Dialog.TitleError
		accentStyle = t.Dialog.TitleAccent
	)
	switch k.state {
	case APIKeyInputStateInitial:
		return textStyle.Render("Enter your ") + accentStyle.Render("Kagi API Key") + textStyle.Render(".")
	case APIKeyInputStateVerifying:
		return textStyle.Render("Verifying your ") + accentStyle.Render("Kagi API Key") + textStyle.Render("...")
	case APIKeyInputStateVerified:
		return accentStyle.Render("Kagi API Key") + textStyle.Render(" validated.")
	case APIKeyInputStateError:
		return errorStyle.Render("Kagi API key failed to validate. Try again?")
	}
	return ""
}

func (k *KagiAPIKeyInput) inputView() string {
	t := k.com.Styles

	switch k.state {
	case APIKeyInputStateInitial:
		k.input.Prompt = "> "
		k.input.SetStyles(t.TextInput)
		k.input.Focus()
	case APIKeyInputStateVerifying:
		ts := t.TextInput
		ts.Blurred.Prompt = ts.Focused.Prompt

		k.input.Prompt = k.spinner.View()
		k.input.SetStyles(ts)
		k.input.Blur()
	case APIKeyInputStateVerified:
		ts := t.TextInput
		ts.Blurred.Prompt = ts.Focused.Prompt

		k.input.Prompt = styles.CheckIcon + " "
		k.input.SetStyles(ts)
		k.input.Blur()
	case APIKeyInputStateError:
		ts := t.TextInput
		ts.Focused.Prompt = ts.Focused.Prompt.Foreground(charmtone.Cherry)

		k.input.Prompt = styles.LSPErrorIcon + " "
		k.input.SetStyles(ts)
		k.input.Focus()
	}
	return k.input.View()
}

func (k *KagiAPIKeyInput) Cursor() *tea.Cursor {
	return InputCursor(k.com.Styles, k.input.Cursor())
}

func (k *KagiAPIKeyInput) verifyAPIKey(apiKey string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()

		err := k.validateAPIKey(apiKey)

		elapsed := time.Since(start)
		minimum := 750 * time.Millisecond
		if elapsed < minimum {
			time.Sleep(minimum - elapsed)
		}

		if err == nil {
			return ActionChangeAPIKeyState{State: APIKeyInputStateVerified, APIKey: apiKey}
		}
		return ActionChangeAPIKeyState{State: APIKeyInputStateError}
	}
}

func (k *KagiAPIKeyInput) validateAPIKey(apiKey string) error {
	resolvedAPIKey := apiKey
	if k.com.Workspace != nil {
		resolved, err := k.com.Workspace.Resolver().ResolveValue(apiKey)
		if err != nil {
			return fmt.Errorf("failed to resolve Kagi API key: %w", err)
		}
		resolvedAPIKey = resolved
	}
	if resolvedAPIKey == "" {
		return fmt.Errorf("kagi API key is required")
	}

	searchURL := "https://kagi.com/api/v0/search?" + url.Values{
		"q":     {"crush"},
		"limit": {"1"},
	}.Encode()

	req, err := http.NewRequestWithContext(context.Background(), "GET", searchURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create Kagi validation request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+resolvedAPIKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute Kagi validation request: %w", err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read Kagi validation response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusPaymentRequired:
		return nil
	default:
		return fmt.Errorf("kagi API key failed to validate: %s", resp.Status)
	}
}

func (k *KagiAPIKeyInput) saveAPIKey() Action {
	apiKey := k.verifiedAPIKey
	if apiKey == "" {
		apiKey = strings.TrimSpace(k.input.Value())
	}
	return ActionSaveKagiAPIKey{APIKey: apiKey}
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
