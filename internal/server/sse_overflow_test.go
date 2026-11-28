package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// stalledWriter is a ResponseWriter whose writes block until resume is
// closed, like a client that stopped reading its stream. writing is
// closed when the first write starts blocking.
type stalledWriter struct {
	header  http.Header
	resume  <-chan struct{}
	writing chan struct{}
	once    sync.Once
}

func (w *stalledWriter) Header() http.Header { return w.header }
func (w *stalledWriter) WriteHeader(int)     {}
func (w *stalledWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.writing) })
	<-w.resume
	return len(p), nil
}

// countConfigChanged counts the config_changed events of evc until it
// closes, publishing the running count on counts and passing other events
// on to other.
func countConfigChanged(evc <-chan rawSSEEvent, counts chan<- int, other chan<- rawSSEEvent) {
	n := 0
	for ev := range evc {
		if ev.Payload.Type == pubsub.PayloadTypeConfigChanged {
			n++
			counts <- n
			continue
		}
		select {
		case other <- ev:
		default:
		}
	}
}

// TestE2E_StalledStreamClosedOnOverflow guards against keeping a client's
// event stream open with a silent gap after events were dropped for it,
// and against a stalled stream holding back the others: it fills one
// stream's queue exactly, then publishes a must-deliver event, which
// overflows the stalled stream while a healthy stream keeps receiving.
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

	stalled := &stalledWriter{header: http.Header{}, resume: resume, writing: make(chan struct{})}
	req := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/v1/workspaces/"+h.workspace.ID+"/events?client_id="+uuid.New().String(), nil)
	req.Host = "localhost"
	go func() {
		defer close(served)
		h.srv.Handler().ServeHTTP(stalled, req)
	}()
	h.waitForAttached(t, 1)

	healthyEvents, cancelHealthy := h.subscribeRawSSE(t, ctx, uuid.New().String(), "", false)
	t.Cleanup(cancelHealthy)
	h.waitForAttached(t, 2)
	counts := make(chan int, 1<<13)
	other := make(chan rawSSEEvent, 16)
	go countConfigChanged(healthyEvents, counts, other)
	waitCount := func(want int) {
		t.Helper()
		timeout := time.After(10 * time.Second)
		for {
			select {
			case n := <-counts:
				if n >= want {
					return
				}
			case <-timeout:
				require.FailNow(t, "healthy stream stopped receiving", "want %d events", want)
			}
		}
	}

	// The stalled handler takes the first event and blocks writing it.
	h.configChanged("first")
	select {
	case <-stalled.writing:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the stalled stream never started writing")
	}
	waitCount(1)

	// Then fill its queue exactly, in batches the healthy stream drains,
	// so nothing but the stalled queue backs up.
	const queue, batch = 4096, 256
	sent := 1
	for sent < 1+queue {
		for range batch {
			h.configChanged("filler")
		}
		sent += batch
		waitCount(sent)
	}

	// A must-deliver event cannot fit: the stalled stream overflows at
	// once, and the healthy one gets it.
	h.app.RunCompletions().PublishMustDeliver(ctx, pubsub.CreatedEvent, notify.RunComplete{SessionID: "s", RunID: "r"})
	select {
	case ev := <-other:
		require.Equal(t, pubsub.PayloadTypeRunComplete, ev.Payload.Type)
		require.NotEmpty(t, ev.ID)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the healthy stream did not get the must-deliver event")
	}
	h.configChanged("after")
	waitCount(sent + 1)

	close(resume)
	select {
	case <-served:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream of a client that fell behind was left open")
	}

	// The healthy stream is still attached and live.
	h.configChanged("still")
	waitCount(sent + 2)
}
