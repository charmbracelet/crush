package dialog

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	// ThemeNewID is the identifier for the new theme dialog.
	ThemeNewID = "theme_new"

	themeNewDialogMaxWidth = 60
)

// ThemeNew prompts for a name and base when creating a new theme.
type ThemeNew struct {
	com   *common.Common
	help  help.Model
	input textinput.Model
	base  string
	err   string

	keyMap struct {
		Confirm key.Binding
		Close   key.Binding
	}
}

var _ Dialog = (*ThemeNew)(nil)

// NewThemeNew creates a dialog that asks for the name of a new theme. The
// new theme inherits its palette from base.
func NewThemeNew(com *common.Common, base string) *ThemeNew {
	if base == "" {
		base = "charmtone"
	}
	d := &ThemeNew{com: com, base: base}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	d.help = h

	d.input = textinput.New()
	d.input.SetVirtualCursor(false)
	d.input.Placeholder = "my-theme"
	d.input.SetStyles(com.Styles.TextInput)
	d.input.Focus()

	d.keyMap.Confirm = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "create"),
	)
	d.keyMap.Close = CloseKey

	return d
}

func (d *ThemeNew) ID() string {
	return ThemeNewID
}

func (d *ThemeNew) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, d.keyMap.Confirm):
			name := strings.TrimSpace(d.input.Value())
			if err := styles.ValidateThemeName(name); err != nil {
				d.err = err.Error()
				return nil
			}
			return ActionCreateTheme{Name: name, Base: d.base}
		default:
			var cmd tea.Cmd
			d.input, cmd = d.input.Update(msg)
			d.err = ""
			return ActionCmd{cmd}
		}
	}
	return nil
}

func (d *ThemeNew) Cursor() *tea.Cursor {
	return InputCursor(d.com.Styles, d.input.Cursor())
}

func (d *ThemeNew) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := d.com.Styles
	width := max(0, min(themeNewDialogMaxWidth, area.Dx()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	d.input.SetWidth(innerWidth - t.Dialog.InputPrompt.GetHorizontalFrameSize() - 1)
	d.help.SetWidth(innerWidth)

	rc := NewRenderContext(t, width)
	rc.Title = "New Theme"
	rc.AddPart(t.Dialog.InputPrompt.Render(d.input.View()))
	if d.err != "" {
		errWidth := innerWidth - 2 // account for left and right margins
		errStyle := t.Dialog.TitleError.Margin(0, 1).MarginBottom(1).Width(errWidth)
		rc.AddPart(errStyle.Render(d.err))
	}
	rc.Help = renderDialogHelp(t, &d.help, d, innerWidth)

	view := rc.Render()
	cur := d.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

func (d *ThemeNew) ShortHelp() []key.Binding {
	return []key.Binding{d.keyMap.Confirm, d.keyMap.Close}
}

func (d *ThemeNew) FullHelp() [][]key.Binding {
	return [][]key.Binding{{d.keyMap.Confirm, d.keyMap.Close}}
}

var _ help.KeyMap = (*ThemeNew)(nil)
