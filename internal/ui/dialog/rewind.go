package dialog

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/charmbracelet/crush/internal/filehistory"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"unicode"
)

const RewindID = "rewind"

// ActionRewind requests a history list, a selected restore, or redo.
type ActionRewind struct{ SessionID, Action, Target string }

// Rewind is a prompt selector with an explicit combined-restore confirmation.
type Rewind struct {
	com      *common.Common
	session  string
	points   []filehistory.Choice
	selected int
	confirm  bool
}

func NewRewind(com *common.Common, session string, points []filehistory.Choice) *Rewind {
	return &Rewind{com: com, session: session, points: points}
}
func (r *Rewind) ID() string { return RewindID }
func (r *Rewind) HandleMsg(msg tea.Msg) Action {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch key.String() {
	case "esc":
		if r.confirm {
			r.confirm = false
			return nil
		}
		return ActionClose{}
	case "up", "k":
		if !r.confirm {
			r.selected = max(0, r.selected-1)
		}
	case "down", "j":
		if !r.confirm {
			r.selected = min(len(r.points)-1, r.selected+1)
		}
	case "enter":
		if len(r.points) == 0 {
			return ActionClose{}
		}
		if !r.confirm {
			r.confirm = true
			return nil
		}
		return ActionRewind{SessionID: r.session, Action: "rewind", Target: r.points[r.selected].Turn}
	}
	return nil
}
func (r *Rewind) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	width := max(0, min(76, area.Dx()))
	style := r.com.Styles.Dialog.View.Width(width)
	inner := max(1, width-style.GetHorizontalFrameSize())
	lines := []string{"Rewind — files and conversation", ""}
	if r.confirm {
		lines = append(lines, "Restore the state before this prompt?", cleanPrompt(r.points[r.selected].Label), "", "Enter: restore both · Esc: back", "/redo returns to the state before rewind.")
	} else {
		count := max(1, area.Dy()-8)
		start := max(0, r.selected-count+1)
		for index := start; index < min(len(r.points), start+count); index++ {
			prefix := "  "
			if index == r.selected {
				prefix = "> "
			}
			lines = append(lines, fmt.Sprintf("%s%s  %s", prefix, r.points[index].Time, cleanPrompt(r.points[index].Label)))
		}
		lines = append(lines, "", "↑/↓: choose · Enter: select · Esc: cancel")
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], inner, "…")
	}
	DrawCenter(scr, area, style.Render(strings.Join(lines, "\n")))
	return nil
}
func cleanPrompt(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, ansi.Strip(text))
}
