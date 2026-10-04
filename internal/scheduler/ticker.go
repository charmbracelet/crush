package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// tickInterval matches Claude Code: the scheduler checks for due tasks
// once per second.
const tickInterval = time.Second

// FireFunc runs a due task's prompt. It receives the task as of the
// moment it was observed due.
type FireFunc func(ctx context.Context, task Task) error

// TransientError reports a fire failure caused by a temporary condition
// the next attempt may not hit, such as a database hiccup or a context
// canceled during shutdown. The scheduler retries the task after
// RetryIn rather than recording a failure, so a one-shot task is not
// lost to a blip and a recurring one keeps its schedule.
type TransientError struct {
	Err     error
	RetryIn time.Duration
}

func (e *TransientError) Error() string {
	if e.RetryIn > 0 {
		return fmt.Sprintf("%v (retrying in %s)", e.Err, e.RetryIn)
	}
	return e.Err.Error()
}

func (e *TransientError) Unwrap() error { return e.Err }

// Retry defers a task's next run by delay without recording a failure,
// keeping it alive across a transient fire error (a database hiccup, a
// canceled context) instead of letting MarkError retire a one-shot or
// stamp a recurring task with a LastError it did not earn.
func (s *Store) Retry(id string, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return
	}
	next := s.now().Add(delay)
	if t.NextRunAt.After(next) {
		return
	}
	t.NextRunAt = next
	s.persistBestEffort(id)
}

// Scheduler polls a Store and fires due tasks. Fired prompts run
// between turns via the provided FireFunc; there is no catch-up for
// missed fires.
type Scheduler struct {
	store *Store
	fire  FireFunc
}

// NewScheduler returns a Scheduler that fires the store's due tasks.
func NewScheduler(store *Store, fire FireFunc) *Scheduler {
	return &Scheduler{store: store, fire: fire}
}

// Run starts the tick loop and blocks until ctx is canceled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// Tick fires any currently-due tasks once. Exported for tests.
func (s *Scheduler) Tick(ctx context.Context) {
	s.tick(ctx)
}

func (s *Scheduler) tick(ctx context.Context) {
	for _, task := range s.store.DueTasks() {
		slog.Debug("Firing scheduled task", "id", task.ID, "session_id", task.SessionID)
		if err := s.fire(ctx, task); err != nil {
			var transient *TransientError
			if errors.As(err, &transient) {
				slog.Warn("Scheduled task fire deferred", "id", task.ID, "error", err)
				s.store.Retry(task.ID, transient.RetryIn)
				continue
			}
			slog.Error("Scheduled task failed", "id", task.ID, "error", err)
			s.store.MarkError(task.ID, err)
			continue
		}
		s.store.MarkFired(task.ID)
	}
}
