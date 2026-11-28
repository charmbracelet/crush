package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// deadlineWriter is a ResponseWriter whose writes block, like a client
// that stopped reading its stream and never resumes, until a write
// deadline in the past is set on it.
type deadlineWriter struct {
	header  http.Header
	expired chan struct{}
	once    sync.Once
}

func (w *deadlineWriter) Header() http.Header { return w.header }
func (w *deadlineWriter) WriteHeader(int)     {}
func (w *deadlineWriter) Write([]byte) (int, error) {
	<-w.expired
	return 0, os.ErrDeadlineExceeded
}

func (w *deadlineWriter) SetWriteDeadline(d time.Time) error {
	if !d.IsZero() && !d.After(time.Now()) {
		w.once.Do(func() { close(w.expired) })
	}
	return nil
}

// TestE2E_BlockedStreamClosedOnOverflow guards against a handler that
// stays blocked writing to a client that never resumes reading, which
// keeps its subscription attached after events were dropped for it.
func TestE2E_BlockedStreamClosedOnOverflow(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	served := make(chan struct{})
	w := &deadlineWriter{header: http.Header{}, expired: make(chan struct{})}
	t.Cleanup(func() {
		cancel()
		// Release a write the handler is still blocked in, so a failing
		// run ends instead of hanging.
		_ = w.SetWriteDeadline(time.Now())
		<-served
	})

	cid := uuid.New().String()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/v1/workspaces/"+h.workspace.ID+"/events?client_id="+cid, nil)
	req.Host = "localhost"
	go func() {
		defer close(served)
		h.srv.Handler().ServeHTTP(w, req)
	}()
	h.waitForAttached(t, 1)
	h.setCurrentSession(t, ctx, h.workspace.ID, cid, "s-blocked")

	// Overflow the stalled subscriber's buffer with events that cannot
	// be coalesced. The client never resumes reading.
	for i := range 4096 + 64 {
		h.app.SendEvent(pubsub.Event[message.Message]{
			Type:    pubsub.UpdatedEvent,
			Payload: message.Message{ID: fmt.Sprintf("m-%d", i), SessionID: "s-blocked", Role: message.Assistant},
		})
	}
	select {
	case <-served:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream of a client that never resumed was left open")
	}
	require.Zero(t, backend.WorkspaceLiveStreamCountForTest(h.workspace))
}

// stalledWriter is a ResponseWriter whose writes block until resume is
// closed, like a client that stopped reading its stream.
type stalledWriter struct {
	header http.Header
	resume <-chan struct{}
}

func (w *stalledWriter) Header() http.Header { return w.header }
func (w *stalledWriter) WriteHeader(int)     {}
func (w *stalledWriter) Write(p []byte) (int, error) {
	<-w.resume
	return len(p), nil
}

// TestE2E_StalledStreamClosedOnOverflow guards against keeping a client's
// event stream open with a silent gap after events were dropped for it.
func TestE2E_StalledStreamClosedOnOverflow(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx, cancel := context.WithCancel(t.Context())
	resume := make(chan struct{})
	served := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		select {
		case <-resume:
		default:
			close(resume)
		}
		<-served
	})

	cid := uuid.New().String()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/v1/workspaces/"+h.workspace.ID+"/events?client_id="+cid, nil)
	req.Host = "localhost"
	go func() {
		defer close(served)
		h.srv.Handler().ServeHTTP(&stalledWriter{header: http.Header{}, resume: resume}, req)
	}()
	h.waitForAttached(t, 1)
	h.setCurrentSession(t, ctx, h.workspace.ID, cid, "s-overflow")

	// Overflow pubsub's 4096-event buffer for the stalled subscriber. Each
	// event updates a different message, so the broker cannot coalesce them.
	for i := range 4096 + 64 {
		h.app.SendEvent(pubsub.Event[message.Message]{
			Type:    pubsub.UpdatedEvent,
			Payload: message.Message{ID: fmt.Sprintf("m-%d", i), SessionID: "s-overflow", Role: message.Assistant},
		})
	}
	close(resume)
	select {
	case <-served:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream of a client that fell behind was left open")
	}
}
