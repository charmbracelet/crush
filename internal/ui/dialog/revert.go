package dialog

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

// RevertID is the unique identifier for the revert dialog.
const RevertID = "revert"

// revertChoice represents one of the three revert options.
type revertChoice int

const (
	revertBoth revertChoice = iota
	revertCodeOnly
	revertConversationOnly
)

// Revert is a confirm dialog that lets the user choose which parts of
// the session to revert (code, conversation, or both).
type Revert struct {
	com            *common.Common
	messageID      string
	messageContent string
	selected       revertChoice
	keyMap         struct {
		LeftRight key.Binding
		Enter     key.Binding
		Close     key.Binding
	}
}

var _ Dialog = (*Revert)(nil)

// NewRevert creates a new revert confirm dialog.
func NewRevert(com *common.Common, messageID, messageContent string) *Revert {
	r := &Revert{
		com:            com,
		messageID:      messageID,
		messageContent: messageContent,
		selected:       revertBoth,
	}
	r.keyMap.LeftRight = key.NewBinding(
		key.WithKeys("left", "right"),
		key.WithHelp("←/→", "switch option"),
	)
	r.keyMap.Enter = key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter", "confirm"),
	)
	r.keyMap.Close = CloseKey
	return r
}

// ID implements Dialog.
func (*Revert) ID() string { return RevertID }

// HandleMsg implements Dialog.
func (r *Revert) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, r.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, r.keyMap.LeftRight):
			r.selected = (r.selected + 1) % 3
			return nil
		case key.Matches(msg, r.keyMap.Enter):
			switch r.selected {
			case revertBoth:
				return ActionRevertToMessage{
					MessageID:           r.messageID,
					MessageContent:      r.messageContent,
					RestoreCode:         true,
					RestoreConversation: true,
				}
			case revertCodeOnly:
				return ActionRevertToMessage{
					MessageID:           r.messageID,
					MessageContent:      r.messageContent,
					RestoreCode:         true,
					RestoreConversation: false,
				}
			case revertConversationOnly:
				return ActionRevertToMessage{
					MessageID:           r.messageID,
					MessageContent:      r.messageContent,
					RestoreCode:         false,
					RestoreConversation: true,
				}
			}
		}
	}
	return nil
}

// Draw implements Dialog.
func (r *Revert) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	const question = "Revert to this point?"

	baseStyle := r.com.Styles.Dialog.Revert.Content
	questionStyle := baseStyle.Bold(true)

	buttons := []common.ButtonOpts{
		{Text: "Code + Chat", Selected: r.selected == revertBoth, Padding: 1},
		{Text: "Code Only", Selected: r.selected == revertCodeOnly, Padding: 1},
		{Text: "Chat Only", Selected: r.selected == revertConversationOnly, Padding: 1},
	}
	buttonRow := common.ButtonGroup(r.com.Styles, buttons, " ")

	hint := baseStyle.Render(fmt.Sprintf("  ← → to choose, enter to confirm"))
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		questionStyle.Render(question),
		"",
		buttonRow,
		hint,
	)
	view := r.com.Styles.Dialog.Revert.Frame.Render(content)
	DrawCenter(scr, area, view)
	return nil
}
