package subagents

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
)

// RunningEntry holds the live state of a single running sub-agent.
type RunningEntry struct {
	ChildSessionID  string
	ParentSessionID string
	Name            string
	Color           string
	Model           string
	Status          string
	StartedAt       time.Time
}

// Live and terminal statuses for sub-agent runs.
const (
	StatusRunning   = "running"
	StatusRetrying  = "retrying"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
	StatusFailed    = "failed"
)

// RuntimeEvent is published whenever the set of running sub-agents changes.
// Finished is non-nil when the event reflects a sub-agent that just finished,
// carrying its final entry (including terminal Status) so the UI can react.
type RuntimeEvent struct {
	ParentSessionID string
	Entries         []RunningEntry
	Finished        *RunningEntry
}

// Runtime tracks which sub-agents are currently running across all sessions.
// There is exactly one Runtime per workspace; it is safe for concurrent use.
type Runtime struct {
	mu      sync.RWMutex
	entries map[string]RunningEntry // keyed by childSessionID
	broker  *pubsub.Broker[RuntimeEvent]
}

// NewRuntime constructs an empty Runtime ready for use.
func NewRuntime() *Runtime {
	return &Runtime{
		entries: make(map[string]RunningEntry),
		broker:  pubsub.NewBroker[RuntimeEvent](),
	}
}

// Register records a new running sub-agent and publishes a RuntimeEvent.
// It is a no-op when r is nil.
func (r *Runtime) Register(parentSessionID, childSessionID, name, color, model string) RunningEntry {
	if r == nil {
		return RunningEntry{}
	}
	entry := RunningEntry{
		ChildSessionID:  childSessionID,
		ParentSessionID: parentSessionID,
		Name:            name,
		Color:           color,
		Model:           model,
		Status:          StatusRunning,
		StartedAt:       time.Now(),
	}
	r.mu.Lock()
	r.entries[childSessionID] = entry
	r.mu.Unlock()

	r.publish(parentSessionID)
	return entry
}

// Finish removes a running sub-agent entry with a terminal status and publishes
// a RuntimeEvent whose Finished field carries the removed entry. Use one of the
// Status* constants for finalStatus. It is a no-op when r is nil.
func (r *Runtime) Finish(childSessionID, finalStatus string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	entry, ok := r.entries[childSessionID]
	if ok {
		entry.Status = finalStatus
		delete(r.entries, childSessionID)
	}
	r.mu.Unlock()

	if !ok {
		return
	}

	r.broker.Publish(pubsub.UpdatedEvent, RuntimeEvent{
		ParentSessionID: entry.ParentSessionID,
		Entries:         r.entriesFor(entry.ParentSessionID),
		Finished:        &entry,
	})
}

// SetStatus updates the Status field of a running sub-agent and publishes a
// RuntimeEvent. It is a no-op when r is nil or the entry is not found.
func (r *Runtime) SetStatus(childSessionID, status string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	entry, ok := r.entries[childSessionID]
	if ok {
		entry.Status = status
		r.entries[childSessionID] = entry
	}
	r.mu.Unlock()

	if ok {
		r.publish(entry.ParentSessionID)
	}
}

// List returns a snapshot of all running entries belonging to parentSessionID.
// The returned slice is a copy; mutating it does not affect internal state.
// Returns nil when r is nil or no entries match.
func (r *Runtime) List(parentSessionID string) []RunningEntry {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []RunningEntry
	for _, e := range r.entries {
		if e.ParentSessionID == parentSessionID {
			out = append(out, e)
		}
	}
	return out
}

// Subscribe returns a channel that receives RuntimeEvents whenever the set of
// running sub-agents changes. The channel is closed when ctx is cancelled or
// Shutdown is called. Returns a closed channel when r is nil.
func (r *Runtime) Subscribe(ctx context.Context) <-chan pubsub.Event[RuntimeEvent] {
	if r == nil {
		ch := make(chan pubsub.Event[RuntimeEvent])
		close(ch)
		return ch
	}
	return r.broker.Subscribe(ctx)
}

// Shutdown releases broker resources. It is a no-op when r is nil.
func (r *Runtime) Shutdown() {
	if r == nil {
		return
	}
	r.broker.Shutdown()
}

// publish gathers all entries for parentSessionID and sends a RuntimeEvent.
// Called with no locks held.
func (r *Runtime) publish(parentSessionID string) {
	r.broker.Publish(pubsub.UpdatedEvent, RuntimeEvent{
		ParentSessionID: parentSessionID,
		Entries:         r.entriesFor(parentSessionID),
	})
}

// entriesFor returns a snapshot of all entries belonging to parentSessionID.
// Acquires the read lock; callers must hold no locks.
func (r *Runtime) entriesFor(parentSessionID string) []RunningEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var entries []RunningEntry
	for _, e := range r.entries {
		if e.ParentSessionID == parentSessionID {
			entries = append(entries, e)
		}
	}
	return entries
}
