// Package scheduler implements session-scoped scheduled tasks ("cron
// jobs") for the CronCreate, CronList, and CronDelete agent tools. The
// model and semantics deliberately mirror Claude Code's cron tools and
// the Codex scheduled-tasks proposal: strict 5-field local-time cron
// expressions, one-shot versus recurring tasks, 8-character IDs, at most
// 50 tasks per session, in-memory session tasks, and opt-in durable
// tasks persisted to disk.
package scheduler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// MaxTasksPerSession caps the number of scheduled tasks a single session
// may hold, matching Claude Code's limit.
const MaxTasksPerSession = 50

// Task is a single scheduled prompt.
type Task struct {
	ID        string     `json:"id"`
	SessionID string     `json:"sessionId"`
	Prompt    string     `json:"prompt"`
	Cron      string     `json:"cron"`
	Recurring bool       `json:"recurring"`
	Durable   bool       `json:"durable"`
	CreatedAt time.Time  `json:"createdAt"`
	NextRunAt time.Time  `json:"nextRunAt"`
	LastRunAt *time.Time `json:"lastRunAt,omitempty"`
	LastError string     `json:"lastError,omitempty"`
	RunCount  int        `json:"runCount"`
}

// NewTaskID returns a random 8-character hex ID, matching the ID shape
// used by both Claude Code and the Codex scheduled-tasks prototype.
func NewTaskID() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("failed to generate task ID: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// ErrTaskNotFound is returned when a delete targets an unknown task ID.
var ErrTaskNotFound = errors.New("no scheduled task with that ID")

// ErrTooManyTasks is returned when a session already holds the maximum
// number of scheduled tasks.
var ErrTooManyTasks = errors.New("session already has the maximum of 50 scheduled tasks")

// ErrNeverFires is returned when a cron expression parses but can never
// match, such as "0 0 30 2 *" (February 30th).
var ErrNeverFires = errors.New("cron expression is valid but will never fire")

// Store keeps scheduled tasks for all sessions. Session tasks live only
// in memory; durable tasks are additionally persisted to a JSON file so
// they survive restarts.
type Store struct {
	mu       sync.RWMutex
	tasks    map[string]*Task // keyed by ID
	filePath string           // durable persistence path; "" disables durable tasks
	now      func() time.Time // for tests
}

// NewStore returns a Store persisting durable tasks at filePath. An
// empty filePath keeps every task in memory only.
func NewStore(filePath string) *Store {
	return &Store{
		tasks:    make(map[string]*Task),
		filePath: filePath,
		now:      time.Now,
	}
}

// Load reads durable tasks from the store's persistence file. Missing
// files are not an error. Tasks whose next run is far in the past are
// rescheduled from now rather than firing a backlog of missed runs —
// there is no catch-up for missed fires.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read scheduled tasks: %w", err)
	}

	var durable []Task
	if err := json.Unmarshal(data, &durable); err != nil {
		return fmt.Errorf("failed to parse scheduled tasks: %w", err)
	}

	perSession := make(map[string]int)
	for i := range durable {
		t := durable[i]
		if !t.Durable || t.ID == "" {
			continue
		}
		sched, err := Parse(t.Cron)
		if err != nil {
			slog.Warn("Dropping scheduled task with unparseable cron expression", "id", t.ID, "cron", t.Cron, "error", err)
			continue
		}
		if t.NextRunAt.Before(s.now().Add(-time.Minute)) {
			t.NextRunAt = sched.Next(s.now())
		}
		// A zero next run is always "due", so a task carrying one would
		// fire on every tick forever. Drop it instead.
		if t.NextRunAt.IsZero() {
			slog.Warn("Dropping scheduled task that can never fire again", "id", t.ID, "cron", t.Cron)
			continue
		}
		if perSession[t.SessionID] >= MaxTasksPerSession {
			slog.Warn("Dropping scheduled task over the per-session limit", "id", t.ID, "session_id", t.SessionID)
			continue
		}
		perSession[t.SessionID]++
		task := t
		s.tasks[task.ID] = &task
	}
	return nil
}

// Create validates and registers a new task for sessionID.
func (s *Store) Create(sessionID, cronExpr, prompt string, recurring, durable bool) (Task, error) {
	if prompt == "" {
		return Task{}, errors.New("prompt is required")
	}
	sched, err := Parse(cronExpr)
	if err != nil {
		return Task{}, err
	}
	if durable && s.filePath == "" {
		return Task{}, errors.New("durable tasks are not available: no persistence path configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, t := range s.tasks {
		if t.SessionID == sessionID {
			count++
		}
	}
	if count >= MaxTasksPerSession {
		return Task{}, ErrTooManyTasks
	}

	now := s.now()
	nextRun := sched.Next(now)
	// Reject schedules that can never match ("0 0 30 2 *"). Storing a
	// zero next run would make the task perpetually due.
	if nextRun.IsZero() {
		return Task{}, fmt.Errorf("%w: %s", ErrNeverFires, cronExpr)
	}

	id, err := NewTaskID()
	if err != nil {
		return Task{}, err
	}
	task := &Task{
		ID:        id,
		SessionID: sessionID,
		Prompt:    prompt,
		Cron:      cronExpr,
		Recurring: recurring,
		Durable:   durable,
		CreatedAt: now.UTC(),
		NextRunAt: nextRun,
	}
	s.tasks[id] = task
	if err := s.persistLocked(); err != nil {
		delete(s.tasks, id)
		return Task{}, err
	}
	return *task, nil
}

// List returns the tasks belonging to sessionID, ordered by next run.
func (s *Store) List(sessionID string) []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []Task
	for _, t := range s.tasks {
		if t.SessionID == sessionID {
			out = append(out, *t)
		}
	}
	sortTasks(out)
	return out
}

// ListAll returns every task in the store, ordered by next run.
func (s *Store) ListAll() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, *t)
	}
	sortTasks(out)
	return out
}

// Delete removes the task with the given ID. The task must belong to
// sessionID: a session cannot delete another session's tasks.
func (s *Store) Delete(sessionID, id string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok || t.SessionID != sessionID {
		return Task{}, ErrTaskNotFound
	}
	deleted := *t
	delete(s.tasks, id)
	if err := s.persistLocked(); err != nil {
		// Put it back so memory and disk stay in agreement; otherwise the
		// task is gone from this process but returns on the next restart.
		s.tasks[id] = t
		return Task{}, err
	}
	return deleted, nil
}

// Remove deletes a task by ID regardless of which session owns it or
// whether it is durable. It is used to retire tasks whose owning session
// no longer exists; Delete is the session-scoped, user-facing path.
func (s *Store) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return
	}
	delete(s.tasks, id)
	if err := s.persistLocked(); err != nil {
		slog.Error("Failed to persist scheduled tasks after removal", "id", id, "error", err)
	}
}

// DueTasks returns every task whose next run time has passed as of now.
func (s *Store) DueTasks() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := s.now()
	var out []Task
	for _, t := range s.tasks {
		if !t.NextRunAt.After(now) {
			out = append(out, *t)
		}
	}
	sortTasks(out)
	return out
}

// MarkFired records a successful firing: recurring tasks reschedule
// themselves, one-shots delete themselves.
func (s *Store) MarkFired(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return
	}
	now := s.now()
	t.LastRunAt = &now
	t.RunCount++
	t.LastError = ""
	if !t.Recurring {
		delete(s.tasks, id)
		s.persistBestEffort(id)
		return
	}
	s.rescheduleLocked(t, now)
	s.persistBestEffort(id)
}

// rescheduleLocked advances a recurring task to its next fire time,
// deleting it if it can never fire again. Callers must hold s.mu.
//
// Deleting is the only safe response to an unschedulable task: a zero
// NextRunAt is always in the past, so DueTasks would hand the task back
// on every tick and the scheduler would spin firing it.
func (s *Store) rescheduleLocked(t *Task, now time.Time) {
	sched, err := Parse(t.Cron)
	if err != nil {
		slog.Warn("Deleting scheduled task with unparseable cron expression", "id", t.ID, "cron", t.Cron, "error", err)
		delete(s.tasks, t.ID)
		return
	}
	next := sched.Next(now)
	if next.IsZero() {
		slog.Warn("Deleting scheduled task that can never fire again", "id", t.ID, "cron", t.Cron)
		delete(s.tasks, t.ID)
		return
	}
	t.NextRunAt = next
}

// persistBestEffort writes durable tasks, logging rather than returning
// a failure. Callers must hold s.mu.
func (s *Store) persistBestEffort(id string) {
	if err := s.persistLocked(); err != nil {
		slog.Error("Failed to persist scheduled tasks", "id", id, "error", err)
	}
}

// MarkError records a firing failure and reschedules recurring tasks so
// a transient error does not kill the job.
func (s *Store) MarkError(id string, fireErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return
	}
	now := s.now()
	t.LastError = fireErr.Error()
	if t.Recurring {
		s.rescheduleLocked(t, now)
	} else {
		delete(s.tasks, id)
	}
	s.persistBestEffort(id)
}

// DropSession removes every task belonging to a session, durable ones
// included.
//
// It is called when a session turns out to no longer exist, which makes
// its durable tasks unrunnable garbage: keeping them would leave the
// scheduler retrying a session that can never come back, logging an
// error and rescheduling on every fire, forever.
func (s *Store) DropSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dropped := false
	for id, t := range s.tasks {
		if t.SessionID == sessionID {
			delete(s.tasks, id)
			dropped = true
		}
	}
	if dropped {
		s.persistBestEffort("")
	}
}

// persistLocked writes durable tasks to disk. Callers must hold s.mu.
func (s *Store) persistLocked() error {
	if s.filePath == "" {
		return nil
	}

	var durable []Task
	for _, t := range s.tasks {
		if t.Durable {
			durable = append(durable, *t)
		}
	}
	sortTasks(durable)

	data, err := json.MarshalIndent(durable, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode scheduled tasks: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("failed to create scheduled tasks directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.filePath), ".scheduled_tasks-*.json")
	if err != nil {
		return fmt.Errorf("failed to create scheduled tasks temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write scheduled tasks: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to write scheduled tasks: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to secure scheduled tasks file: %w", err)
	}
	if err := os.Rename(tmpName, s.filePath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to replace scheduled tasks file: %w", err)
	}
	return nil
}

// sortTasks orders tasks by next run, breaking ties on ID. The tiebreak
// matters: tasks are held in a map, and several tasks routinely share a
// next-run time (anything created in the same minute), so without it the
// order CronList prints varies between calls.
func sortTasks(tasks []Task) {
	slices.SortFunc(tasks, func(a, b Task) int {
		if c := a.NextRunAt.Compare(b.NextRunAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
}
