package model

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

// The preview methods the checkpoint handoff drives. Each records what it was
// called with, because the point of these tests is which workspace call each
// answer produces - not what the prompt looks like, which the dialog package
// covers.

func (w *testWorkspace) AgentSummarizePreview(_ context.Context, _, instructions string) (compaction.Preview, error) {
	w.previewCalls = append(w.previewCalls, instructions)
	return compaction.Preview{
		CheckpointID: "staged-" + instructions,
		Replaced:     10,
		Kept:         3,
	}, nil
}

func (w *testWorkspace) AgentConfirmSummarize(_ context.Context, _ string, preview compaction.Preview) error {
	w.confirmCalls = append(w.confirmCalls, preview.CheckpointID)
	return nil
}

func (w *testWorkspace) AgentDiscardSummarize(_ context.Context, _, checkpointID string) error {
	w.discardCalls = append(w.discardCalls, checkpointID)
	return nil
}

func newPreviewUI(t *testing.T) (*UI, *testWorkspace, compaction.Preview) {
	t.Helper()

	u, ws := newPlanUI(t, "sess-1")
	preview := compaction.Preview{CheckpointID: "staged-1", Replaced: 120, Kept: 24}
	u.pendingPreview = &preview
	u.state = uiChat
	u.focus = uiFocusEditor
	return u, ws, preview
}

// TestPreviewOpensTheHandoff: a staged checkpoint must ask, not announce. This
// is the wiring that replaced the expiring banner.
func TestPreviewOpensTheHandoff(t *testing.T) {
	t.Parallel()

	u, _, preview := newPreviewUI(t)

	u.openCheckpointHandoff()

	inline, ok := u.activeInline.(*dialog.CheckpointHandoffInline)
	require.True(t, ok, "a previewed checkpoint opens the handoff prompt")
	require.NotNil(t, inline)
	require.Equal(t, uiFocusEditor, u.focus)
	require.False(t, u.textarea.Focused(), "the editor yields to the question")
	require.NotNil(t, u.pendingPreview, "dismissing keeps the decision pending")
	require.Equal(t, preview.CheckpointID, u.pendingPreview.CheckpointID)
}

// TestHandoffWithoutAPreviewOpensNothing: the prompt only makes sense about a
// staged checkpoint. An empty call must not park the editor with no question to
// answer.
func TestHandoffWithoutAPreviewOpensNothing(t *testing.T) {
	t.Parallel()

	u, _ := newPlanUI(t, "sess-1")
	u.state = uiChat
	u.focus = uiFocusEditor
	u.textarea.Focus()

	u.openCheckpointHandoff()

	require.Nil(t, u.activeInline, "no question without a staged checkpoint")
	require.True(t, u.textarea.Focused(), "the editor keeps the textarea")
}

// TestHandoffKeepConfirmsTheStagedCheckpoint.
func TestHandoffKeepConfirmsTheStagedCheckpoint(t *testing.T) {
	t.Parallel()

	u, ws, preview := newPreviewUI(t)
	u.openCheckpointHandoff()
	inline := u.activeInline.(*dialog.CheckpointHandoffInline)

	done, cmd := inline.HandleKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
	require.True(t, done)
	require.NotNil(t, cmd)
	cmd()

	require.Equal(t, []string{preview.CheckpointID}, ws.confirmCalls)
	require.Empty(t, ws.discardCalls)
}

// TestHandoffDiscardDropsTheStagedCheckpoint.
func TestHandoffDiscardDropsTheStagedCheckpoint(t *testing.T) {
	t.Parallel()

	u, ws, preview := newPreviewUI(t)
	u.openCheckpointHandoff()
	inline := u.activeInline.(*dialog.CheckpointHandoffInline)

	_, cmd := inline.HandleKey(tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.NotNil(t, cmd)
	cmd()

	require.Equal(t, []string{preview.CheckpointID}, ws.discardCalls)
	require.Empty(t, ws.confirmCalls)
}

// TestHandoffRedoDiscardsBeforeRegenerating is the part that prevents a real
// leak: previews are transcript rows, so a second pass has to clear the first
// or the session accumulates checkpoints nobody answered, each rendering as a
// summary.
func TestHandoffRedoDiscardsBeforeRegenerating(t *testing.T) {
	t.Parallel()

	u, ws, preview := newPreviewUI(t)
	u.openCheckpointHandoff()
	inline := u.activeInline.(*dialog.CheckpointHandoffInline)

	inline.HandleKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for _, r := range "keep the rationale" {
		inline.HandleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	done, cmd := inline.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, done)
	require.NotNil(t, cmd)
	cmd()

	require.Equal(t, []string{preview.CheckpointID}, ws.discardCalls,
		"the staged checkpoint is dropped before another is written")
	require.Equal(t, []string{"keep the rationale"}, ws.previewCalls,
		"the notes become the next pass's emphasis")
	require.Nil(t, u.pendingPreview,
		"the old decision is cleared so it cannot be answered twice")
}

// TestPreviewMessageOpensTheHandoff drives the real event, since the previous
// versions of these tests passed while the trigger was never wired.
func TestPreviewMessageOpensTheHandoff(t *testing.T) {
	t.Parallel()

	u, _, preview := newPreviewUI(t)
	u.activeInline = nil

	u.Update(previewCheckpointMsg{preview: preview})

	_, ok := u.activeInline.(*dialog.CheckpointHandoffInline)
	require.True(t, ok, "the previewed checkpoint asks in the foreground")
	require.NotNil(t, u.pendingPreview)
	require.Equal(t, preview.CheckpointID, u.pendingPreview.CheckpointID)
}

// TestDiscardMessageClosesTheDecision, so accepting or discarding removes the
// staged preview from the palette's keep and discard rows as well, and
// triggers a session reload to update the chat feed and scroll to bottom.
func TestDiscardMessageClosesTheDecision(t *testing.T) {
	t.Parallel()

	u, _, _ := newPreviewUI(t)
	_, cmd := u.Update(discardPreviewMsg{})

	require.Nil(t, u.pendingPreview)
	require.NotNil(t, cmd, "must reload session to restore feed and bottom scroll")
}
