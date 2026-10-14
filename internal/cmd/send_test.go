package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// fakeSendClient records calls; onSend runs inside SendMessage with the
// run ID the code under test minted, so it can answer on the event stream.
type fakeSendClient struct {
	sessions []proto.Session
	events   chan any
	onSend   func(sessionID, runID string)

	queued    []string // GetAgentSessionQueuedPromptsList answer
	queuedErr error

	sent       []string // "sessionID|prompt"
	subscribed bool
	created    string
	agentReady bool
	inits      int
}

func (f *fakeSendClient) GetAgentInfo(context.Context, string) (*proto.AgentInfo, error) {
	return &proto.AgentInfo{IsReady: f.agentReady}, nil
}

func (f *fakeSendClient) InitiateAgentProcessing(context.Context, string, bool) error {
	f.inits++
	return nil
}

func newFakeSendClient(sessions ...proto.Session) *fakeSendClient {
	return &fakeSendClient{sessions: sessions, events: make(chan any, 16)}
}

func (f *fakeSendClient) SendMessage(_ context.Context, _, sessionID, runID, _, prompt string, _ ...message.Attachment) error {
	f.sent = append(f.sent, sessionID+"|"+prompt)
	if f.onSend != nil {
		f.onSend(sessionID, runID)
	}
	return nil
}

func (f *fakeSendClient) SubscribeEvents(context.Context, string) (<-chan any, error) {
	f.subscribed = true
	return f.events, nil
}

func (f *fakeSendClient) ListSessions(context.Context, string) ([]proto.Session, error) {
	return f.sessions, nil
}

func (f *fakeSendClient) CreateSession(_ context.Context, _, title string) (*proto.Session, error) {
	f.created = title
	s := proto.Session{ID: "new-session"}
	f.sessions = append(f.sessions, s)
	return &s, nil
}

func (f *fakeSendClient) GetSession(_ context.Context, _, id string) (*proto.Session, error) {
	for _, s := range f.sessions {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("session not found: %s", id)
}

func (f *fakeSendClient) GetAgentSessionQueuedPromptsList(context.Context, string, string) ([]string, error) {
	return f.queued, f.queuedErr
}

func runComplete(sessionID, runID, text, errText string) pubsub.Event[proto.RunComplete] {
	return pubsub.Event[proto.RunComplete]{Payload: proto.RunComplete{
		SessionID: sessionID, RunID: runID, MessageID: "m", Text: text, Error: errText,
	}}
}

func TestSendNoWaitPrintsSession(t *testing.T) {
	t.Parallel()
	f := newFakeSendClient(proto.Session{ID: "s1"})
	var out bytes.Buffer
	res, err := runSend(t.Context(), f, "ws", sendOptions{sessionID: "s1", prompt: "do it"}, &out)
	require.NoError(t, err)
	require.Equal(t, "s1\n", out.String())
	require.Equal(t, "s1", res.SessionID)
	require.Equal(t, []string{"s1|do it"}, f.sent)
	require.False(t, f.subscribed, "no --wait: never listens for events")
}

func TestSendNewSessionAndPrefix(t *testing.T) {
	t.Parallel()
	f := newFakeSendClient(proto.Session{ID: "long-id-123"})
	var out bytes.Buffer
	res, err := runSend(t.Context(), f, "ws", sendOptions{newSession: true, title: "review", prompt: "p"}, &out)
	require.NoError(t, err)
	require.Equal(t, "review", f.created)
	require.Equal(t, "new-session", res.SessionID)

	prefix := session.HashID("long-id-123")[:6]
	res, err = runSend(t.Context(), f, "ws", sendOptions{sessionID: prefix, prompt: "p"}, &out)
	require.NoError(t, err)
	require.Equal(t, "long-id-123", res.SessionID)
}

func TestSendWaitReturnsOnOwnRunComplete(t *testing.T) {
	t.Parallel()
	f := newFakeSendClient(proto.Session{ID: "s1", PromptTokens: 5, CompletionTokens: 6, Cost: 0.25})
	f.onSend = func(sessionID, runID string) {
		f.events <- runComplete("other", runID, "WRONG SESSION", "")             // other session
		f.events <- runComplete(sessionID, "someone-elses-run", "WRONG RUN", "") // same session, other turn
		f.events <- runComplete(sessionID, runID, "final answer", "")
		f.events <- runComplete(sessionID, runID, "NEVER READ", "")
	}
	var out bytes.Buffer
	res, err := runSend(t.Context(), f, "ws", sendOptions{sessionID: "s1", prompt: "p", wait: true}, &out)
	require.NoError(t, err)
	require.Equal(t, "final answer", res.Text)
	require.Equal(t, "final answer\n", out.String())
	require.Equal(t, runJSONResult{
		SessionID: "s1", Text: "final answer", PromptTokens: 5, CompletionTokens: 6, Cost: 0.25,
	}, res)
}

func TestSendWaitReportsRunErrorAndTimeout(t *testing.T) {
	t.Parallel()
	f := newFakeSendClient(proto.Session{ID: "s1"})
	f.onSend = func(sessionID, runID string) {
		f.events <- runComplete(sessionID, runID, "partial", "boom")
	}
	res, err := runSend(t.Context(), f, "ws", sendOptions{sessionID: "s1", prompt: "p", wait: true}, &bytes.Buffer{})
	require.ErrorContains(t, err, "boom")
	require.Equal(t, "s1", res.SessionID, "session_id survives a failed run")

	// Only foreign completions arrive: --timeout must end the wait.
	f = newFakeSendClient(proto.Session{ID: "s1"})
	f.onSend = func(_, runID string) { f.events <- runComplete("other", runID, "x", "") }
	start := time.Now()
	_, err = runSend(t.Context(), f, "ws", sendOptions{sessionID: "s1", prompt: "p", wait: true, timeout: 50 * time.Millisecond}, &bytes.Buffer{})
	require.ErrorContains(t, err, "timed out")
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestSendUnknownSession(t *testing.T) {
	t.Parallel()
	f := newFakeSendClient(proto.Session{ID: "s1"})
	_, err := runSend(t.Context(), f, "ws", sendOptions{sessionID: "zzzz", prompt: "p"}, &bytes.Buffer{})
	require.ErrorContains(t, err, "not found")
	require.Empty(t, f.sent, "an unknown session must not receive anything")
}

func TestSendRequiresClientServer(t *testing.T) {
	t.Setenv("CRUSH_CLIENT_SERVER", "0")
	require.NoError(t, sendCmd.Flags().Set("new", "true"))
	t.Cleanup(func() {
		f := sendCmd.Flags().Lookup("new")
		_ = f.Value.Set("false")
		f.Changed = false
	})

	err := sendCmd.RunE(sendCmd, []string{"hello"})
	var ue *usageError
	require.True(t, errors.As(err, &ue), "want a usage error (exit 2), got %v", err)
	require.ErrorContains(t, err, "CRUSH_CLIENT_SERVER=1")
}

func TestSendEnsureAgentOnlyWhenNotReady(t *testing.T) {
	t.Parallel()
	f := newFakeSendClient()
	f.agentReady = true
	require.NoError(t, ensureAgent(t.Context(), f, "ws"))
	require.Zero(t, f.inits, "a ready agent must not be re-initialised: that replaces the coordinator")

	f.agentReady = false
	require.NoError(t, ensureAgent(t.Context(), f, "ws"))
	require.Equal(t, 1, f.inits)
}

func TestSendWaitFailsOnPermissionOrQuestionPrompt(t *testing.T) {
	t.Parallel()
	for name, mk := range map[string]func(sid string) any{
		"permission": func(sid string) any {
			return pubsub.Event[proto.PermissionRequest]{Payload: proto.PermissionRequest{SessionID: sid, ToolName: "bash"}}
		},
		"question": func(sid string) any {
			return pubsub.Event[proto.QuestionRequest]{Payload: proto.QuestionRequest{SessionID: sid}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFakeSendClient(proto.Session{ID: "s1"})
			f.onSend = func(sid, _ string) {
				f.events <- mk("other") // another session's prompt: ignored
				f.events <- mk(sid)
			}
			res, err := runSend(t.Context(), f, "ws", sendOptions{sessionID: "s1", prompt: "p", wait: true}, &bytes.Buffer{})
			require.ErrorContains(t, err, "TUI on session s1")
			require.Equal(t, "s1", res.SessionID)
		})
	}
}

// TestSendWaitQueuedBehindOtherTurn: `send --session S --wait` can queue
// behind a TUI turn in S. That turn's permission requests and questions are
// not send's: while its prompt is still queued (or the queue cannot be read)
// send keeps waiting, says so on stderr and returns its own run's text.
func TestSendWaitQueuedBehindOtherTurn(t *testing.T) {
	// Not parallel: swaps runStderr.
	for name, tc := range map[string]struct {
		queued []string
		err    error
	}{
		"prompt queued":    {queued: []string{"tui follow-up", "p"}},
		"queue unreadable": {err: errors.New("boom")},
	} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			prev := runStderr
			runStderr = &stderr
			t.Cleanup(func() { runStderr = prev })

			f := newFakeSendClient(proto.Session{ID: "s1"})
			f.queued, f.queuedErr = tc.queued, tc.err
			f.onSend = func(sid, runID string) {
				f.events <- pubsub.Event[proto.PermissionRequest]{Payload: proto.PermissionRequest{SessionID: sid, ToolName: "bash"}}
				f.events <- pubsub.Event[proto.QuestionRequest]{Payload: proto.QuestionRequest{SessionID: sid}}
				f.events <- runComplete(sid, runID, "final", "")
			}
			var out bytes.Buffer
			res, err := runSend(t.Context(), f, "ws", sendOptions{sessionID: "s1", prompt: "p", wait: true}, &out)
			require.NoError(t, err)
			require.Equal(t, "final\n", out.String())
			require.Equal(t, "final", res.Text)
			require.Equal(t, 2, strings.Count(stderr.String(), "another turn"), stderr.String())
		})
	}
}

func TestSendUsage(t *testing.T) {
	t.Parallel()
	ok := func(p, s, title string, isNew, wait bool, to time.Duration) error {
		return sendUsage(p, s, title, isNew, wait, to)
	}
	require.NoError(t, ok("p", "s1", "", false, false, 0))
	require.NoError(t, ok("p", "", "t", true, true, time.Second))
	require.Error(t, ok("", "s1", "", false, false, 0), "no prompt")
	require.Error(t, ok("p", "", "", false, false, 0), "neither --session nor --new")
	require.ErrorContains(t, ok("p", "s1", "t", false, false, 0), "--title")
	require.ErrorContains(t, ok("p", "s1", "", false, false, time.Second), "--timeout")
}

func TestSendJSONShapes(t *testing.T) {
	t.Parallel()
	b := func(v any) string {
		out, err := json.Marshal(v)
		require.NoError(t, err)
		return string(out)
	}
	require.JSONEq(t, `{"session_id":"s1"}`, b(sendJSON(runJSONResult{SessionID: "s1"}, false)))
	require.JSONEq(t, `{"session_id":"s1","error":"boom","text":"","prompt_tokens":0,"completion_tokens":0,"cost":0}`,
		b(sendJSON(runJSONResult{SessionID: "s1", Error: "boom"}, false)))
	require.JSONEq(t, `{"session_id":"s1","text":"hi","prompt_tokens":1,"completion_tokens":2,"cost":0.5}`,
		b(sendJSON(runJSONResult{SessionID: "s1", Text: "hi", PromptTokens: 1, CompletionTokens: 2, Cost: 0.5}, true)))
}
