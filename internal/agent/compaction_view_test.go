package agent

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// seedTurns creates turns user/assistant pairs and returns the messages in
// creation order.
func seedTurns(t *testing.T, env fakeEnv, sessionID string, turns int) []message.Message {
	t.Helper()

	ctx := t.Context()
	created := make([]message.Message, 0, turns*2)
	for i := range turns {
		for _, role := range []message.MessageRole{message.User, message.Assistant} {
			msg, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
				Role:  role,
				Parts: []message.ContentPart{message.TextContent{Text: fmt.Sprintf("%s turn %d", role, i)}},
			})
			require.NoError(t, err)
			created = append(created, msg)
		}
	}
	return created
}

func seedCheckpoint(t *testing.T, env fakeEnv, sessionID string) message.Message {
	t.Helper()

	checkpoint, err := env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:             message.Assistant,
		IsSummaryMessage: true,
		Parts:            []message.ContentPart{message.TextContent{Text: "the checkpoint"}},
	})
	require.NoError(t, err)
	return checkpoint
}

func ids(msgs []message.Message) []string {
	out := make([]string, len(msgs))
	for i, msg := range msgs {
		out[i] = msg.ID
	}
	return out
}

func TestGetSessionMessagesWithoutCheckpointSendsEverything(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	created := seedTurns(t, env, sess.ID, 3)

	view, err := sa.getSessionMessages(ctx, sess)
	require.NoError(t, err)
	require.Equal(t, ids(created), ids(view))
}

func TestGetSessionMessagesPutsCheckpointAheadOfTheRetainedTail(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	// The summarized region predates the checkpoint, and the retained tail
	// predates it too: the read has to reorder rather than slice.
	created := seedTurns(t, env, sess.ID, 4)
	cut := created[6]
	checkpoint := seedCheckpoint(t, env, sess.ID)
	after := seedTurns(t, env, sess.ID, 1)

	sess.SummaryMessageID = checkpoint.ID
	sess.SummaryCutMessageID = cut.ID
	_, err = env.sessions.Save(ctx, sess)
	require.NoError(t, err)

	view, err := sa.getSessionMessages(ctx, sess)
	require.NoError(t, err)

	require.Equal(t, checkpoint.ID, view[0].ID, "the checkpoint leads the context")
	require.Equal(t, message.User, view[0].Role, "the checkpoint is sent as the user turn carrying it")
	want := []string{checkpoint.ID, created[6].ID, created[7].ID}
	for _, msg := range after {
		want = append(want, msg.ID)
	}
	require.Equal(t, want, ids(view))
}

func TestGetSessionMessagesWithoutACutKeepsLegacyBehavior(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedTurns(t, env, sess.ID, 3)
	checkpoint := seedCheckpoint(t, env, sess.ID)
	later := seedTurns(t, env, sess.ID, 1)

	sess.SummaryMessageID = checkpoint.ID
	_, err = env.sessions.Save(ctx, sess)
	require.NoError(t, err)

	view, err := sa.getSessionMessages(ctx, sess)
	require.NoError(t, err)
	require.Equal(t, append([]string{checkpoint.ID}, ids(later)...), ids(view))
	require.Equal(t, message.User, view[0].Role)
}

func TestGetSessionMessagesDoesNotEraseHistoryOnADanglingBoundary(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	created := seedTurns(t, env, sess.ID, 3)
	checkpoint := seedCheckpoint(t, env, sess.ID)

	// The checkpoint survives but the retained-tail boundary is deleted.
	// Showing only part of a session is worse than showing all of it.
	sess.SummaryMessageID = checkpoint.ID
	sess.SummaryCutMessageID = "gone"
	_, err = env.sessions.Save(ctx, sess)
	require.NoError(t, err)
	require.NoError(t, env.messages.Delete(ctx, created[0].ID))

	view, err := sa.getSessionMessages(ctx, sess)
	require.NoError(t, err)
	// Five surviving messages plus the checkpoint: the whole session, not a
	// fragment of it.
	require.Len(t, view, len(created))
}

func TestSplitCheckpoint(t *testing.T) {
	t.Parallel()

	footer := compaction.Info{ReplacedMessages: 3, TranscriptPath: "/tmp/x.txt"}.Render()
	msgs := []message.Message{
		{
			Role:             message.User,
			IsSummaryMessage: true,
			Parts:            []message.ContentPart{message.TextContent{Text: "earlier work\n\n" + footer}},
		},
		{Role: message.User},
		{Role: message.Assistant},
	}

	previous, body := splitCheckpoint(msgs)
	require.Equal(t, "earlier work", previous)
	require.Len(t, body, 2)

	previous, body = splitCheckpoint(msgs[1:])
	require.Empty(t, previous)
	require.Len(t, body, 2)
}
