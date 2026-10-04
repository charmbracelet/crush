package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// recordingModel is a [fantasy.LanguageModel] that keeps a rendering of every
// call it receives, so tests can assert on what actually reached the model.
type recordingModel struct {
	calls     []string
	maxOutput []int64
	text      string
	fail      error
}

func (m *recordingModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{}, nil
}

func (m *recordingModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if m.fail != nil {
		return nil, m.fail
	}
	m.calls = append(m.calls, fmt.Sprintf("%+v", call))
	m.maxOutput = append(m.maxOutput, derefMaxTokens(call.MaxOutputTokens))
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

// derefMaxTokens reports 0 when the caller left max_tokens to the provider.
func derefMaxTokens(tokens *int64) int64 {
	if tokens == nil {
		return 0
	}
	return *tokens
}

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

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	updated, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.SummaryMessageID)
	require.NotEmpty(t, updated.SummaryCutMessageID, "the checkpoint must record where the retained tail starts")
	require.Positive(t, updated.PromptTokens, "a compacted session does not start from an empty context")

	require.Len(t, model.calls, 1)
	require.Equal(t, int64(compaction.MaxCheckpointTokens), model.maxOutput[0],
		"the checkpoint request must not inherit an unbounded provider default")
	require.NotContains(t, model.calls[0], "turn 7", "the retained tail must not be re-sent to be summarized")
	require.Contains(t, model.calls[0], "turn 0", "the replaced region is what the checkpoint is built from")

	view, err := sa.getSessionMessages(ctx, updated)
	require.NoError(t, err)
	require.True(t, view[0].IsSummaryMessage)
	require.Equal(t, message.User, view[0].Role)
	require.Contains(t, view[0].Content().Text, "the new checkpoint")
	require.Contains(t, view[0].Content().Text, `<compaction_info replaced_messages=`)
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
	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	// Grow the session and compact again.
	seedSizedTurns(t, env, sess.ID, 2, 5_000)
	model.text = "checkpoint two"
	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))

	require.Len(t, model.calls, 2)
	require.Contains(t, model.calls[1], "<previous_checkpoint>",
		"an existing checkpoint is merged, not compressed a second time")
	require.Contains(t, model.calls[1], "checkpoint one")
	require.NotContains(t, model.calls[1], "<compaction_info",
		"generated bookkeeping is not fed back to the model as prose")
}

func TestSummarizeOnAnEmptySessionDoesNothing(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "should never run"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))
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

func TestSummarizeAppliesUserInstructions(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)
	seedSizedTurns(t, env, sess.ID, 8, 5_000)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "focus on the failing tests", fantasy.ProviderOptions{}, nil))

	require.Len(t, model.calls, 1)
	require.Contains(t, model.calls[0], "<instructions>")
	require.Contains(t, model.calls[0], "focus on the failing tests")
	require.NotContains(t, model.calls[0], "<previous_checkpoint>", "there is no checkpoint to merge yet")
}

// seedToolTurn creates one user turn whose tool call returns a blob of roughly
// tokens tokens, so pruning has something stale enough to be worth removing.
func seedToolTurn(t *testing.T, env fakeEnv, sessionID, label string, tokens int) {
	t.Helper()
	ctx := t.Context()
	call := "call-" + label

	_, err := env.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "read " + label}},
	})
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.ToolCall{ID: call, Name: "view"}},
	})
	require.NoError(t, err)
	_, err = env.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{message.ToolResult{
			ToolCallID: call, Name: "view", Content: strings.Repeat("o", tokens*4),
		}},
	})
	require.NoError(t, err)
}

// TestSummarizeKeepsFullTextWhenItFits is half the design: a checkpoint is the
// last moment the text is in hand, so a region that still fits the request is
// summarized whole rather than from skeletons.
func TestSummarizeKeepsFullTextWhenItFits(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	seedToolTurn(t, env, sess.ID, "old", 60_000)
	seedSizedTurns(t, env, sess.ID, 2, 10)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))
	require.Len(t, model.calls, 1)
	require.Contains(t, model.calls[0], strings.Repeat("o", 4_000))
	require.NotContains(t, model.calls[0], "[tool output pruned:")
}

// TestSummarizeFallsBackToTheSentView covers the other half: a region no
// request can carry is truncated by some providers rather than rejected, which
// would write a checkpoint that never saw most of what it replaced. Pruning at
// least keeps the request honest about what is missing.
func TestSummarizeFallsBackToTheSentView(t *testing.T) {
	env := testEnv(t)
	model := &recordingModel{text: "the checkpoint"}
	sa := testSessionAgent(env, model, nil, "test prompt").(*sessionAgent)
	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	seedToolTurn(t, env, sess.ID, "ancient", 300_000)
	seedSizedTurns(t, env, sess.ID, 2, 10)

	require.NoError(t, sa.Summarize(ctx, sess.ID, "", fantasy.ProviderOptions{}, nil))
	require.Len(t, model.calls, 1)
	require.Contains(t, model.calls[0], "[tool output pruned: view")
	require.NotContains(t, model.calls[0], strings.Repeat("o", 4_000))
}

func TestBuildSummaryPromptOrdersItsSections(t *testing.T) {
	t.Parallel()

	out := buildSummaryPrompt(
		[]session.Todo{{Content: "ship it", Status: session.TodoStatusPending}},
		"focus on the failing tests",
		"earlier work",
		"/data/compaction/sess/1-checkpoint.txt",
	)

	instructions := strings.Index(out, "<instructions>")
	previous := strings.Index(out, "<previous_checkpoint>")
	todos := strings.Index(out, "## Current Todo List")
	pointer := strings.Index(out, "/data/compaction/sess/1-checkpoint.txt")
	require.Less(t, instructions, previous, "the user's emphasis leads")
	require.Less(t, previous, todos, "the merged checkpoint precedes the task list")
	require.Less(t, todos, pointer, "the recovery pointer is last")
	require.Contains(t, out, "ship it")

	// Every block is optional and independent.
	bare := buildSummaryPrompt(nil, "", "", "")
	require.NotContains(t, bare, "<instructions>")
	require.NotContains(t, bare, "<previous_checkpoint>")
	require.NotContains(t, bare, "Current Todo List")
	require.Equal(t, "Write the checkpoint for the conversation above.", bare)
}
