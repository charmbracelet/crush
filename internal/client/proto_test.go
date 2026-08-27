package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// TestSubscribeEventsServerEndsStreamClosesEvents pins the ordinary
// end-of-stream case: when the server's handler returns, the reader
// must close the event channel so consumers stop waiting on it.
func TestSubscribeEventsServerEndsStreamClosesEvents(t *testing.T) {
	t.Parallel()

	payload := marshalSSEPayload(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		_, err := fmt.Fprintf(w, "data: %s\n\n", payload)
		require.NoError(t, err)
		flusher.Flush()
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	events, err := c.SubscribeEvents(t.Context(), "ws1")
	require.NoError(t, err)

	select {
	case <-events:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for first event")
	}

	select {
	case _, ok := <-events:
		require.False(t, ok, "event channel must close when the stream ends")
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for event channel close")
	}
}

// TestSubscribeEventsAbruptConnectionLossClosesEvents is the
// regression test for the hang in `crush run`: a server that dies (or
// a dropped connection) mid-frame makes the body read fail with
// something other than io.EOF. The reader used to log that error,
// sleep two seconds and retry the same dead bufio.Reader forever, so
// the event channel never closed and every consumer waiting on it —
// `crush run`'s stream loop and the TUI's resubscribe loop — waited
// for an event that could never arrive.
func TestSubscribeEventsAbruptConnectionLossClosesEvents(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		require.True(t, ok)
		conn, buf, err := hj.Hijack()
		require.NoError(t, err)
		// Send a valid chunked SSE response head plus one chunk
		// holding an incomplete frame (no trailing newline), then
		// drop the socket without the terminating zero-length
		// chunk. That is what a killed server looks like to the
		// client: an unexpected EOF, not a clean one.
		_, _ = buf.WriteString("HTTP/1.1 200 OK\r\n" +
			"Content-Type: text/event-stream\r\n" +
			"Transfer-Encoding: chunked\r\n\r\n")
		const frame = `data: {"type":`
		_, _ = fmt.Fprintf(buf, "%x\r\n%s\r\n", len(frame), frame)
		require.NoError(t, buf.Flush())
		require.NoError(t, conn.Close())
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	events, err := c.SubscribeEvents(t.Context(), "ws1")
	require.NoError(t, err)

	select {
	case _, ok := <-events:
		require.False(t, ok, "event channel must close after an abrupt connection loss")
	case <-time.After(5 * time.Second):
		require.Fail(t, "event channel never closed: the reader is retrying a dead connection")
	}
}

func TestSendMessageAcceptsStatusAccepted(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	require.NoError(t, c.SendMessage(context.Background(), "ws1", "sess1", "", "hello"))
}

func TestSendMessageAcceptsStatusOK(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	require.NoError(t, c.SendMessage(context.Background(), "ws1", "sess1", "", "hello"))
}

func TestSendMessageDecodesErrorBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(proto.Error{Message: "session id is required"})
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	err := c.SendMessage(context.Background(), "ws1", "", "", "hello")
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
	err := c.SendMessage(context.Background(), "ws1", "sess1", "", "hello")
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
	err := c.SendMessage(context.Background(), "ws1", "sess1", "", "hello")
	require.Error(t, err)
	require.Contains(t, err.Error(), "status code 500")
}

// TestAutoApproveSessionPostsSessionID pins the request `crush run`
// makes in client/server mode: the session it is about to drive must be
// named in the body, otherwise the server has nothing to approve and
// the run hangs on the first permission prompt (issue 3648).
func TestAutoApproveSessionPostsSessionID(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath string
	var got proto.PermissionAutoApproveRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	require.NoError(t, c.AutoApproveSession(context.Background(), "ws1", "sess1"))

	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/v1/workspaces/ws1/permissions/auto-approve", gotPath)
	require.Equal(t, "sess1", got.SessionID)
}

func TestAutoApproveSessionNonOKStatusIsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	err := c.AutoApproveSession(context.Background(), "ws1", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "status code 400")
}

// TestRevokeAutoApproveSessionDeletesSession pins the exit call: the
// session is named in the path, so the server drops exactly the hold
// this run took instead of leaving the session auto-approved for
// whichever client keeps the workspace alive afterwards.
func TestRevokeAutoApproveSessionDeletesSession(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	require.NoError(t, c.RevokeAutoApproveSession(context.Background(), "ws1", "sess1"))

	require.Equal(t, http.MethodDelete, gotMethod)
	require.Equal(t, "/v1/workspaces/ws1/permissions/auto-approve/sess1", gotPath)
}

func TestRevokeAutoApproveSessionNonOKStatusIsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := captureClient(t, srv)
	err := c.RevokeAutoApproveSession(context.Background(), "ws1", "sess1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "status code 404")
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
