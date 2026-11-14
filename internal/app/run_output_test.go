package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func assistantReply(id, sessionID, text string) message.Message {
	return message.Message{ID: id, SessionID: sessionID, Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: text}}}
}

func userPrompt(id, sessionID, text string) message.Message {
	return message.Message{ID: id, SessionID: sessionID, Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: text}}}
}

// storedMessages is a messageLister that returns the messages and the
// error it holds.
type storedMessages struct {
	msgs []message.Message
	err  error
}

func (s storedMessages) List(context.Context, string) ([]message.Message, error) {
	return s.msgs, s.err
}

// newTestPrinter returns a printer for a new session "s".
func newTestPrinter(t *testing.T, out *bytes.Buffer) *answerPrinter {
	t.Helper()
	p, err := newAnswerPrinter(t.Context(), out, "s", storedMessages{}, false)
	require.NoError(t, err)
	return p
}

func TestAnswerPrinter_PrintsEachPartOnce(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)

	for _, text := range []string{"  Hel", "  Hello", "  Hello, world."} {
		p.write(assistantReply("m1", "s", text))
	}
	p.write(assistantReply("m2", "other", "not ours"))
	p.write(userPrompt("u", "s", "prompt"))
	require.Equal(t, "Hello, world.", out.String(), "leading spaces are trimmed, each addition is printed once, and other sessions and user messages are left out")
}

func TestAnswerPrinter_SkipsLeadingBlankReplies(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)
	p.write(assistantReply("m1", "s", " \n"))
	p.write(assistantReply("m2", "s", "Done."))
	require.Equal(t, "Done.", out.String())
}

// TestAnswerPrinter_LeadingBlankLines checks that leading blank lines and
// spaces are dropped until something is printed, whether the reply arrives
// in pieces or whole, as it does from the store.
func TestAnswerPrinter_LeadingBlankLines(t *testing.T) {
	t.Parallel()
	for name, texts := range map[string][]string{
		"whole":               {"\nHello"},
		"pieces":              {"\n", "\nHello"},
		"more blank lines":    {"\n", "\n\nHello"},
		"spaces":              {" ", "  Hello"},
		"blank then indented": {"\n", "\n  Hello"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			p := newTestPrinter(t, &out)
			for _, text := range texts {
				p.write(assistantReply("m", "s", text))
			}
			require.Equal(t, "Hello", out.String())
		})
	}
}

// TestAnswerPrinter_RetryReplacesReply covers a provider retry that resets
// the reply after part of it has streamed. The new text is printed in full
// on a new line, whether it is shorter or longer than the first attempt.
func TestAnswerPrinter_RetryReplacesReply(t *testing.T) {
	t.Parallel()
	for name, retry := range map[string]string{
		"shorter": "Real.",
		"longer":  "The real answer, which is longer than the first attempt.",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			p := newTestPrinter(t, &out)
			p.write(assistantReply("m", "s", "A first attempt that"))
			// The reset update has no parts, so it is not a reply.
			p.write(message.Message{ID: "m", SessionID: "s", Role: message.Assistant})
			p.write(assistantReply("m", "s", retry))
			require.Equal(t, "A first attempt that\n"+retry, out.String())
		})
	}
}

// TestAnswerPrinter_RetryWithSameStartContinues covers a retry that starts
// with the same text as the failed attempt. The output continues from
// where it stopped.
func TestAnswerPrinter_RetryWithSameStartContinues(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)
	p.write(assistantReply("m", "s", "The ans"))
	p.write(assistantReply("m", "s", "The answer is 42."))
	require.Equal(t, "The answer is 42.", out.String())
}

// TestAnswerPrinter_StaleUpdateAddsNothing checks that an update older
// than one already printed adds nothing.
func TestAnswerPrinter_StaleUpdateAddsNothing(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)
	p.write(assistantReply("m", "s", "Hello world"))
	p.write(assistantReply("m", "s", "Hello"))
	p.write(assistantReply("m", "s", "Hello world!"))
	require.Equal(t, "Hello world!", out.String())
}

// TestAnswerPrinter_FlushPrintsWhatTheEventsMissed covers the race: the
// agent reports done before the events carrying the end of its reply have
// been read. flush then prints the missing end from the store.
func TestAnswerPrinter_FlushPrintsWhatTheEventsMissed(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)
	p.write(assistantReply("tool-turn", "s", "Let me check."))
	p.write(assistantReply("final", "s", "TOOL SAID: On"))

	stored := storedMessages{msgs: []message.Message{
		userPrompt("u", "s", "check the status"),
		assistantReply("tool-turn", "s", "Let me check."),
		assistantReply("final", "s", "TOOL SAID: On branch master"),
	}}
	p.flush(t.Context(), stored)
	require.Equal(t, "Let me check.TOOL SAID: On branch master", out.String(), "only the missing end is added")

	p.flush(t.Context(), stored)
	require.Equal(t, "Let me check.TOOL SAID: On branch master", out.String(), "flushing again adds nothing")
}

func TestAnswerPrinter_FlushWithNoEventsAtAll(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	newTestPrinter(t, &out).flush(t.Context(), storedMessages{msgs: []message.Message{assistantReply("final", "s", "  The whole answer.")}})
	require.Equal(t, "The whole answer.", out.String())
}

// TestAnswerPrinter_FlushContinuesPastReplacedReply checks that a reply
// replaced by a retry does not stop flush from printing the replies after
// it.
func TestAnswerPrinter_FlushContinuesPastReplacedReply(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)
	p.write(assistantReply("m1", "s", "partial answer from failed attempt"))
	p.flush(t.Context(), storedMessages{msgs: []message.Message{
		assistantReply("m1", "s", "retry."),
		assistantReply("m2", "s", "FINAL ANSWER"),
	}})
	require.Equal(t, "partial answer from failed attempt\nretry.FINAL ANSWER", out.String())
}

func TestAnswerPrinter_FlushIgnoresAListError(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)
	p.write(assistantReply("m", "s", "Partial"))
	p.flush(t.Context(), storedMessages{err: errors.New("database is closed")})
	require.Equal(t, "Partial", out.String(), "what was already printed stays, and the list error does not fail the run")
}

// TestAnswerPrinter_SkipsSummaries checks that a compaction summary is not
// printed, from its events or from the store, wherever it falls among the
// replies. It is stored as an assistant message in the run's session, but
// it is not part of the answer.
func TestAnswerPrinter_SkipsSummaries(t *testing.T) {
	t.Parallel()
	summary := func(id string) message.Message {
		msg := assistantReply(id, "s", "SUMMARY")
		msg.IsSummaryMessage = true
		return msg
	}
	msgs := []message.Message{
		assistantReply("m1", "s", "Before."), summary("mid"),
		assistantReply("m2", "s", "After."), summary("last"),
	}

	var fromEvents, fromStore bytes.Buffer
	p := newTestPrinter(t, &fromEvents)
	for _, msg := range msgs {
		p.write(msg)
	}
	newTestPrinter(t, &fromStore).flush(t.Context(), storedMessages{msgs: msgs})
	require.Equal(t, "Before.After.", fromEvents.String())
	require.Equal(t, "Before.After.", fromStore.String())
}

// TestNewAnswerPrinter_ContinuedSession checks that a run in a continued
// session prints only what it added. flush reads the whole session, which
// also holds the replies of earlier runs.
func TestNewAnswerPrinter_ContinuedSession(t *testing.T) {
	t.Parallel()
	earlier := []message.Message{
		userPrompt("u1", "s", "first prompt"),
		assistantReply("a1", "s", "The earlier answer."),
	}
	var out bytes.Buffer
	p, err := newAnswerPrinter(t.Context(), &out, "s", storedMessages{msgs: earlier}, true)
	require.NoError(t, err)

	p.write(assistantReply("a2", "s", "  The new"))
	p.flush(t.Context(), storedMessages{msgs: append(earlier,
		userPrompt("u2", "s", "second prompt"),
		assistantReply("a2", "s", "  The new answer."),
	)})
	require.Equal(t, "The new answer.", out.String(), "earlier replies are not repeated, and the new reply is trimmed as the first reply of this run")
}

// TestNewAnswerPrinter_ListsOnlyContinuedSessions checks that a new
// session is not read from the store. A continued session is, and the run
// fails if that read fails.
func TestNewAnswerPrinter_ListsOnlyContinuedSessions(t *testing.T) {
	t.Parallel()
	broken := storedMessages{err: errors.New("database is closed")}
	_, err := newAnswerPrinter(t.Context(), &bytes.Buffer{}, "s", broken, false)
	require.NoError(t, err)
	_, err = newAnswerPrinter(t.Context(), &bytes.Buffer{}, "s", broken, true)
	require.ErrorContains(t, err, "database is closed")
}

// TestAnswerPrinter_PrintUntilDoneWhenDoneWins checks that the whole answer
// is printed from the store when done is ready and no events ever arrive.
func TestAnswerPrinter_PrintUntilDoneWhenDoneWins(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)
	done := make(chan error, 1)
	done <- nil
	stopped := 0

	err := p.printUntilDone(t.Context(), make(chan pubsub.Event[message.Message]), done,
		storedMessages{msgs: []message.Message{assistantReply("final", "s", "The whole answer.")}},
		func() {}, func() { stopped++ })
	require.NoError(t, err)
	require.Equal(t, "The whole answer.", out.String())
	require.Positive(t, stopped, "the spinner is stopped")
}

func TestAnswerPrinter_PrintUntilDoneAfterEvents(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := newTestPrinter(t, &out)
	events := make(chan pubsub.Event[message.Message], 1)
	events <- pubsub.Event[message.Message]{Type: pubsub.UpdatedEvent, Payload: assistantReply("final", "s", "The whole")}
	done := make(chan error, 1)
	waits := 0
	beforeWait := func() {
		// Make done ready only after the event has been read.
		if waits++; waits == 2 {
			done <- nil
		}
	}

	err := p.printUntilDone(t.Context(), events, done,
		storedMessages{msgs: []message.Message{assistantReply("final", "s", "The whole answer.")}},
		beforeWait, func() {})
	require.NoError(t, err)
	require.Equal(t, "The whole answer.", out.String())
}

func TestAnswerPrinter_PrintUntilDoneErrors(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"failure":         {err: errors.New("provider down"), want: "agent processing failed: provider down"},
		"context cancel":  {err: context.Canceled},
		"request cancel":  {err: agent.ErrRequestCancelled},
		"wrapped cancels": {err: errors.Join(errors.New("stream"), context.Canceled)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			p := newTestPrinter(t, &out)
			done := make(chan error, 1)
			done <- tc.err

			err := p.printUntilDone(t.Context(), nil, done,
				storedMessages{msgs: []message.Message{assistantReply("m", "s", "Partial")}},
				func() {}, func() {})
			if tc.want == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.want)
			}
			require.Equal(t, "Partial", out.String(), "what the agent stored is printed before the run ends")
		})
	}
}

func TestAnswerPrinter_PrintUntilDoneContextDone(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	p := newTestPrinter(t, &bytes.Buffer{})
	err := p.printUntilDone(ctx, nil, nil, storedMessages{}, func() {}, func() {})
	require.ErrorIs(t, err, context.Canceled)
}

// TestAnswerPrinter_MessageService runs the printer against the real
// message service. It stores the messages of a run and calls FlushAll, as
// the agent does before it reports done, then reads a single event. flush
// must print the rest, must not repeat the earlier reply of the continued
// session, and must leave out the compaction summary.
func TestAnswerPrinter_MessageService(t *testing.T) {
	t.Parallel()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	q := db.New(conn)
	sess, err := session.NewService(q, conn).Create(t.Context(), "test")
	require.NoError(t, err)
	msgs := message.NewService(q)

	// An earlier run's reply.
	old, err := msgs.Create(t.Context(), sess.ID, message.CreateMessageParams{Role: message.Assistant})
	require.NoError(t, err)
	old.AppendContent("The earlier answer.")
	old.AddFinish(message.FinishReasonEndTurn, "", "")
	require.NoError(t, msgs.Update(t.Context(), old))

	var out bytes.Buffer
	p, err := newAnswerPrinter(t.Context(), &out, sess.ID, msgs, true)
	require.NoError(t, err)
	events := msgs.Subscribe(t.Context())

	// The run: two assistant messages, streamed in debounced deltas, with
	// a compaction summary between them.
	for i, text := range []string{"Let me check. ", "The answer is forty-two."} {
		if i == 1 {
			summary, err := msgs.Create(t.Context(), sess.ID, message.CreateMessageParams{Role: message.Assistant, IsSummaryMessage: true})
			require.NoError(t, err)
			summary.AppendContent("The summary.")
			summary.AddFinish(message.FinishReasonEndTurn, "", "")
			require.NoError(t, msgs.Update(t.Context(), summary))
		}
		m, err := msgs.Create(t.Context(), sess.ID, message.CreateMessageParams{Role: message.Assistant})
		require.NoError(t, err)
		for _, r := range text {
			m.AppendContent(string(r))
			require.NoError(t, msgs.Update(t.Context(), m))
		}
		m.AddFinish(message.FinishReasonEndTurn, "", "")
		require.NoError(t, msgs.Update(t.Context(), m))
	}
	require.NoError(t, msgs.FlushAll(t.Context()))

	// Read only the first event, as if done had then won the select.
	p.write((<-events).Payload)
	p.flush(t.Context(), msgs)
	require.Equal(t, "Let me check. The answer is forty-two.", out.String())
}
