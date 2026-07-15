package dialog

import (
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func newTestPlanHandoff() *PlanHandoffInline {
	sty := styles.CharmtonePantera()
	return NewPlanHandoffInline(&common.Common{Styles: &sty})
}

func TestPlanHandoffKeepEditingKeyboard(t *testing.T) {
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
	require.Len(t, p.ShortHelp(), 2)

	called := false
	p.OnKeepEditing = func(feedback string) tea.Cmd {
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

func TestPlanHandoffEscapeFromChoiceCloses(t *testing.T) {
	t.Parallel()

	p := newTestPlanHandoff()
	done, cmd := p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.True(t, done)
	require.Nil(t, cmd)
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

func TestPlanHandoffMouseKeepEditing(t *testing.T) {
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

func TestPlanHandoffImplement(t *testing.T) {
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
