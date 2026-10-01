package dialog

import (
	"image"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func newTestPlanHandoff() *PlanHandoffInline {
	sty := styles.CharmtonePantera()
	return NewPlanHandoffInline(&common.Common{Styles: &sty})
}

func TestPlanHandoffRequestChangesKeyboard(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	p.SetFocused(true)

	done, cmd := p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.False(t, done)
	require.Nil(t, cmd)
	require.True(t, p.editing)
	require.True(t, p.editor.Focused())
	require.True(t, p.HeightChanged())
	require.Greater(t, p.Height(80), 3)
	require.Len(t, p.ShortHelp(), 3)

	called := false
	p.OnRequestChanges = func(feedback string) tea.Cmd {
		called = true
		require.Equal(t, "Revise the scope", feedback)
		return nil
	}

	done, cmd = p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.False(t, done, "empty feedback must not submit")
	require.Nil(t, cmd)
	require.False(t, called)

	p.editor.SetValue("  Revise the scope  ")
	done, cmd = p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, done)
	require.Nil(t, cmd)
	require.True(t, called)
}

func TestPlanHandoffRequestChangesNewline(t *testing.T) {
	t.Parallel()

	tests := map[string]tea.KeyPressMsg{
		"shift enter": {Code: tea.KeyEnter, Mod: tea.ModShift},
		"ctrl j":      {Code: 'j', Mod: tea.ModCtrl},
	}
	for name, msg := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p := newTestPlanHandoff()
			p.SetFocused(true)
			p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
			p.editor.SetValue("First line")

			done, _ := p.HandleKey(msg)

			require.False(t, done)
			require.Equal(t, "First line\n", p.editor.Value())
		})
	}
}

func TestPlanHandoffRequestChangesMouseSelection(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	p.SetFocused(true)
	p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	p.editor.SetValue("copy me")
	p.SetWidth(80)
	scr := uv.NewScreenBuffer(80, p.Height(80))
	p.Draw(scr, image.Rect(0, 0, 80, p.Height(80)))

	textX, textY := p.editorTextArea.Min.X, p.editorTextArea.Min.Y
	require.True(t, p.HandleMouseDown(textX, textY))
	require.True(t, p.HandleMouseDrag(textX+4, textY))
	require.Equal(t, "copy", p.SelectedText())
	handled, cmd := p.HandleMouseRelease(textX+4, textY)
	require.True(t, handled)
	require.NotNil(t, cmd)

	selected := uv.NewScreenBuffer(80, p.Height(80))
	p.Draw(selected, image.Rect(0, 0, 80, p.Height(80)))
	for x := textX; x < textX+4; x++ {
		requirePlanHandoffColorEqual(
			t,
			p.com.Styles.TextSelection.GetBackground(),
			selected.CellAt(x, textY).Style.Bg,
		)
	}
}

func TestPlanHandoffRequestChangesMouseSelectionAcrossLines(t *testing.T) {
	t.Parallel()

	t.Run("logical lines in reverse", func(t *testing.T) {
		t.Parallel()

		p := newTestPlanHandoff()
		p.SetFocused(true)
		p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
		p.editor.SetValue("first\nsecond")
		p.SetWidth(80)
		scr := uv.NewScreenBuffer(80, p.Height(80))
		p.Draw(scr, image.Rect(0, 0, 80, p.Height(80)))

		textX, textY := p.editorTextArea.Min.X, p.editorTextArea.Min.Y
		require.True(t, p.HandleMouseDown(textX+6, textY+1))
		require.True(t, p.HandleMouseDrag(textX, textY))
		require.Equal(t, "first\nsecond", p.SelectedText())
	})

	t.Run("soft wrapped line", func(t *testing.T) {
		t.Parallel()

		p := newTestPlanHandoff()
		p.SetFocused(true)
		p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
		p.SetWidth(14)
		p.editor.SetValue("one two three")
		scr := uv.NewScreenBuffer(14, p.Height(14))
		p.Draw(scr, image.Rect(0, 0, 14, p.Height(14)))

		textX, textY := p.editorTextArea.Min.X, p.editorTextArea.Min.Y
		require.True(t, p.HandleMouseDown(textX, textY))
		require.True(t, p.HandleMouseDrag(textX+5, textY+1))
		require.Equal(t, "one two three", p.SelectedText())
	})
}

func TestPlanHandoffEscapePreservesDraft(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	p.SetFocused(true)
	p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	p.editor.SetValue("Keep this draft")

	done, cmd := p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, done)
	require.Nil(t, cmd)
	require.False(t, p.editing)
	require.False(t, p.editor.Focused())
	require.Equal(t, "Keep this draft", p.editor.Value())
	require.True(t, p.HeightChanged())
	require.Equal(t, 5, p.Height(80))

	done, _ = p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.False(t, done)
	require.True(t, p.editing)
	require.Equal(t, "Keep this draft", p.editor.Value())
}

func TestPlanHandoffEscapeFromChoiceRequestsCollapse(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	done, cmd := p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, done)
	require.NotNil(t, cmd)
	_, ok := cmd().(CollapseInlineMsg)
	require.True(t, ok)
}

func TestPlanHandoffPasteAndCursor(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	p.SetFocused(true)
	p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})

	p.HandlePaste(tea.PasteMsg{Content: "Pasted feedback"})
	require.Equal(t, "Pasted feedback", p.editor.Value())

	scr := uv.NewScreenBuffer(80, p.Height(80))
	cursor := p.Draw(scr, image.Rect(0, 0, 80, p.Height(80)))
	require.NotNil(t, cursor)
	require.GreaterOrEqual(t, cursor.Y, 2)
}

func TestPlanHandoffMouseRequestChanges(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	p.SetFocused(true)
	scr := uv.NewScreenBuffer(80, 10)
	p.Draw(scr, image.Rect(0, 0, 80, 10))

	x, y := planHandoffButtonPoint(t, p, 2)
	done, handled := p.HandleMouseClick(x, y)
	require.False(t, done)
	require.True(t, handled)
	require.True(t, p.editing)
	require.True(t, p.editor.Focused())
}

func TestPlanHandoffAdaptiveChoiceLayout(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	wide := p.choiceLayout(80)
	narrow := p.choiceLayout(24)

	require.Equal(t, " ", wide.spacing)
	require.Equal(t, "\n", narrow.spacing)
	require.Greater(t, narrow.height, wide.height)
	require.Equal(t, wide.height, p.Height(80))
	require.Equal(t, narrow.height, p.Height(24))

	scr := uv.NewScreenBuffer(24, narrow.height)
	p.Draw(scr, image.Rect(0, 0, 24, narrow.height))
	firstX, firstY := planHandoffButtonPoint(t, p, 0)
	secondX, secondY := planHandoffButtonPoint(t, p, 1)
	require.Equal(t, firstX, secondX)
	require.Equal(t, firstY+1, secondY)
}

// The choice view carries the question badge; the badge must follow focus:
// the focused icon while the editor is focused, the blurred icon when the
// chat has focus.
func TestPlanHandoffChoiceQuestionBadgeFollowsFocus(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	p := newTestPlanHandoff()

	p.SetFocused(true)
	focused := uv.NewScreenBuffer(80, p.Height(80))
	p.Draw(focused, image.Rect(0, 0, 80, p.Height(80)))
	focusedLine := ansi.Strip(strings.SplitN(focused.Render(), "\n", 2)[0])
	require.Contains(t, focusedLine, "?")
	require.Contains(t, focusedLine, planHandoffQuestion)
	requirePlanHandoffColorEqual(t,
		sty.Editor.PromptQuestionIconFocused.GetBackground(),
		focused.CellAt(1+planHandoffIndent, 0).Style.Bg)

	p.SetFocused(false)
	blurred := uv.NewScreenBuffer(80, p.Height(80))
	p.Draw(blurred, image.Rect(0, 0, 80, p.Height(80)))
	blurredLine := ansi.Strip(strings.SplitN(blurred.Render(), "\n", 2)[0])
	require.Contains(t, blurredLine, "?")
	require.Contains(t, blurredLine, planHandoffQuestion)
	requirePlanHandoffColorEqual(t,
		sty.Editor.PromptQuestionIconBlurred.GetBackground(),
		blurred.CellAt(1+planHandoffIndent, 0).Style.Bg)
}

// When the chat has focus the handoff must keep its focused layout — the
// question and all three buttons — rendered in the inactive (lighter)
// button colors instead of collapsing to a one-line prompt.
func TestPlanHandoffBlurredViewKeepsChoiceLayout(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	p.SetFocused(true)
	focused := uv.NewScreenBuffer(80, p.Height(80))
	p.Draw(focused, image.Rect(0, 0, 80, p.Height(80)))

	p.SetFocused(false)
	height := p.Height(80)
	blurred := uv.NewScreenBuffer(80, height)
	p.Draw(blurred, image.Rect(0, 0, 80, height))

	plain := ansi.Strip(blurred.Render())
	require.Contains(t, plain, planHandoffQuestion)
	require.Contains(t, plain, "Start coding")
	require.Contains(t, plain, "Code with YOLO")
	require.Contains(t, plain, "Revise plan")

	sty := styles.CharmtonePantera()
	focusedBg := sty.Button.Focused.GetBackground()
	inactiveBg := sty.Button.Inactive.GetBackground()
	require.NotEqual(t, inactiveBg, sty.Button.Blurred.GetBackground(),
		"the inactive button style must be distinct from the blurred one")
	var sawInactiveButton bool
	for y, line := range blurred.Lines {
		for x, cell := range line {
			if cell.Width == 0 || cell.Content == " " {
				continue
			}
			require.False(t, planHandoffColorsEqual(focusedBg, cell.Style.Bg),
				"blurred handoff must not use the focused button background at (%d,%d)", x, y)
			if planHandoffColorsEqual(inactiveBg, cell.Style.Bg) {
				sawInactiveButton = true
			}
		}
	}
	require.True(t, sawInactiveButton, "blurred handoff must render buttons with the inactive style")

	var sawFocusedButton bool
	for _, line := range focused.Lines {
		for _, cell := range line {
			if cell.Width > 0 && cell.Content != " " && planHandoffColorsEqual(focusedBg, cell.Style.Bg) {
				sawFocusedButton = true
			}
		}
	}
	require.True(t, sawFocusedButton, "focused handoff must highlight the selected button")
}

func TestPlanHandoffSetWidthRecalculatesFeedbackHeight(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	p.SetFocused(true)
	p.HandleKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	p.SetWidth(80)
	p.editor.SetValue(strings.Repeat("feedback ", 7))
	wideHeight := p.Height(80)
	p.HeightChanged()

	p.SetWidth(20)
	narrowHeight := p.Height(20)

	require.Greater(t, narrowHeight, wideHeight)
	require.True(t, p.HeightChanged())
}

func TestPlanHandoffStartCoding(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	confirmed := 0
	p.OnConfirm = func(yolo bool) tea.Cmd {
		require.False(t, yolo)
		return func() tea.Msg {
			confirmed++
			return nil
		}
	}

	done, cmd := p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, done)
	require.NotNil(t, cmd)
	cmd()
	require.Equal(t, 1, confirmed)
}

func TestPlanHandoffCodingKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  tea.KeyPressMsg
		yolo bool
	}{
		{"coding lowercase", tea.KeyPressMsg{Code: 'c', Text: "c"}, false},
		{"coding uppercase", tea.KeyPressMsg{Code: 'C', Text: "C"}, false},
		{"yolo lowercase", tea.KeyPressMsg{Code: 'y', Text: "y"}, true},
		{"yolo uppercase", tea.KeyPressMsg{Code: 'Y', Text: "Y"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := newTestPlanHandoff()
			confirmed := false
			p.OnConfirm = func(yolo bool) tea.Cmd {
				require.Equal(t, tt.yolo, yolo)
				confirmed = true
				return nil
			}

			done, _ := p.HandleKey(tt.key)
			require.True(t, done)
			require.True(t, confirmed)
		})
	}
}

func TestPlanHandoffShortHelpShowsCodingKeys(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	help := p.ShortHelp()
	require.Len(t, help, 5)
	require.Contains(t, help[2].Help().Key, "c")
	require.Contains(t, help[3].Help().Key, "y")
}

func planHandoffButtonPoint(t *testing.T, p *PlanHandoffInline, index int) (int, int) {
	t.Helper()
	for y := range 10 {
		for x := range 80 {
			if common.HitButtonIndex(p.compositor, x, y) == index {
				return x, y
			}
		}
	}
	t.Fatalf("button %d was not rendered", index)
	return 0, 0
}

func requirePlanHandoffColorEqual(t *testing.T, want, got color.Color) {
	t.Helper()
	require.NotNil(t, want)
	require.NotNil(t, got)
	require.True(t, planHandoffColorsEqual(want, got))
}

func planHandoffColorsEqual(left, right color.Color) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftR, leftG, leftB, leftA := left.RGBA()
	rightR, rightG, rightB, rightA := right.RGBA()
	return [4]uint32{leftR, leftG, leftB, leftA} == [4]uint32{rightR, rightG, rightB, rightA}
}

func TestPlanHandoffYOLO(t *testing.T) {
	t.Parallel()
	for _, mouse := range []bool{false, true} {
		p := newTestPlanHandoff()
		confirmed := false
		p.OnConfirm = func(yolo bool) tea.Cmd {
			require.True(t, yolo)
			confirmed = true
			return nil
		}
		if mouse {
			scr := uv.NewScreenBuffer(24, p.Height(24))
			p.Draw(scr, image.Rect(0, 0, 24, p.Height(24)))
			x, y := planHandoffButtonPoint(t, p, 1)
			done, handled := p.HandleMouseClick(x, y)
			require.True(t, done)
			require.True(t, handled)
		} else {
			p.HandleKey(tea.KeyPressMsg{Code: tea.KeyRight})
			done, _ := p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			require.True(t, done)
		}
		require.True(t, confirmed)
	}
}

func TestPlanHandoffChoiceWrapAndBottomGap(t *testing.T) {
	t.Parallel()
	p := newTestPlanHandoff()
	p.HandleKey(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Equal(t, 2, p.selectedChoice)
	p.HandleKey(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Equal(t, 0, p.selectedChoice)
	for _, width := range []int{80, 24} {
		scr := uv.NewScreenBuffer(width, p.Height(width))
		p.Draw(scr, image.Rect(0, 0, width, p.Height(width)))
		lines := strings.Split(ansi.Strip(scr.Render()), "\n")
		require.Empty(t, strings.TrimSpace(lines[len(lines)-1]))
	}
}
