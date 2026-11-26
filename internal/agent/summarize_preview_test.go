package agent

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/hooks"
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

	preview, err := sa.SummarizePreview(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil)
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
	require.NotContains(t, textOf(view), "the previewed checkpoint",
		"an unadopted preview must never be sent to the model")

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

	preview, err := sa.SummarizePreview(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil)
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

func TestAbandonedPreviewIsCleanedUpOnNextTurnAndShutdown(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "stray preview text"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 6, 5_000)

	preview, err := sa.SummarizePreview(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil)
	require.NoError(t, err)

	// Continuing to chat without answering discards the staged preview and
	// never sends it to the model.
	_, err = sa.Run(ctx, SessionAgentCall{SessionID: sess.ID, Prompt: "keep going"})
	require.NoError(t, err)
	_, err = env.messages.Get(ctx, preview.CheckpointID)
	require.Error(t, err, "abandoned preview row must be removed on next turn")
	require.NotContains(t, model.calls[len(model.calls)-1], "stray preview text")

	// Shutting down while a preview is staged cleans it up too.
	preview2, err := sa.SummarizePreview(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil)
	require.NoError(t, err)
	sa.CancelAll()
	_, err = env.messages.Get(ctx, preview2.CheckpointID)
	require.Error(t, err, "abandoned preview row must be removed on shutdown")
}

func TestConfirmSummarizeRunsPostCompactHookAndMemoryDecay(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "previewed checkpoint"}
	sa := attachMemory(testSessionAgent(env, model, nil, "test prompt"), env)
	readHooks := recordingHooks(t, sa, hooks.EventPreCompact, hooks.EventPostCompact)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 6, 5_000)

	// Seed enough low-relevance memory entries to exceed the memory budget
	// so decay retires some when the checkpoint is accepted.
	big := strings.Repeat("filler ", 400)
	entries := make([]compaction.Entry, 0, 25)
	for i := range 25 {
		entries = append(entries, compaction.Entry{
			Kind:      compaction.KindObservation,
			Relevance: compaction.RelevanceContext,
			Text:      fmt.Sprintf("low-value note %d %s", i, big),
		})
	}
	_, err = env.ledger.Append(ctx, sess.ID, entries)
	require.NoError(t, err)
	beforeCount, err := env.ledger.Count(ctx, sess.ID)
	require.NoError(t, err)

	preview, err := sa.SummarizePreview(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"PreCompact manual"}, readHooks(),
		"PostCompact must wait until the preview is accepted")
	stagedCount, err := env.ledger.Count(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, beforeCount, stagedCount, "previewing must not decay memory")

	require.NoError(t, sa.ConfirmSummarize(ctx, sess.ID, preview))
	require.Equal(t, []string{"PreCompact manual", "PostCompact manual"}, readHooks(),
		"accepting a preview must fire PostCompact")
	afterCount, err := env.ledger.Count(ctx, sess.ID)
	require.NoError(t, err)
	require.Greater(t, afterCount, beforeCount, "accepting a preview must record memory decay")
}
