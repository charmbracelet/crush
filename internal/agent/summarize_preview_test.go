package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/stretchr/testify/require"
)

// TestSummarizePreviewLeavesTheSessionAlone is the promise that makes
// previewing safe to offer: the checkpoint is written for reading, and nothing
// about the session changes until it is accepted.
func TestSummarizePreviewLeavesTheSessionAlone(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the previewed checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 8, 5_000)

	preview, err := sa.SummarizePreview(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil)
	require.NoError(t, err)
	require.NotEmpty(t, preview.CheckpointID)
	require.Contains(t, preview.Text, "the previewed checkpoint")
	require.Positive(t, preview.Replaced)

	staged, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, staged.SummaryMessageID, "a preview must not adopt itself")
	require.Empty(t, staged.SummaryCutMessageID)

	view, err := sa.getSessionMessages(ctx, staged)
	require.NoError(t, err)
	require.Contains(t, textOf(view), "user turn 0",
		"the session still reads its whole history while a preview waits")

	// Accepting applies exactly what the preview computed, without a second
	// model call, so the checkpoint the user read is the one that lands.
	require.NoError(t, sa.ConfirmSummarize(ctx, sess.ID, preview))
	accepted, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, preview.CheckpointID, accepted.SummaryMessageID)
	require.Equal(t, preview.CutID, accepted.SummaryCutMessageID)
	require.Equal(t, preview.PromptTokens, accepted.PromptTokens)

	after, err := sa.getSessionMessages(ctx, accepted)
	require.NoError(t, err)
	require.NotContains(t, textOf(after), "user turn 0", "accepting is what drops the older turns")
	require.True(t, after[0].IsSummaryMessage)
	require.Len(t, model.calls, 1, "accepting must not summarize a second time")
}

// TestDiscardSummarizeChangesNothing is the other half: rejecting costs the row
// and leaves the session exactly as it was.
func TestDiscardSummarizeChangesNothing(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "unwanted"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 6, 5_000)

	preview, err := sa.SummarizePreview(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil)
	require.NoError(t, err)
	require.NoError(t, sa.DiscardSummarize(ctx, sess.ID, preview.CheckpointID))

	after, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, after.SummaryMessageID)

	view, err := sa.getSessionMessages(ctx, after)
	require.NoError(t, err)
	require.Contains(t, textOf(view), "user turn 0")
	for _, msg := range view {
		require.False(t, msg.IsSummaryMessage, "the discarded preview leaves no card behind")
	}
}

// TestConfirmSummarizeWithoutAPreview refuses rather than corrupting counters
// with an empty pointer pair.
func TestConfirmSummarizeWithoutAPreview(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "unused"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	require.ErrorContains(t, sa.ConfirmSummarize(ctx, sess.ID, compaction.Preview{}), "nothing to accept")
}
