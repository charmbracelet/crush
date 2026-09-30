package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// recordingModel is a [fantasy.LanguageModel] that keeps a rendering of every
// call it receives, so tests can assert on what actually reached the model.
type recordingModel struct {
	calls []string
	text  string
}

func (m *recordingModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{}, nil
}

func (m *recordingModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls = append(m.calls, fmt.Sprintf("%+v", call))
	text := m.text
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: text})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *recordingModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, nil
}

func (m *recordingModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

func (m *recordingModel) Provider() string { return "fake" }
func (m *recordingModel) Model() string    { return "fake-model" }

// seedSizedTurns creates turns user/assistant pairs whose text is about the
// given number of context tokens each.
func seedSizedTurns(t *testing.T, env fakeEnv, sessionID string, turns, tokensPerMessage int) {
	t.Helper()

	body := strings.Repeat("x", tokensPerMessage*4)
	for i := range turns {
		for _, role := range []message.MessageRole{message.User, message.Assistant} {
			_, err := env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
				Role:  role,
				Parts: []message.ContentPart{message.TextContent{Text: fmt.Sprintf("%s turn %d %s", role, i, body)}},
			})
			require.NoError(t, err)
		}
	}
}

func TestSummarizeKeepsRecentTurnsVerbatim(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the new checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	// Eight turns well past the retained-tail budget.
	seedSizedTurns(t, env, sess.ID, 8, 5_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, fantasy.ProviderOptions{}, nil))

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryMessageID)
	require.NotEmpty(t, updated.SummaryCutMessageID, "the checkpoint must record where the retained tail starts")
	require.Positive(t, updated.PromptTokens, "a compacted session does not start from an empty context")

	require.Len(t, model.calls, 1)
	require.NotContains(t, model.calls[0], "turn 7", "the retained tail must not be re-sent to be summarized")
	require.Contains(t, model.calls[0], "turn 0", "the replaced region is what the checkpoint is built from")

	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)
	require.True(t, view[0].IsSummaryMessage)
	require.Equal(t, message.User, view[0].Role)
	require.Contains(t, view[0].Content().Text, "the new checkpoint")
	require.Contains(t, view[0].Content().Text, "<compaction_info>")
	require.NotContains(t, view[0].Content().Text, "Full text of the replaced region",
		"no pointer is promised when no transcript was written")

	// The most recent turn is present character for character, not described.
	last := view[len(view)-1]
	require.Equal(t, message.Assistant, last.Role)
	require.Contains(t, last.Content().Text, "assistant turn 7 "+strings.Repeat("x", 4))
}

func TestSummarizeMergesThePreviousCheckpoint(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "checkpoint one"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 8, 5_000)
	require.NoError(t, sa.Summarize(ctx, sess.ID, fantasy.ProviderOptions{}, nil))

	// Grow the session and compact again.
	seedSizedTurns(t, env, sess.ID, 2, 5_000)
	model.text = "checkpoint two"
	require.NoError(t, sa.Summarize(ctx, sess.ID, fantasy.ProviderOptions{}, nil))

	require.Len(t, model.calls, 2)
	require.Contains(t, model.calls[1], "<previous_checkpoint>",
		"an existing checkpoint is merged, not compressed a second time")
	require.Contains(t, model.calls[1], "checkpoint one")
	require.NotContains(t, model.calls[1], "<compaction_info>",
		"generated bookkeeping is not fed back to the model as prose")
}

func TestSummarizeOnAnEmptySessionDoesNothing(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "should never run"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(ctx, sess.ID, fantasy.ProviderOptions{}, nil))
	require.Empty(t, model.calls)

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Empty(t, updated.SummaryMessageID)
	require.Empty(t, updated.SummaryCutMessageID)
}

func TestCompactionTranscriptDirFollowsTheDataDirectory(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt").(*sessionAgent)

	// Tests build the agent without a config store, which means nowhere to
	// keep displaced transcript.
	require.Empty(t, sa.compactionTranscriptDir())
	require.Equal(t, "/data/compaction", compaction.TranscriptDir("/data"))
	require.Empty(t, compaction.TranscriptDir(""))
}
