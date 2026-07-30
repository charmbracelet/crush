package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// tickInterval matches Claude Code: the scheduler checks for due tasks
// once per second.
const tickInterval = time.Second

// FireFunc runs a due task's prompt. It receives the task as of the
// moment it was observed due.
type FireFunc func(ctx context.Context, task Task) error

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
			slog.Error("Scheduled task failed", "id", task.ID, "error", err)
			s.store.MarkError(task.ID, err)
			continue
		}
		s.store.MarkFired(task.ID)
	}
}
