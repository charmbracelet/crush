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

// okCmd stands in for the command a production callback returns.
func okCmd() tea.Cmd { return func() tea.Msg { return nil } }

func newTestCheckpointHandoff() *CheckpointHandoffInline {
	sty := styles.CharmtonePantera()
	return NewCheckpointHandoffInline(&common.Common{Styles: &sty})
}

func drawCheckpointHandoff(c *CheckpointHandoffInline, width int) string {
	c.SetWidth(width)
	scr := uv.NewScreenBuffer(width, c.Height(width))
	c.Draw(scr, image.Rect(0, 0, width, c.Height(width)))
	return screenText(scr)
}

// TestCheckpointHandoffShowsEveryChoice: the prompt has to name all three
// answers at once. That is the whole reason it replaced a banner that mentioned
// two of them in passing.
func TestCheckpointHandoffShowsEveryChoice(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(true)

	out := drawCheckpointHandoff(c, 100)

	require.Contains(t, out, "Keep this checkpoint?")
	require.Contains(t, out, "Keep")
	require.Contains(t, out, "Discard")
	require.Contains(t, out, "Redo with notes")
}

// TestCheckpointHandoffNarrowStacksChoices rather than clipping one: a choice
// the user never saw is not a choice.
func TestCheckpointHandoffNarrowStacksChoices(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(true)

	require.Greater(t, c.Height(20), c.Height(100),
		"stacked choices need more lines than a single row")
}

func TestCheckpointHandoffKeepAndDiscardByKeys(t *testing.T) {
	t.Parallel()

	t.Run("k keeps", func(t *testing.T) {
		t.Parallel()

		c := newTestCheckpointHandoff()
		c.SetFocused(true)
		var ran string
		c.OnKeep = func() tea.Cmd { ran = "keep"; return okCmd() }
		c.OnDiscard = func() tea.Cmd { ran = "discard"; return okCmd() }

		done, cmd := c.HandleKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
		require.True(t, done)
		require.NotNil(t, cmd)
		cmd()
		require.Equal(t, "keep", ran)
	})

	t.Run("d discards", func(t *testing.T) {
		t.Parallel()

		c := newTestCheckpointHandoff()
		c.SetFocused(true)
		var ran string
		c.OnKeep = func() tea.Cmd { ran = "keep"; return okCmd() }
		c.OnDiscard = func() tea.Cmd { ran = "discard"; return okCmd() }

		done, cmd := c.HandleKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
		require.True(t, done)
		require.NotNil(t, cmd)
		cmd()
		require.Equal(t, "discard", ran)
	})
}

// TestCheckpointHandoffConfirmRunsTheSelectedChoice covers the row-then-enter
// path, which is how someone without the direct keys answers.
func TestCheckpointHandoffConfirmRunsTheSelectedChoice(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(true)

	kept := false
	c.OnKeep = func() tea.Cmd { kept = true; return okCmd() }

	done, cmd := c.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, done, "Keep is selected by default, so enter answers it")
	require.NotNil(t, cmd)
	cmd()
	require.True(t, kept)

	t.Run("stepping right reaches discard", func(t *testing.T) {
		t.Parallel()

		c := newTestCheckpointHandoff()
		c.SetFocused(true)
		discarded := false
		c.OnDiscard = func() tea.Cmd { discarded = true; return okCmd() }

		c.HandleKey(tea.KeyPressMsg{Code: tea.KeyRight, Text: "right"})
		done, cmd := c.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.True(t, done)
		cmd()
		require.True(t, discarded)
	})

	t.Run("stepping right again reaches the notes editor", func(t *testing.T) {
		t.Parallel()

		c := newTestCheckpointHandoff()
		c.SetFocused(true)
		c.OnDiscard = func() tea.Cmd { t.Fatal("discard must not run"); return nil }

		c.HandleKey(tea.KeyPressMsg{Code: tea.KeyRight, Text: "right"})
		c.HandleKey(tea.KeyPressMsg{Code: tea.KeyRight, Text: "right"})
		done, cmd := c.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.False(t, done, "Redo asks for notes instead of answering")
		require.Nil(t, cmd)
		require.True(t, c.editing)
	})
}

// TestCheckpointHandoffRedoNeedsNotes: an empty redo is a guess, so it must not
// submit, and the prompt stays open rather than silently closing the decision.
func TestCheckpointHandoffRedoNeedsNotes(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(true)

	done, cmd := c.HandleKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.False(t, done, "r opens the notes editor rather than answering")
	require.Nil(t, cmd)
	require.True(t, c.editing)
	require.True(t, c.editor.Focused())
	require.True(t, c.HeightChanged())
	require.Greater(t, c.Height(80), 3)
	require.Len(t, c.ShortHelp(), 3)

	done, cmd = c.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.False(t, done, "empty notes must not submit a redo")
	require.Nil(t, cmd)
	require.True(t, c.editing, "the prompt stays open for notes")
}

func TestCheckpointHandoffRedoSendsNotes(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(true)

	c.HandleKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for _, r := range "keep the memory appendix" {
		c.HandleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	var got string
	c.OnRedo = func(comments string) tea.Cmd { got = comments; return okCmd() }

	done, cmd := c.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, done)
	require.NotNil(t, cmd)
	cmd()
	require.Equal(t, "keep the memory appendix", got)
}

// TestCheckpointHandoffEscapeKeepsTheDecision: dismissing is not answering. The
// staged checkpoint must survive so the palette can still offer keep and
// discard, which is the same contract the plan handoff keeps.
func TestCheckpointHandoffEscapeKeepsTheDecision(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(true)
	c.OnKeep = func() tea.Cmd { t.Fatal("escape must not keep"); return nil }
	c.OnDiscard = func() tea.Cmd { t.Fatal("escape must not discard"); return nil }

	done, cmd := c.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape, Text: "esc"})
	require.False(t, done, "escape collapses without answering")
	require.NotNil(t, cmd)
	_, ok := cmd().(CollapseInlineMsg)
	require.True(t, ok, "escape must collapse the prompt, not resolve it")
}

// TestCheckpointHandoffEscapeLeavesNotesEditing: escape inside the notes editor
// returns to the choices instead of closing the whole prompt, so a mistaken r
// is recoverable.
func TestCheckpointHandoffEscapeLeavesNotesEditing(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(true)
	c.HandleKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.True(t, c.editing)

	done, cmd := c.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape, Text: "esc"})
	require.False(t, done)
	require.Nil(t, cmd, "leaving the editor is not a collapse")
	require.False(t, c.editing, "the choices come back")
}

// TestCheckpointHandoffPasteOnlyWhileEditing keeps pasted text out of the
// choice row, where it would be swallowed.
func TestCheckpointHandoffPasteOnlyWhileEditing(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(true)

	require.Nil(t, c.HandlePaste(tea.PasteMsg{Content: "notes"}),
		"paste outside the notes editor is ignored")

	c.HandleKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	c.HandlePaste(tea.PasteMsg{Content: "keep the rationale"})
	require.Contains(t, c.editor.Value(), "keep the rationale")
}

// TestCheckpointHandoffUnfocusedStillReads: the prompt is often on screen while
// the chat holds focus, so every choice must stay legible rather than blending
// into the background.
func TestCheckpointHandoffUnfocusedStillReads(t *testing.T) {
	t.Parallel()

	c := newTestCheckpointHandoff()
	c.SetFocused(false)

	out := drawCheckpointHandoff(c, 100)
	require.Contains(t, out, "Keep")
	require.Contains(t, out, "Discard")
	require.Contains(t, out, "Redo with notes")
}
