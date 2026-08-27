package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// blockingListService holds the nth messages.List call open. That call is
// the observation point for session ownership during a summarize:
// summarizeSession reads the transcript through it, after the frame that
// owns the session has registered but before anything streams.
//
// List is called exactly once per Run and once per summarize, so the call
// index identifies the caller: n=1 is a bare Summarize, n=2 is the
// auto-compaction inside a turn whose own List was n=1. Title generation
// does not read messages, so no goroutine can shift the count.
type blockingListService struct {
	message.Service
	n       int64
	calls   atomic.Int64
	entered chan struct{}
	gate    chan struct{}
}

func (s *blockingListService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	if s.calls.Add(1) == s.n {
		close(s.entered)
		select {
		case <-s.gate:
		case <-ctx.Done():
		}
	}
	return s.Service.List(ctx, sessionID)
}

// seedUserMessage gives the session something to summarize and a user
// text message, which suppresses the title-generation goroutine.
func seedUserMessage(t *testing.T, env fakeEnv, sessionID string) {
	t.Helper()
	_, err := env.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "earlier work"}},
	})
	require.NoError(t, err)
}

// TestSummarize_OwnsSessionBeforeItReadsTheTranscript pins Defect D. The
// exported Summarize used to check IsSessionBusy and only register its
// cancel func three DB round trips later. A run dispatched in that
// window passed its own busy check and started a second turn on the same
// session; whichever registered second overwrote the other's
// *activeCancel, so the loser could no longer be cancelled, its deferred
// CompareAndDelete no-oped, and IsSessionBusy called the session idle
// while it was still streaming.
//
// The busy check and the registration now happen together under the
// per-session dispatch mutex, so the session is owned from before the
// first read: a concurrent run must queue.
func TestSummarize_OwnsSessionBeforeItReadsTheTranscript(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	blocking := &blockingListService{
		Service: env.messages,
		n:       1,
		entered: make(chan struct{}),
		gate:    make(chan struct{}),
	}
	env.messages = blocking

	model := &scriptedStreamModel{
		steps: []scriptedStep{{text: "summary", finishReason: fantasy.FinishReasonStop}},
	}
	sa := testSessionAgent(env, model, fastModel{}, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	seedUserMessage(t, env, sess.ID)

	summarizeErr := make(chan error, 1)
	go func() {
		summarizeErr <- sa.Summarize(t.Context(), SummarizeCall{SessionID: sess.ID})
	}()

	select {
	case <-blocking.entered:
	case <-time.After(5 * time.Second):
		close(blocking.gate)
		t.Fatal("summarize never reached the transcript read")
	}

	require.True(t, sa.IsSessionBusy(sess.ID),
		"the summarize must own the session before it reads the transcript")

	result, runErr := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "prompt",
	})
	require.NoError(t, runErr)
	require.Nil(t, result, "the run must queue behind the summarize, not start its own turn")
	require.Equal(t, 1, sa.QueuedPrompts(sess.ID))

	close(blocking.gate)
	require.NoError(t, <-summarizeErr)

	// The summarize handed the session on to the queued prompt and both
	// have finished, so nothing owns the session any more.
	require.False(t, sa.IsSessionBusy(sess.ID))
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))
}

// TestSummarize_IsCancellableBeforeItStreams is the other half of Defect
// D: registering late did not just admit a second run, it also left the
// summarize itself unreachable. A Cancel arriving while the transcript
// read was in flight found no entry and did nothing, and the summary was
// written anyway.
func TestSummarize_IsCancellableBeforeItStreams(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	blocking := &blockingListService{
		Service: env.messages,
		n:       1,
		entered: make(chan struct{}),
		gate:    make(chan struct{}),
	}
	env.messages = blocking

	model := &scriptedStreamModel{
		steps: []scriptedStep{{text: "summary", finishReason: fantasy.FinishReasonStop}},
	}
	sa := testSessionAgent(env, model, fastModel{}, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	seedUserMessage(t, env, sess.ID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = sa.Summarize(t.Context(), SummarizeCall{SessionID: sess.ID})
	}()

	select {
	case <-blocking.entered:
	case <-time.After(5 * time.Second):
		close(blocking.gate)
		t.Fatal("summarize never reached the transcript read")
	}

	sa.Cancel(sess.ID)
	close(blocking.gate)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("summarize never returned")
	}

	// A cancelled summarize leaves no summary behind: either the stream
	// never ran, or its writes failed on the cancelled context.
	after, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Empty(t, after.SummaryMessageID, "a cancelled summarize must not record a summary")
	require.False(t, sa.IsSessionBusy(sess.ID))
}

// TestRun_InTurnSummarizeKeepsSessionOwnedByTheTurn pins Defect E for the
// auto-compaction path. Run used to drop its activeRequests entry before
// the in-turn summarize so the exported Summarize's busy check would
// pass, which left the session unowned for the summarize's DB reads: the
// turn was still live, yet IsSessionBusy reported idle and a concurrent
// dispatch could start a fully concurrent run on the same session.
//
// The turn now keeps its entry and calls summarizeSession, which does no
// ownership bookkeeping at all.
func TestRun_InTurnSummarizeKeepsSessionOwnedByTheTurn(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	// n=2: the turn's own transcript read is the first List call, the
	// in-turn summarize's is the second.
	blocking := &blockingListService{
		Service: env.messages,
		n:       2,
		entered: make(chan struct{}),
		gate:    make(chan struct{}),
	}
	env.messages = blocking

	// testSessionAgent gives the large model a 200k context window, so
	// the auto-summarize threshold is 40k: a step reporting 180k prompt
	// tokens trips it.
	model := &scriptedStreamModel{
		steps: []scriptedStep{
			{
				text:         "work",
				usage:        fantasy.Usage{InputTokens: 180_000, OutputTokens: 100},
				finishReason: fantasy.FinishReasonStop,
			},
			{text: "summary", finishReason: fantasy.FinishReasonStop},
		},
	}
	sa := testSessionAgent(env, model, fastModel{}, "system").(*sessionAgent)

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)
	seedUserMessage(t, env, sess.ID)

	runErr := make(chan error, 1)
	go func() {
		_, err := sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "fill the context window",
		})
		runErr <- err
	}()

	select {
	case <-blocking.entered:
	case <-time.After(10 * time.Second):
		close(blocking.gate)
		t.Fatal("the turn never reached its in-turn summarize")
	}

	require.True(t, sa.IsSessionBusy(sess.ID),
		"the turn must still own the session while its auto-compaction reads the transcript")

	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "follow-up",
	})
	require.NoError(t, err)
	require.Nil(t, result, "a dispatch during the in-turn summarize must queue, not start a second run")

	close(blocking.gate)
	select {
	case err := <-runErr:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the turn never returned")
	}

	// The turn's end-of-turn handoff ran the queued follow-up, so the
	// session is idle and the queue is empty.
	require.False(t, sa.IsSessionBusy(sess.ID))
	require.Equal(t, 0, sa.QueuedPrompts(sess.ID))
}
