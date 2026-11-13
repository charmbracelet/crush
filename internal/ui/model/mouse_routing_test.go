package model

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// findPainted returns the cell where text first appears in a frame.
func findPainted(t testing.TB, frame, text string) image.Point {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(frame), "\n") {
		if before, _, ok := strings.Cut(line, text); ok {
			return image.Pt(ansi.StringWidth(before), y)
		}
	}
	t.Fatalf("%q is not painted", text)
	return image.Point{}
}

func TestOnlyLeftClicksAct(t *testing.T) {
	t.Parallel()
	type target func(*testing.T) (u *UI, at image.Point, acted func() bool)
	chatAt := func(at image.Point) target {
		return func(t *testing.T) (*UI, image.Point, func() bool) {
			u := newFrameTestUI(t)
			u.chat.SetMessages(frameTestItems("message")[:12]...)
			u.layout.main = image.Rect(3, 2, 43, 12)
			u.chat.SetSize(40, 10)
			u.chat.ScrollToTop()
			return u, at, func() bool {
				u.Update(tea.MouseMotionMsg{X: 20, Y: 6, Button: tea.MouseLeft})
				return u.chat.HasHighlight()
			}
		}
	}
	for _, tc := range []struct {
		name     string
		open     target
		leftActs bool
	}{
		{"chat text", chatAt(image.Pt(5, 4)), true},
		{"left of the chat", chatAt(image.Pt(2, 4)), false},
		{"right of the chat", chatAt(image.Pt(43, 4)), false},
		{"inline answer", func(t *testing.T) (*UI, image.Point, func() bool) {
			u := newFrameTestUI(t)
			f := dialog.NewQuestionForm(u.com.Styles, question.Request{Questions: []question.Question{{
				ID: "confirm", Type: question.TypeYesNo, Text: "Proceed?",
			}}})
			answered := false
			f.OnAnswer = func([]question.Answer) { answered = true }
			u.activeInline = f
			u.updateLayoutAndSize()
			return u, findPainted(t, u.View().Content, "Yes"), func() bool { return answered }
		}, true},
	} {
		for _, button := range []tea.MouseButton{tea.MouseLeft, tea.MouseRight, tea.MouseMiddle} {
			t.Run(tc.name+"/"+button.String(), func(t *testing.T) {
				t.Parallel()
				u, at, acted := tc.open(t)
				u.Update(tea.MouseClickMsg{X: at.X, Y: at.Y, Button: button})
				require.Equal(t, button == tea.MouseLeft && tc.leftActs, acted())
			})
		}
	}
}

func TestDialogOpenedByUpdateEndsConversationDrag(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.SetMessages(frameTestItems("message")[:12]...)
	u.layout.main = image.Rect(3, 2, 43, 12)
	u.chat.SetSize(40, 10)
	u.chat.ScrollToTop()
	u.Update(tea.MouseClickMsg{X: 10, Y: 4, Button: tea.MouseLeft})
	u.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	require.True(t, u.dialog.ContainsDialog(dialog.QuitID))
	u.dialog.CloseFrontDialog()
	u.Update(tea.MouseMotionMsg{X: 20, Y: 6, Button: tea.MouseLeft})
	require.False(t, u.chat.HasHighlight(), "opening the dialog ended the drag")
}

func TestDialogEndsConversationDragBeneathIt(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.SetMessages(frameTestItems("message")[:12]...)
	u.layout.main = image.Rect(3, 2, 43, 12)
	u.chat.SetSize(40, 10)
	u.chat.ScrollToTop()
	u.Update(tea.MouseClickMsg{X: 10, Y: 4, Button: tea.MouseLeft})
	u.dialog.OpenDialog(dialog.NewPermissions(u.com, permission.PermissionRequest{ToolName: "custom"}))
	u.Update(tea.MouseReleaseMsg{X: 10, Y: 4, Button: tea.MouseLeft})
	u.dialog.CloseFrontDialog()
	u.Update(tea.MouseMotionMsg{X: 20, Y: 6})
	require.False(t, u.chat.HasHighlight(), "the dialog ended the drag that began beneath it")
}
