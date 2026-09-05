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
		p.SetWidth(12)
		p.editor.SetValue("one two three")
		scr := uv.NewScreenBuffer(12, p.Height(12))
		p.Draw(scr, image.Rect(0, 0, 12, p.Height(12)))

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
	require.Equal(t, 3, p.Height(80))

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

	x, y := planHandoffButtonPoint(t, p, 1)
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

func TestPlanHandoffChoiceCopyHasNoQuestionBadge(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	scr := uv.NewScreenBuffer(80, p.Height(80))
	p.Draw(scr, image.Rect(0, 0, 80, p.Height(80)))
	rendered := strings.TrimSpace(ansi.Strip(scr.Render()))

	require.True(t, strings.HasPrefix(rendered, planHandoffQuestion))
	require.Contains(t, rendered, "Start coding")
	require.Contains(t, rendered, "Revise plan")
	require.NotContains(t, rendered, " ? "+planHandoffQuestion)
}

func TestPlanHandoffCollapsedView(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	scr := uv.NewScreenBuffer(80, p.CollapsedHeight())
	p.DrawCollapsed(scr, image.Rect(0, 0, 80, p.CollapsedHeight()))

	require.True(t, p.ShouldCollapse(80, 30))
	require.Equal(t, "review plan", p.CollapsedHelp())
	require.Contains(t, ansi.Strip(scr.Render()), planHandoffCollapsedPrompt)
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
	p.OnConfirm = func() tea.Cmd {
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
	wantR, wantG, wantB, wantA := want.RGBA()
	gotR, gotG, gotB, gotA := got.RGBA()
	require.Equal(t, [4]uint32{wantR, wantG, wantB, wantA}, [4]uint32{gotR, gotG, gotB, gotA})
}
