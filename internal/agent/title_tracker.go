package agent

import (
	"sync"
	"time"
)

// titleTimeout bounds one title generation so a stalled request ends.
const titleTimeout = 60 * time.Second

// titleTracker counts in-flight title generations so shutdown can wait for
// them before the database closes. The zero value is ready to use.
type titleTracker struct {
	mu   sync.Mutex
	n    int
	idle chan struct{} // Closed when n drops back to zero.
}

// begin registers a title generation; pair each call with end.
func (t *titleTracker) begin() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.n == 0 {
		t.idle = make(chan struct{})
	}
	t.n++
}

func (t *titleTracker) end() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.n--
	if t.n == 0 {
		close(t.idle)
	}
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
