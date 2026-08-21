package dialog

import (
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// TrustID is the identifier for the trust dialog.
const TrustID = "trust"

// ActionTrustAccept indicates the user accepted the project configs.
type ActionTrustAccept struct{}

// ActionTrustReject indicates the user rejected the project configs.
type ActionTrustReject struct{}

// Trust represents a confirmation dialog for trusting project config files.
type Trust struct {
	com        *common.Common
	selectedNo bool
	untrusted  []string
	workingDir string
	keyMap     struct {
		LeftRight,
		EnterSpace,
		Yes,
		No,
		Tab,
		Close key.Binding
	}
}

var _ Dialog = (*Trust)(nil)

// NewTrust creates a new trust confirmation dialog.
func NewTrust(com *common.Common, untrusted []string, workingDir string) *Trust {
	t := &Trust{
		com:        com,
		selectedNo: true,
		untrusted:  untrusted,
		workingDir: workingDir,
	}
	t.keyMap.LeftRight = key.NewBinding(
		key.WithKeys("left", "right"),
		key.WithHelp("←/→", "switch options"),
	)
	t.keyMap.EnterSpace = key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "confirm"),
	)
	t.keyMap.Yes = key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y/Y", "trust"),
	)
	t.keyMap.No = key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n/N", "reject"),
	)
	t.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch options"),
	)
	t.keyMap.Close = CloseKey
	return t
}

// ID implements [Dialog].
func (*Trust) ID() string {
	return TrustID
}

// HandleMsg implements [Dialog].
func (t *Trust) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, t.keyMap.Close):
			return ActionTrustReject{}
		case key.Matches(msg, t.keyMap.LeftRight, t.keyMap.Tab):
			t.selectedNo = !t.selectedNo
		case key.Matches(msg, t.keyMap.EnterSpace):
			if !t.selectedNo {
				return ActionTrustAccept{}
			}
			return ActionTrustReject{}
		case key.Matches(msg, t.keyMap.Yes):
			return ActionTrustAccept{}
		case key.Matches(msg, t.keyMap.No):
			return ActionTrustReject{}
		}
	}
	return nil
}

// Draw implements [Dialog].
func (t *Trust) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var (
		baseStyle = t.com.Styles.Dialog.Quit.Content
		hintStyle = t.com.Styles.Dialog.Quit.Hint
	)

	frameStyle := t.com.Styles.Dialog.Quit.Frame
	contentWidth := area.Dx() - frameStyle.GetHorizontalBorderSize() - frameStyle.GetHorizontalPadding()
	if contentWidth < 30 {
		contentWidth = 30
	}

	header := baseStyle.Render("Trust project configuration?")

	dangerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	descText := "This project contains config files that Crush has not seen before or that have changed. These configs can define providers, MCP servers, hooks, and other settings. This is " + dangerStyle.Render("potentially dangerous") + "."
	desc := hintStyle.Width(contentWidth).Render(descText)

	var fileList strings.Builder
	for _, p := range t.untrusted {
		rel, err := filepath.Rel(t.workingDir, p)
		if err != nil {
			rel = p
		}
		fileList.WriteString(hintStyle.Render("  • "+rel) + "\n")
	}

	buttons := common.ButtonGroup(t.com.Styles, []common.ButtonOpts{
		{Text: "Trust", Selected: !t.selectedNo, Padding: 3},
		{Text: "Reject", Selected: t.selectedNo, Padding: 3},
	}, " ")

	content := baseStyle.Width(contentWidth).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			"",
			desc,
			"",
			strings.TrimRight(fileList.String(), "\n"),
			"",
			buttons,
		),
	)

	view := frameStyle.Render(content)
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements [help.KeyMap].
func (t *Trust) ShortHelp() []key.Binding {
	return []key.Binding{
		t.keyMap.LeftRight,
		t.keyMap.EnterSpace,
	}
}

// FullHelp implements [help.KeyMap].
func (t *Trust) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{t.keyMap.LeftRight, t.keyMap.EnterSpace, t.keyMap.Yes, t.keyMap.No},
		{t.keyMap.Tab, t.keyMap.Close},
	}
}
