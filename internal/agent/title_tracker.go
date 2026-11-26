package agent

import (
	"context"
	"sync"
	"time"
)

// titleTimeout bounds one title generation so a stalled request ends.
const titleTimeout = 60 * time.Second

// titleTracker tracks in-flight title generations so shutdown can stop them
// and wait for their last database write. The zero value is ready to use.
type titleTracker struct {
	once     sync.Once
	stopCtx  context.Context
	stopFunc context.CancelFunc

	mu   sync.Mutex
	n    int
	idle chan struct{} // Closed when n drops back to zero.
}

func (t *titleTracker) init() {
	t.once.Do(func() { t.stopCtx, t.stopFunc = context.WithCancel(context.Background()) })
}

// begin registers a title generation, returning a context detached from
// parent's cancellation but cancelled by stop, and the func that ends it.
func (t *titleTracker) begin(parent context.Context) (context.Context, func()) {
	t.init()
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	unhook := context.AfterFunc(t.stopCtx, cancel)

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.n == 0 {
		t.idle = make(chan struct{})
	}
	t.n++
	return ctx, func() {
		unhook()
		cancel()
		t.mu.Lock()
		defer t.mu.Unlock()
		t.n--
		if t.n == 0 {
			close(t.idle)
		}
	}
}

// shutdown cancels every title in flight and every one begun later, for an
// agent being discarded. A stopped title still saves the fallback name.
func (t *titleTracker) shutdown() {
	t.init()
	t.stopFunc()
}

// wait blocks until no title is in flight or timeout passes, and reports
// whether every title finished.
func (t *titleTracker) wait(timeout time.Duration) bool {
	t.mu.Lock()
	n, idle := t.n, t.idle
	t.mu.Unlock()
	if n == 0 {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-idle:
		return true
	case <-timer.C:
		return false
	}
}
