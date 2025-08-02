package sessions

import (
	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/tui/components/chat"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/charmbracelet/lipgloss/v2"
)

const DeleteSessionDialogID dialogs.DialogID = "delete-session"

type DeleteSessionDialog interface {
	dialogs.DialogModel
}

type deleteSessionDialogCmp struct {
	wWidth     int
	wHeight    int
	session    session.Session
	selectedNo bool
	keymap     DeleteKeyMap
}

type DeleteKeyMap struct {
	LeftRight,
	EnterSpace,
	Yes,
	No,
	Tab,
	Close key.Binding
}

func DefaultDeleteKeymap() DeleteKeyMap {
	return DeleteKeyMap{
		LeftRight: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←/→", "switch options"),
		),
		EnterSpace: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter/space", "confirm"),
		),
		Yes: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y/Y", "yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n/N", "no"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch options"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

func NewDeleteSessionDialog(session session.Session) DeleteSessionDialog {
	return &deleteSessionDialogCmp{
		session:    session,
		selectedNo: true,
		keymap:     DefaultDeleteKeymap(),
	}
}

func (d *deleteSessionDialogCmp) Init() tea.Cmd {
	return nil
}

func (d *deleteSessionDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.wWidth = msg.Width
		d.wHeight = msg.Height
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, d.keymap.LeftRight, d.keymap.Tab):
			d.selectedNo = !d.selectedNo
			return d, nil
		case key.Matches(msg, d.keymap.EnterSpace):
			if !d.selectedNo {
				return d, tea.Sequence(
					util.CmdHandler(dialogs.CloseDialogMsg{}),
					util.CmdHandler(chat.SessionDeletedMsg{Session: d.session}),
				)
			}
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, d.keymap.Yes):
			return d, tea.Sequence(
				util.CmdHandler(dialogs.CloseDialogMsg{}),
				util.CmdHandler(chat.SessionDeletedMsg{Session: d.session}),
			)
		case key.Matches(msg, d.keymap.No, d.keymap.Close):
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return d, nil
}

func (d *deleteSessionDialogCmp) View() string {
	t := styles.CurrentTheme()
	baseStyle := t.S().Base
	yesStyle := t.S().Text
	noStyle := yesStyle

	if d.selectedNo {
		noStyle = noStyle.Foreground(t.White).Background(t.Secondary)
		yesStyle = yesStyle.Background(t.BgSubtle)
	} else {
		yesStyle = yesStyle.Foreground(t.White).Background(t.Secondary)
		noStyle = noStyle.Background(t.BgSubtle)
	}

	question := "Delete session \"" + d.session.Title + "\"?"
	const horizontalPadding = 3
	yesButton := yesStyle.Padding(0, horizontalPadding).Render("Delete")
	noButton := noStyle.Padding(0, horizontalPadding).Render("Cancel")

	buttons := baseStyle.Width(lipgloss.Width(question)).Align(lipgloss.Right).Render(
		lipgloss.JoinHorizontal(lipgloss.Center, yesButton, "  ", noButton),
	)

	content := baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			question,
			"",
			buttons,
		),
	)

	deleteDialogStyle := baseStyle.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)

	return deleteDialogStyle.Render(content)
}

func (d *deleteSessionDialogCmp) Position() (int, int) {
	question := "Delete session \"" + d.session.Title + "\"?"
	row := d.wHeight / 2
	row -= 7 / 2
	col := d.wWidth / 2
	col -= (lipgloss.Width(question) + 4) / 2

	return row, col
}

func (d *deleteSessionDialogCmp) ID() dialogs.DialogID {
	return DeleteSessionDialogID
}
