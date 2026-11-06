package agent

import (
	"context"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// finishReasonModel streams a short text part and ends the step with
// the configured finish reason, counting streams so a test can prove
// an abnormal finish ends the turn instead of being re-requested.
type finishReasonModel struct {
	finishStreamModel
	reason  fantasy.FinishReason
	streams atomic.Int32
}

func (m *finishReasonModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.streams.Add(1)
	text := m.text
	reason := m.reason
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: reason})
	}, nil
}

// TestSummarizeRecordsTruncatedSummary covers the compaction path:
// a summary cut off by the token limit becomes the record the session
// reads from afterwards, so it must not be filed under a clean
// end_turn.
func TestSummarizeRecordsTruncatedSummary(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	large := &finishReasonModel{finishStreamModel: finishStreamModel{text: "a summary that stops"}, reason: fantasy.FinishReasonLength}
	small := &finishStreamModel{text: "title"}
	sa := testSessionAgent(env, large, small, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
	require.NoError(t, err)
	require.NoError(t, sa.Summarize(t.Context(), sess.ID, nil, nil))

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var summary *message.Message
	for i := range msgs {
		if msgs[i].IsSummaryMessage {
			summary = &msgs[i]
		}
	}
	require.NotNil(t, summary, "compaction must leave a summary message")
	require.Equal(t, message.FinishReasonMaxTokens, summary.FinishReason(),
		"a truncated summary says so instead of pretending the turn ended")
}

// TestFinishReasonMapping locks in the OnStepFinish mapping: the
// upstream finish reason must land on the assistant message, abnormal
// finishes must not be retried, and provider-side early stops (error /
// other) must surface as an error finish with banner copy rather than
// a turn that looks like it completed normally.
func TestFinishReasonMapping(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		reason      fantasy.FinishReason
		want        message.FinishReason
		wantMessage bool
	}{
		{"length maps to max_tokens", fantasy.FinishReasonLength, message.FinishReasonMaxTokens, false},
		{"error maps to error", fantasy.FinishReasonError, message.FinishReasonError, true},
		{"other maps to error", fantasy.FinishReasonOther, message.FinishReasonError, true},
		{"stop maps to end_turn", fantasy.FinishReasonStop, message.FinishReasonEndTurn, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := testEnv(t)
			large := &finishReasonModel{finishStreamModel: finishStreamModel{text: "partial"}, reason: tc.reason}
			small := &finishStreamModel{text: "title"}
			sa := testSessionAgent(env, large, small, "system")

			sess, err := env.sessions.Create(t.Context(), "session")
			require.NoError(t, err)
			_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hi"})
			require.NoError(t, err,
				"an abnormal finish is a completed turn, not a stream error")

			msgs, err := env.messages.List(t.Context(), sess.ID)
			require.NoError(t, err)
			var asst *message.Message
			for i := range msgs {
				if msgs[i].Role == message.Assistant {
					asst = &msgs[i]
				}
			}
			require.NotNil(t, asst, "the turn must persist an assistant message")

			finish := asst.FinishPart()
			require.NotNil(t, finish)
			require.Equal(t, tc.want, finish.Reason)
			require.Equal(t, "partial", asst.Content().Text,
				"streamed content up to the early stop is kept")
			require.EqualValues(t, 1, large.streams.Load(),
				"an abnormal finish is not retried")
			if tc.wantMessage {
				require.NotEmpty(t, finish.Message,
					"error-like finishes carry a banner title")
				require.NotEmpty(t, finish.Details,
					"error-like finishes carry banner details")
			}
		})
	}
}
