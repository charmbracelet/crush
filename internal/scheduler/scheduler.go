// Package scheduler implements session-scoped scheduled tasks ("cron
// jobs") for the CronCreate, CronList, and CronDelete agent tools. The
// model and semantics deliberately mirror Claude Code's cron tools and
// the Codex scheduled-tasks proposal: strict 5-field local-time cron
// expressions, one-shot versus recurring tasks, 8-character IDs, at most
// 50 tasks per session, in-memory session tasks, and opt-in durable
// tasks persisted to disk.
package scheduler

import (
	"context"
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

	"github.com/charmbracelet/crush/internal/lock"
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

// ErrOneShotInPast is returned when a one-shot (non-recurring) task's
// schedule already matched at an earlier minute today. The intended
// fire time is then unrecoverable: the schedule's next match has jumped
// to tomorrow, next month, or next year, which is never what a one-shot
// means. The usual cause is an agent computing the cron fields against
// a stale clock (e.g. the session-start time in its prompt rather than
// the actual current time), so the error names the next match to make
// the discrepancy visible.
var ErrOneShotInPast = errors.New("one-shot schedule's fire time has already passed today")

// fileLockTimeout bounds the wait on the scheduled-tasks file lock. The
// lock is only ever held for the duration of one read-modify-write, so
// a healthy peer releases it in milliseconds; anything longer is a
// wedged process and is better reported as a persist failure than
// blocked on.
const fileLockTimeout = 5 * time.Second

// ownershipRetryInterval is how often a process that lost (or never
// held) the scheduler ownership lock retries it. The kernel drops the
// lock when the owning process exits, so this is how a remaining
// window takes over firing durable tasks instead of waiting for a
// restart.
const ownershipRetryInterval = 30 * time.Second

// Store keeps scheduled tasks for all sessions. Session tasks live only
// in memory; durable tasks are additionally persisted to a JSON file so
// they survive restarts.
//
// The file is shared by every Crush process working on the same
// project, so the store guards it two ways: an ownership lock (held for
// the process lifetime) elects the single process that fires durable
// tasks, and a short-held file lock serializes every read-modify-write
// so concurrent processes merge their changes instead of overwriting
// each other's.
type Store struct {
	mu       sync.RWMutex
	tasks    map[string]*Task // keyed by ID
	filePath string           // durable persistence path; "" disables durable tasks
	now      func() time.Time // for tests

	// owner reports whether this process holds the ownership lock for
	// the persistence path. Only the owner fires durable tasks; every
	// process still serves its own sessions' cron tools.
	owner        bool
	ownerRelease func()

	// removedSinceSync tracks durable task IDs deleted in this process
	// since the last successful on-disk sync, so a merge write removes
	// them even if another process rewrote the file in between.
	removedSinceSync map[string]bool
	// dirtySinceSync tracks durable task IDs this process has modified
	// since the last successful on-disk sync, so a merge write only
	// overwrites a task's disk entry when this process actually changed
	// it. A non-owner process holding a stale in-memory copy (loaded at
	// startup, advanced on disk by the owner since) must not let that
	// copy win over the owner's newer NextRunAt / RunCount / LastError.
	dirtySinceSync map[string]bool
	// lastDiskIDs holds the task IDs present on disk as of the last
	// successful sync, so a durable task another process deleted from
	// the file is not resurrected by this process's next write.
	lastDiskIDs map[string]bool
	// lastModTime is the persistence file's mtime as of the last read,
	// gating refreshes to one stat call per tick when nothing changed.
	lastModTime time.Time
	// lastOwnerAttempt throttles ownership lock retries.
	lastOwnerAttempt time.Time
}

// NewStore returns a Store persisting durable tasks at filePath. An
// empty filePath keeps every task in memory only.
func NewStore(filePath string) *Store {
	return &Store{
		tasks:            make(map[string]*Task),
		filePath:         filePath,
		now:              time.Now,
		removedSinceSync: make(map[string]bool),
		dirtySinceSync:   make(map[string]bool),
		lastDiskIDs:      make(map[string]bool),
	}
}

// opLockPath is the short-held lock file serializing file access.
func (s *Store) opLockPath() string { return s.filePath + ".lock" }

// ownerLockPath is the lifetime lock file electing the firing process.
func (s *Store) ownerLockPath() string { return s.filePath + ".owner.lock" }

// Owner reports whether this process holds the scheduler ownership
// lock. When several Crush processes share a project's data directory,
// exactly one of them is the owner; the others never fire durable
// tasks, so a task cannot run twice.
func (s *Store) Owner() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.owner
}

// Close releases the ownership lock. The kernel already releases it on
// process exit (including crash); Close exists for tests and explicit
// teardown. In-memory stores (no persistence path) are a no-op.
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ownerRelease != nil {
		s.ownerRelease()
		s.ownerRelease = nil
		s.owner = false
		s.lastOwnerAttempt = time.Time{}
	}
}

// Load reads durable tasks from the persistence file, taking the
// scheduler ownership lock along the way. Missing files are not an
// error. Tasks whose next run is far in the past are rescheduled from
// now rather than firing a backlog of missed runs — there is no
// catch-up for missed fires.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() error {
	if s.filePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return fmt.Errorf("failed to create scheduled tasks directory: %w", err)
	}

	s.acquireOwnershipLocked()

	var durable []Task
	err := s.withOpLock(func() error {
		var err error
		durable, err = readDurableFile(s.filePath)
		return err
	})
	if err != nil {
		return err
	}
	s.adoptDurableLocked(durable)
	if fi, err := os.Stat(s.filePath); err == nil {
		s.lastModTime = fi.ModTime()
		s.rememberDiskIDsLocked(durable)
	}
	return nil
}

// acquireOwnershipLocked tries to take the ownership lock, marking this
// process as the one that fires durable tasks. It is throttled by
// ownershipRetryInterval so a non-owning store can retry cheaply from
// DueTasks without hammering the lock on every tick. Callers must hold
// s.mu.
func (s *Store) acquireOwnershipLocked() {
	if s.filePath == "" || s.owner {
		return
	}
	now := s.now()
	if !s.lastOwnerAttempt.IsZero() && now.Sub(s.lastOwnerAttempt) < ownershipRetryInterval {
		return
	}
	s.lastOwnerAttempt = now
	release, err := lock.TryFile(s.ownerLockPath())
	if err != nil {
		if !errors.Is(err, lock.ErrContended) {
			// Another window owns firing, which is normal; anything else
			// (permissions, full disk) is worth surfacing.
			slog.Error("Failed to acquire scheduled-tasks ownership lock", "error", err)
		}
		return
	}
	s.owner = true
	s.ownerRelease = release
}

// withOpLock runs f while holding the short-lived file lock that
// serializes every scheduled-tasks read-modify-write across processes.
func (s *Store) withOpLock(f func() error) error {
	ctx, cancel := context.WithTimeout(context.Background(), fileLockTimeout)
	defer cancel()
	release, err := lock.File(ctx, s.opLockPath())
	if err != nil {
		return fmt.Errorf("failed to lock scheduled tasks file: %w", err)
	}
	defer release()
	return f()
}

// refreshDurableLocked adopts durable tasks other processes added to the
// file, forgets ones they deleted, and takes the disk version of durable
// tasks this process has not modified since its last sync, so a
// non-owner's CronList does not show tasks that already fired or were
// deleted elsewhere. It costs one stat call when the file has not
// changed. Callers must hold s.mu.
func (s *Store) refreshDurableLocked() {
	if s.filePath == "" {
		return
	}
	fi, err := os.Stat(s.filePath)
	if err != nil || !fi.ModTime().After(s.lastModTime) {
		return
	}
	modTime := fi.ModTime()
	var durable []Task
	err = s.withOpLock(func() error {
		var err error
		durable, err = readDurableFile(s.filePath)
		return err
	})
	if err != nil {
		slog.Error("Failed to refresh scheduled tasks", "error", err)
		return
	}
	for id, t := range s.tasks {
		if !t.Durable {
			continue
		}
		if _, onDisk := lookupTask(durable, id); !onDisk && !s.removedSinceSync[id] {
			// Another process deleted it from the file.
			delete(s.tasks, id)
		}
	}
	// Adopt the disk version of durable tasks this process has not
	// modified since its last sync: the owner may have fired, rescheduled,
	// or errored them since, and a stale local copy would otherwise win in
	// this process's next merge write and show up in CronList.
	for i := range durable {
		dt := durable[i]
		if !dt.Durable || dt.ID == "" {
			continue
		}
		cur, ok := s.tasks[dt.ID]
		if !ok || !cur.Durable || s.dirtySinceSync[dt.ID] {
			continue
		}
		if _, err := Parse(dt.Cron); err != nil {
			continue
		}
		// A zero next run is always "due", so a task carrying one would
		// fire on every tick forever. Keep the local copy over a corrupt
		// disk entry; the next persist drops it.
		if dt.NextRunAt.IsZero() {
			continue
		}
		s.tasks[dt.ID] = &dt
	}
	s.adoptDurableLocked(durable)
	s.lastModTime = modTime
	s.rememberDiskIDsLocked(durable)
}

// lookupTask finds a task by ID in a slice.
func lookupTask(tasks []Task, id string) (Task, bool) {
	for _, t := range tasks {
		if t.ID == id {
			return t, true
		}
	}
	return Task{}, false
}

// rememberDiskIDsLocked records which task IDs the file held as of the
// last read, so a later persist can tell "new task of mine" from
// "another process removed it". Callers must hold s.mu.
func (s *Store) rememberDiskIDsLocked(durable []Task) {
	clear(s.lastDiskIDs)
	for _, t := range durable {
		if t.ID != "" {
			s.lastDiskIDs[t.ID] = true
		}
	}
}

// adoptDurableLocked validates durable tasks read from disk and merges
// the ones this store does not already hold into memory. Tasks already
// present are left alone: this process's view of its own tasks wins.
// Callers must hold s.mu.
func (s *Store) adoptDurableLocked(durable []Task) {
	perSession := make(map[string]int)
	for _, t := range s.tasks {
		if t.Durable {
			perSession[t.SessionID]++
		}
	}
	for i := range durable {
		t := durable[i]
		if !t.Durable || t.ID == "" {
			continue
		}
		if _, exists := s.tasks[t.ID]; exists {
			continue
		}
		sched, err := Parse(t.Cron)
		if err != nil {
			slog.Warn("Dropping scheduled task with unparseable cron expression", "id", t.ID, "cron", t.Cron, "error", err)
			continue
		}
		if t.NextRunAt.Before(s.now().Add(-time.Minute)) {
			t.NextRunAt = sched.Next(s.now()).Truncate(time.Minute)
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
		s.tasks[t.ID] = &t
	}
}

// deleteTaskLocked removes a task and, when it was durable, records the
// removal so the next merge write also drops it from the file even if
// another process rewrote the file in between. Callers must hold s.mu.
func (s *Store) deleteTaskLocked(id string) {
	if t, ok := s.tasks[id]; ok && t.Durable {
		s.removedSinceSync[id] = true
	}
	delete(s.tasks, id)
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
	// Truncate to the start of the minute so a task created at HH:MM:50
	// for minute HH:MM fires at the top of the next minute, not 10
	// seconds later. The ticker polls every second, but cron expressions
	// have minute-level granularity — the stored next-run must reflect
	// that so users are not confused by sub-minute firing.
	nextRun = nextRun.Truncate(time.Minute)

	// A one-shot whose schedule already matched earlier today pinned a
	// fire time that has passed; accepting it would store a task firing
	// tomorrow or next year when the caller meant "in a few minutes".
	if !recurring && matchedEarlierToday(sched, now) {
		return Task{}, fmt.Errorf("%w: %q next matches %s", ErrOneShotInPast, cronExpr, nextRun.Format(time.RFC3339))
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
	// Session-only tasks never touch the file: a disk or lock problem
	// must not be able to block creating (or later deleting) a task that
	// needs no persistence at all.
	if durable {
		if err := s.persistLocked(); err != nil {
			delete(s.tasks, id)
			return Task{}, err
		}
	}
	return *task, nil
}

// matchedEarlierToday reports whether the schedule's most recent match
// is at or before the current minute today. A one-shot created after
// such a match can only fire on a later day than its fields suggest —
// the signature of cron fields computed against a stale clock. The
// current minute counts as "already passed": robfig treats it as
// consumed, so a schedule matching it next fires tomorrow or beyond.
// Next returns the first match strictly after its argument, so the
// cursor starts a second before midnight to include a "0 0 ..."
// schedule firing exactly at midnight.
func matchedEarlierToday(sched *Schedule, now time.Time) bool {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today := now.Truncate(time.Minute)
	next := sched.Next(midnight.Add(-time.Second))
	return !next.IsZero() && !next.After(today)
}

// List returns the tasks belonging to sessionID, ordered by next run.
// Durable tasks are refreshed from disk first (mtime-gated, one stat call
// when nothing changed) so a non-owner process does not list tasks that
// fired or were deleted elsewhere since its last sync.
func (s *Store) List(sessionID string) []Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.refreshDurableLocked()

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
	s.mu.Lock()
	defer s.mu.Unlock()

	s.refreshDurableLocked()

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
	s.deleteTaskLocked(id)
	if t.Durable {
		if err := s.persistLocked(); err != nil {
			// Put it back so memory and disk stay in agreement; otherwise the
			// task is gone from this process but returns on the next restart.
			s.tasks[id] = t
			return Task{}, err
		}
	}
	return deleted, nil
}

// Remove deletes a task by ID regardless of which session owns it or
// whether it is durable. It is used to retire tasks whose owning session
// no longer exists; Delete is the session-scoped, user-facing path.
func (s *Store) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return
	}
	s.deleteTaskLocked(id)
	if t.Durable {
		if err := s.persistLocked(); err != nil {
			slog.Error("Failed to persist scheduled tasks after removal", "id", id, "error", err)
		}
	}
}

// DueTasks returns every task whose next run time has passed as of now.
//
// Durable tasks are shared by every process on the project, so only the
// ownership lock holder gets them back; a second Crush window running
// the same file would otherwise fire them a second time. The owner also
// refreshes from disk first, adopting tasks other windows created.
func (s *Store) DueTasks() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.filePath != "" {
		s.acquireOwnershipLocked()
		if s.owner {
			s.refreshDurableLocked()
		}
	}

	now := s.now()
	var out []Task
	for _, t := range s.tasks {
		if t.Durable && !s.owner {
			continue
		}
		if !t.NextRunAt.After(now) {
			out = append(out, *t)
		}
	}
	sortTasks(out)
	return out
}

// MarkFired records a successful firing: recurring tasks reschedule
// themselves, one-shots delete themselves. Durable mutations are
// tracked as dirty so this process's next merge write overwrites the
// disk entry (a fire is exactly the kind of change a stale copy in
// another process must not clobber).
func (s *Store) MarkFired(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return
	}
	durable := t.Durable
	if durable {
		s.dirtySinceSync[id] = true
	}
	now := s.now()
	t.LastRunAt = &now
	t.RunCount++
	t.LastError = ""
	if !t.Recurring {
		s.deleteTaskLocked(id)
		if durable {
			s.persistBestEffort(id)
		}
		return
	}
	s.rescheduleLocked(t, now)
	if durable {
		s.persistBestEffort(id)
	}
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
		s.deleteTaskLocked(t.ID)
		return
	}
	next := sched.Next(now)
	if next.IsZero() {
		slog.Warn("Deleting scheduled task that can never fire again", "id", t.ID, "cron", t.Cron)
		s.deleteTaskLocked(t.ID)
		return
	}
	t.NextRunAt = next.Truncate(time.Minute)
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
	durable := t.Durable
	if durable {
		s.dirtySinceSync[id] = true
	}
	now := s.now()
	t.LastError = fireErr.Error()
	if t.Recurring {
		s.rescheduleLocked(t, now)
	} else {
		s.deleteTaskLocked(id)
	}
	if durable {
		s.persistBestEffort(id)
	}
}

// SetLastError records a failure that happened after the fire itself was
// accepted: the scheduler already called MarkFired (rescheduling the
// task), so unlike MarkError this must not touch the schedule or delete
// a one-shot that has already run — it only stamps LastError so
// CronList surfaces what happened to the run.
func (s *Store) SetLastError(id string, runErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tasks[id]
	if !ok {
		return
	}
	t.LastError = runErr.Error()
	if t.Durable {
		s.dirtySinceSync[id] = true
		s.persistBestEffort(id)
	}
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

	droppedDurable := false
	for id, t := range s.tasks {
		if t.SessionID == sessionID {
			s.deleteTaskLocked(id)
			if t.Durable {
				droppedDurable = true
			}
		}
	}
	if droppedDurable {
		s.persistBestEffort("")
	}
}

// persistLocked writes durable tasks to disk. Callers must hold s.mu.
//
// The write is a read-modify-write under the short-held file lock:
// durable tasks belonging to other Crush processes on the same project
// are preserved rather than clobbered, tasks this process deleted are
// dropped, and this process's version of a task wins only when it has
// actually modified that task since its last successful sync — a stale
// copy must not overwrite the owner's newer NextRunAt / RunCount /
// LastError. Without the merge, two windows on one project would each
// write their own view and the last rename would silently discard the
// other's tasks.
func (s *Store) persistLocked() error {
	if s.filePath == "" {
		return nil
	}

	if err := s.withOpLock(func() error {
		disk, err := readDurableFile(s.filePath)
		if err != nil {
			return err
		}
		merged := make([]Task, 0, len(disk)+len(s.tasks))
		onDisk := make(map[string]bool, len(disk))
		for _, t := range disk {
			onDisk[t.ID] = true
			if s.removedSinceSync[t.ID] {
				continue
			}
			if cur, ok := s.tasks[t.ID]; ok && cur.Durable && s.dirtySinceSync[t.ID] {
				merged = append(merged, *cur)
				continue
			}
			merged = append(merged, t)
		}
		for _, t := range s.tasks {
			// A task absent from the file that the file never held is
			// new here; one the file used to hold was removed by
			// another process, so honor the removal instead of
			// resurrecting it.
			if t.Durable && !onDisk[t.ID] && !s.lastDiskIDs[t.ID] {
				merged = append(merged, *t)
			}
		}
		sortTasks(merged)
		if err := writeDurableFile(s.filePath, merged); err != nil {
			return err
		}
		if fi, err := os.Stat(s.filePath); err == nil {
			s.lastModTime = fi.ModTime()
		}
		s.rememberDiskIDsLocked(merged)
		clear(s.removedSinceSync)
		clear(s.dirtySinceSync)
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// readDurableFile reads and parses the persistence file. A missing file
// is an empty task list, not an error.
func readDurableFile(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read scheduled tasks: %w", err)
	}
	var durable []Task
	if err := json.Unmarshal(data, &durable); err != nil {
		return nil, fmt.Errorf("failed to parse scheduled tasks: %w", err)
	}
	return durable, nil
}

// writeDurableFile atomically replaces the persistence file.
func writeDurableFile(path string, tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode scheduled tasks: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create scheduled tasks directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".scheduled_tasks-*.json")
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
	if err := os.Rename(tmpName, path); err != nil {
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
