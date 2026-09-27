package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestSendEventAfterContextCancelIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	events := make(chan any, 1)
	require.False(t, sendEvent(ctx, events, "one"))
	require.False(t, sendEvent(ctx, events, "two"))

	select {
	case ev := <-events:
		require.Failf(t, "unexpected event", "event: %v", ev)
	default:
	}
}

func TestSummaryOutcome(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		cancelled bool
	}{
		{"success", http.StatusOK, "", false},
		{"cancelled", http.StatusConflict, `{"cancelled":true}`, true},
		{"unrelated conflict", http.StatusConflict, `{"message":"workspace closing"}`, false},
		{"malformed conflict", http.StatusConflict, `{`, false},
		{"failure", http.StatusInternalServerError, `{"message":"cleanup failed"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/v1/workspaces/workspace/agent/sessions/session/summarize", r.URL.Path)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			err := captureClient(t, server).AgentSummarizeSession(t.Context(), "workspace", "session")
			if tc.cancelled {
				require.ErrorIs(t, err, context.Canceled)
			} else if tc.status == http.StatusOK {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.NotErrorIs(t, err, context.Canceled)
			}
		})
	}
}

func TestSubscribeEventsContextCancelClosesEvents(t *testing.T) {
	t.Parallel()

	payload := marshalSSEPayload(t)
	firstEventSent := make(chan struct{})
	writeSecondEvent := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		_, err := fmt.Fprintf(w, "data: %s\n\n", payload)
		require.NoError(t, err)
		flusher.Flush()
		close(firstEventSent)

		select {
		case <-writeSecondEvent:
		case <-time.After(5 * time.Second):
			return
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := captureClient(t, srv)
	events, err := c.SubscribeEvents(ctx, "ws1")
	require.NoError(t, err)

	select {
	case <-firstEventSent:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for server event")
	}

	select {
	case <-events:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for first event")
	}

	cancel()
	close(writeSecondEvent)

	select {
	case _, ok := <-events:
		require.False(t, ok)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for event channel close")
	}
}

func TestSendMessageWire(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		policy         proto.PermissionRequestPolicy
		hidden         bool
		runID, channel string
		withAttachment bool
	}{
		{name: "plain", policy: proto.PermissionRequestPolicyPrompt},
		{name: "prompt policy", policy: proto.PermissionRequestPolicyPrompt, hidden: true, runID: "run", channel: "channel", withAttachment: true},
		{name: "auto approval", policy: proto.PermissionRequestPolicyAutoApprove, hidden: true, runID: "run", channel: "channel", withAttachment: true},
		{name: "channel only", policy: proto.PermissionRequestPolicyPrompt, channel: "signal"},
		{name: "hidden only", policy: proto.PermissionRequestPolicyPrompt, hidden: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bodies := make(chan []byte, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				bodies <- body
				w.WriteHeader(http.StatusAccepted)
			}))
			defer srv.Close()
			var attachments []message.Attachment
			var wantAttachments []proto.Attachment
			if tc.withAttachment {
				attachments = []message.Attachment{{FilePath: "/tmp/input", FileName: "input", MimeType: "text/plain", Content: []byte("payload")}}
				wantAttachments = []proto.Attachment{{FilePath: "/tmp/input", FileName: "input", MimeType: "text/plain", Content: []byte("payload")}}
			}
			c := captureClient(t, srv)
			ctx := t.Context()
			if tc.hidden {
				ctx = message.WithHiddenUserMessage(ctx)
			}
			require.NoError(t, c.SendMessage(ctx, "workspace", "session", tc.runID, tc.channel, "prompt", tc.policy, attachments...))
			body := <-bodies
			var got proto.AgentMessage
			require.NoError(t, json.Unmarshal(body, &got))
			require.Equal(t, proto.AgentMessage{
				SessionID: "session", RunID: tc.runID, Channel: tc.channel, Prompt: "prompt", HiddenUserMessage: tc.hidden,
				PermissionPolicy: tc.policy, Attachments: wantAttachments,
			}, got)
			if tc.policy == proto.PermissionRequestPolicyPrompt {
				require.NotContains(t, string(body), "permission_policy")
			} else {
				require.Contains(t, string(body), `"permission_policy":"auto_approve"`)
			}
		})
	}
}

func TestSendMessageAcceptsStatusOK(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	require.NoError(t, c.SendMessage(context.Background(), "ws1", "sess1", "", "", "hello", proto.PermissionRequestPolicyPrompt))
}

func TestSendMessageDecodesErrorBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(proto.Error{Message: "session id is required"})
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	err := c.SendMessage(context.Background(), "ws1", "", "", "", "hello", proto.PermissionRequestPolicyPrompt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "status code 400")
	require.Contains(t, err.Error(), "session id is required")
}

func TestSendMessageFallsBackOnMalformedErrorBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	err := c.SendMessage(context.Background(), "ws1", "sess1", "", "", "hello", proto.PermissionRequestPolicyPrompt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "status code 500")
	require.NotContains(t, err.Error(), "not json")
}

func TestSendMessageFallsBackOnEmptyErrorBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	err := c.SendMessage(context.Background(), "ws1", "sess1", "", "", "hello", proto.PermissionRequestPolicyPrompt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "status code 500")
}

func TestSetMainAgentSendsAgentID(t *testing.T) {
	t.Parallel()

	var gotPath string
	var got proto.AgentSetMainRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	require.NoError(t, c.SetMainAgent(context.Background(), "ws1", "plan"))

	require.Equal(t, "/v1/workspaces/ws1/agent/main", gotPath)
	require.Equal(t, "plan", got.AgentID)
}

func TestSetMainAgentPropagatesServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	err := c.SetMainAgent(context.Background(), "ws1", "plan")
	require.Error(t, err)
}

func marshalSSEPayload(t *testing.T) []byte {
	t.Helper()

	eventPayload, err := json.Marshal(pubsub.Event[proto.AgentEvent]{
		Type: pubsub.CreatedEvent,
		Payload: proto.AgentEvent{
			Type: proto.AgentEventTypeResponse,
		},
	})
	require.NoError(t, err)

	payload, err := json.Marshal(pubsub.Payload{
		Type:    pubsub.PayloadTypeAgentEvent,
		Payload: eventPayload,
	})
	require.NoError(t, err)
	return payload
}
