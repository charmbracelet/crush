package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// pipeListener is a [net.Listener] whose connections are in-memory
// [net.Pipe] pairs handed out by dial. A pipe does no buffering, so a
// client that stops reading blocks the server's next write at once, where
// a socket would first absorb an OS-dependent amount of data.
type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{
		conns:  make(chan net.Conn),
		closed: make(chan struct{}),
	}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

// dial is an [http.Transport] DialContext that connects to the listener.
func (l *pipeListener) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.closed:
		client.Close()
		server.Close()
		return nil, net.ErrClosed
	case <-ctx.Done():
		client.Close()
		server.Close()
		return nil, ctx.Err()
	}
}

// pipeAddr names the pipe listener. The harness's server is a tcp one,
// which only accepts loopback-style Host headers, so requests over the pipe
// name localhost.
type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "localhost" }

// newPipeE2EHarness is [newE2EHarness] served over a [pipeListener], so a
// client that stops reading stalls its stream deterministically on every
// OS.
func newPipeE2EHarness(t *testing.T) *e2eHarness {
	t.Helper()
	h := newE2EHarness(t)

	ln := newPipeListener()
	hs := httptest.NewUnstartedServer(h.srv.Handler())
	hs.Listener.Close()
	hs.Listener = ln
	hs.Start()
	hs.Client().Transport.(*http.Transport).DialContext = ln.dial
	t.Cleanup(hs.Close)

	h.httpSrv = hs
	return h
}

// TestE2E_StalledStreamClosedOnOverflow covers a client that stops reading
// its event stream (a suspended or wedged terminal) for so long that the
// server has to drop events for it, including a must-deliver permission
// request. The server must then end that client's stream, so that the
// client's reconnect re-reports its session and gets the pending prompt
// again, instead of leaving the stream open with a silent gap. Other
// clients' streams are left alone.
func TestE2E_StalledStreamClosedOnOverflow(t *testing.T) {
	t.Parallel()
	h := newPipeE2EHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	const (
		sessionID  = "s-overflow"
		toolCallID = "tc-overflow"
		// bufferedPerSubscriber is how many events pubsub buffers for
		// each subscriber.
		bufferedPerSubscriber = 4096
	)
	cidStalled := uuid.New().String()
	cidHealthy := uuid.New().String()

	// The stalled client opens its stream and then never reads it.
	stalledCtx, stalledCancel := context.WithCancel(ctx)
	t.Cleanup(stalledCancel)
	req, err := http.NewRequestWithContext(stalledCtx, http.MethodGet,
		h.httpSrv.URL+"/v1/workspaces/"+h.workspace.ID+"/events?client_id="+cidStalled, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)

	healthy, cancelHealthy := h.subscribeSSE(t, ctx, h.workspace.ID, cidHealthy)
	var healthyMessages atomic.Int64
	healthyGotRequest := make(chan struct{})
	healthyDone := make(chan struct{})
	go func() {
		defer close(healthyDone)
		var once sync.Once
		for ev := range healthy {
			switch e := ev.(type) {
			case pubsub.Event[proto.Message]:
				healthyMessages.Add(1)
			case pubsub.Event[proto.PermissionRequest]:
				if e.Payload.ToolCallID == toolCallID {
					once.Do(func() { close(healthyGotRequest) })
				}
			}
		}
	}()
	t.Cleanup(func() {
		cancelHealthy()
		<-healthyDone
	})

	h.waitForAttached(t, 2)
	h.setCurrentSession(t, ctx, h.workspace.ID, cidStalled, sessionID)
	h.setCurrentSession(t, ctx, h.workspace.ID, cidHealthy, sessionID)

	// Publish more events than the stalled client's buffer holds. The
	// healthy client catches up after every batch, so only the stalled
	// client falls behind.
	publish := func() {
		h.app.SendEvent(pubsub.Event[message.Message]{
			Type: pubsub.UpdatedEvent,
			Payload: message.Message{
				ID:        "m-overflow",
				SessionID: sessionID,
				Role:      message.Assistant,
				Parts:     []message.ContentPart{message.TextContent{Text: strings.Repeat("x", 1024)}},
			},
		})
	}
	const total = bufferedPerSubscriber + 64
	for sent := 0; sent < total; {
		for range min(512, total-sent) {
			publish()
			sent++
		}
		require.Eventually(t, func() bool {
			return healthyMessages.Load() >= int64(sent)
		}, 10*time.Second, time.Millisecond, "the healthy client must keep up")
	}

	// A permission request raised now cannot reach the stalled client.
	reqDone := make(chan struct{})
	go func() {
		defer close(reqDone)
		_, _ = h.app.Permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  toolCallID,
			ToolName:    "view",
			Description: "read a file",
			Action:      "read",
			Path:        h.workspace.Path,
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-reqDone
	})
	select {
	case <-healthyGotRequest:
	case <-time.After(10 * time.Second):
		t.Fatal("the healthy client must receive the permission request")
	}
	// Stay stalled past the broker's 50ms must-deliver timeout, as a
	// suspended client does, so the request is dropped for it.
	time.Sleep(250 * time.Millisecond)

	// The stalled client resumes reading. The server dropped events for
	// it, so it must end the stream rather than carry on with a gap.
	var (
		readErr           error
		stalledMessages   int
		stalledGotRequest bool
	)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				readErr = err
				return
			}
			data, ok := bytes.CutPrefix(bytes.TrimSpace(line), []byte("data:"))
			if !ok {
				continue
			}
			var p pubsub.Payload
			if json.Unmarshal(bytes.TrimSpace(data), &p) != nil {
				continue
			}
			ev, _ := decodeSSEEnvelope(p)
			switch e := ev.(type) {
			case pubsub.Event[proto.Message]:
				stalledMessages++
			case pubsub.Event[proto.PermissionRequest]:
				stalledGotRequest = stalledGotRequest || e.Payload.ToolCallID == toolCallID
			}
		}
	}()
	select {
	case <-readDone:
	case <-time.After(10 * time.Second):
		stalledCancel()
		<-readDone
		t.Fatalf("the server kept the stream open after dropping events for it: "+
			"read %d of %d messages, permission request delivered: %v",
			stalledMessages, total, stalledGotRequest)
	}
	require.ErrorIs(t, readErr, io.EOF, "the server must end the stream cleanly")

	// Reconnecting and re-reporting the session, as the client does after
	// any lost stream, delivers the still-pending request.
	resumed, cancelResumed := h.subscribeSSE(t, ctx, h.workspace.ID, cidStalled)
	t.Cleanup(cancelResumed)
	h.waitForAttached(t, 2)
	h.setCurrentSession(t, ctx, h.workspace.ID, cidStalled, sessionID)
	pickCtx, pickCancel := context.WithTimeout(ctx, 10*time.Second)
	defer pickCancel()
	_, ok := drainUntil(pickCtx, resumed, func(e pubsub.Event[proto.PermissionRequest]) bool {
		return e.Payload.ToolCallID == toolCallID
	})
	require.True(t, ok, "the reconnected client must receive the pending permission request")

	// The healthy client's stream was never closed.
	publish()
	require.Eventually(t, func() bool {
		return healthyMessages.Load() > total
	}, 10*time.Second, time.Millisecond, "the healthy client's stream must stay open")
}
