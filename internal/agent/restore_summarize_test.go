package agent

import (
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// TestRestoreSummarizeBringsBackTheReplacedRegion is the promise undo has to
// keep: a compaction hides text rather than destroying it, so restoring has to
// return the earliest turns character for character, not another description
// of them.
func TestRestoreSummarizeBringsBackTheReplacedRegion(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 8, 5_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil))
	compacted, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, compacted.SummaryMessageID)

	compactedView, err := sa.getSessionMessages(ctx, compacted)
	require.NoError(t, err)
	require.NotContains(t, textOf(compactedView), "user turn 0",
		"the earliest turns are behind the checkpoint while it is in force")

	require.NoError(t, sa.RestoreSummarize(ctx, sess.ID))

	restored, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, restored.SummaryMessageID, "nothing may keep pointing at the discarded checkpoint")
	require.Empty(t, restored.SummaryCutMessageID)
	require.Positive(t, restored.PromptTokens,
		"an uncompressed session is not an empty one: the counters have to come back too")

	view, err := sa.getSessionMessages(ctx, restored)
	require.NoError(t, err)
	require.Contains(t, textOf(view), "user turn 0", "the replaced region is readable again")
	require.Contains(t, textOf(view), "assistant turn 7", "and so is everything the tail kept")

	for _, msg := range view {
		require.False(t, msg.IsSummaryMessage, "the checkpoint row goes with the checkpoint")
	}
}

// TestRestoreSummarizeWithoutACompaction says so rather than quietly clearing
// counters on a session that never compacted.
func TestRestoreSummarizeWithoutACompaction(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "unused"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 2, 10)

	require.ErrorContains(t, sa.RestoreSummarize(ctx, sess.ID), "no compaction to undo")
}

func TestRestoreSummarizeRestoresPreviousCheckpoint(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "checkpoint one"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "very-first-turn-marker"}},
	})
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 8, 5_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil))
	first, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, first.SummaryMessageID)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "between-compactions-marker"}},
	})
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 6, 5_000)
	model.text = "checkpoint two"
	require.NoError(t, sa.Summarize(ctx, sess.ID, sa.Model(), "", fantasy.ProviderOptions{}, nil))
	second, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEqual(t, first.SummaryMessageID, second.SummaryMessageID)
	require.Equal(t, first.SummaryMessageID, second.PrevSummaryMessageID)
	require.Equal(t, first.SummaryCutMessageID, second.PrevSummaryCutMessageID)

	require.NoError(t, sa.RestoreSummarize(ctx, sess.ID))

	restored, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, first.SummaryMessageID, restored.SummaryMessageID)
	require.Equal(t, first.SummaryCutMessageID, restored.SummaryCutMessageID)
	require.Empty(t, restored.PrevSummaryMessageID)
	require.Empty(t, restored.PrevSummaryCutMessageID)

	view, err := sa.getSessionMessages(ctx, restored)
	require.NoError(t, err)
	require.NotEmpty(t, view)
	require.Equal(t, first.SummaryMessageID, view[0].ID)
	require.Contains(t, view[0].Content().Text, "checkpoint one")
	require.Contains(t, textOf(view), "between-compactions-marker")
	require.NotContains(t, textOf(view), "checkpoint two")
	require.NotContains(t, textOf(view), "very-first-turn-marker",
		"undoing the second compaction restores the first checkpoint rather than raw history")
}

func textOf(msgs []message.Message) (all string) {
	for _, msg := range msgs {
		all += msg.Content().Text + "\n"
	}
	return all
}
