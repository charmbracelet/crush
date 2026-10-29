package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

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

	// Overflow pubsub's 4096-event buffer for the stalled subscriber.
	for range 4096 + 64 {
		h.app.SendEvent(pubsub.Event[message.Message]{
			Type:    pubsub.UpdatedEvent,
			Payload: message.Message{ID: "m", SessionID: "s-overflow", Role: message.Assistant},
		})
	}
	close(resume)
	select {
	case <-served:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream of a client that fell behind was left open")
	}
}
