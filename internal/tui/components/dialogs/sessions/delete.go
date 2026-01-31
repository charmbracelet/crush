package sessions

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

const (
	DeleteSessionDialogID dialogs.DialogID = "delete-session"
)

// DeleteSessionDialog represents a confirmation dialog for deleting a session.
type DeleteSessionDialog interface {
	dialogs.DialogModel
}

type deleteSessionDialogCmp struct {
	sessionID    string
	sessionTitle string
	wWidth       int
	wHeight      int
	selectedNo   bool // true if "No" button is selected
	keymap       DeleteKeyMap
	help         help.Model
}

// NewDeleteSessionDialog creates a new delete session confirmation dialog.
func NewDeleteSessionDialog(sessionID, sessionTitle string) DeleteSessionDialog {
	t := styles.CurrentTheme()
	help := help.New()
	help.Styles = t.S().Help
	return &deleteSessionDialogCmp{
		sessionID:    sessionID,
		sessionTitle: sessionTitle,
		selectedNo:   true, // Default to "No" for safety
		keymap:       DefaultDeleteKeyMap(),
		help:         help,
	}
}

func (d *deleteSessionDialogCmp) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the delete session dialog.
func (d *deleteSessionDialogCmp) Update(msg tea.Msg) (util.Model, tea.Cmd) {
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
				return d, util.CmdHandler(DeleteSessionMsg{
					SessionID: d.sessionID,
				})
			}
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		case key.Matches(msg, d.keymap.Yes):
			return d, util.CmdHandler(DeleteSessionMsg{
				SessionID: d.sessionID,
			})
		case key.Matches(msg, d.keymap.No, d.keymap.Close):
			return d, util.CmdHandler(dialogs.CloseDialogMsg{})
		}
	}
	return d, nil
}

// View renders the delete session dialog with Yes/No buttons.
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

	questionText := "Are you sure you want to delete this session?"
	question := baseStyle.Render(questionText)

	titleStyle := t.S().Text.Background(t.BgSubtle).Padding(0, 1).Render(d.sessionTitle)
	sessionInfo := lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		titleStyle,
	)

	const horizontalPadding = 3
	yesButton := yesStyle.PaddingLeft(horizontalPadding).Underline(true).Render("Y") +
		yesStyle.PaddingRight(horizontalPadding).Render("es, delete")
	noButton := noStyle.PaddingLeft(horizontalPadding).Underline(true).Render("N") +
		noStyle.PaddingRight(horizontalPadding).Render("o, cancel")

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yesButton, "  ", noButton)

	content := baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			question,
			sessionInfo,
			"",
			buttons,
			"",
			d.help.View(d.keymap),
		),
	)

	deleteDialogStyle := baseStyle.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Width(d.dialogWidth())

	return deleteDialogStyle.Render(content)
}

func (d *deleteSessionDialogCmp) dialogWidth() int {
	return min(70, d.wWidth-8)
}

func (d *deleteSessionDialogCmp) Position() (int, int) {
	row := d.wHeight / 2
	row -= 12 / 2
	col := d.wWidth / 2
	col -= d.dialogWidth() / 2

	return row, col
}

func (d *deleteSessionDialogCmp) ID() dialogs.DialogID {
	return DeleteSessionDialogID
}

// DeleteKeyMap defines the keyboard bindings for the delete session dialog.
type DeleteKeyMap struct {
	LeftRight,
	EnterSpace,
	Yes,
	No,
	Tab,
	Close key.Binding
}

func DefaultDeleteKeyMap() DeleteKeyMap {
	return DeleteKeyMap{
		LeftRight: key.NewBinding(
			key.WithKeys("left", "right"),
			key.WithHelp("←/→", "switch"),
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
			key.WithHelp("tab", "switch"),
		),
		Close: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// KeyBindings implements layout.KeyMapProvider
func (k DeleteKeyMap) KeyBindings() []key.Binding {
	return []key.Binding{
		k.LeftRight,
		k.EnterSpace,
		k.Yes,
		k.No,
		k.Tab,
		k.Close,
	}
}

// FullHelp implements help.KeyMap.
func (k DeleteKeyMap) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := k.KeyBindings()
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

// ShortHelp implements help.KeyMap.
func (k DeleteKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.LeftRight,
		k.EnterSpace,
		k.Yes,
		k.No,
		k.Close,
	}
}

// DeleteSessionMsg is sent when a session is confirmed for deletion.
type DeleteSessionMsg struct {
	SessionID string
}
