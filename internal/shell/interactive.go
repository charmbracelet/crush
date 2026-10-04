package shell

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/crush/internal/csync"
)

// Interactive session limits.
const (
	// MaxInteractiveSessions bounds how many completed sessions are kept
	// readable (for scrollback) before the oldest are dropped.
	MaxInteractiveSessions = 50
	// completedSessionRetentionMinutes is how long a finished session stays
	// queryable before it is cleaned up.
	completedSessionRetentionMinutes = 8 * 60
)

// ErrInteractiveActive is returned by [InteractiveSessionManager.Start] when
// another interactive session is still running. Only one exists at a time
// so that a single terminal dialog always corresponds to it.
var ErrInteractiveActive = fmt.Errorf("an interactive session is already running")

// InteractiveSessionManager tracks interactive sessions so tools can start,
// inspect, and drive them by ID, mirroring [BackgroundShellManager] for
// background jobs.
type InteractiveSessionManager struct {
	sessions *csync.Map[string, *InteractiveSession]
	counter  atomic.Uint64
	mu       sync.Mutex // serializes Start
}

var (
	interactiveManager     *InteractiveSessionManager
	interactiveManagerOnce sync.Once
)

// GetInteractiveSessionManager returns the singleton interactive session
// manager.
func GetInteractiveSessionManager() *InteractiveSessionManager {
	interactiveManagerOnce.Do(func() {
		interactiveManager = &InteractiveSessionManager{
			sessions: csync.NewMap[string, *InteractiveSession](),
		}
	})
	return interactiveManager
}

// Start spawns an interactive session and registers it under a fresh ID.
// It fails with [ErrInteractiveActive] while another session is running.
func (m *InteractiveSessionManager) Start(opts InteractiveSessionOptions) (*InteractiveSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupLocked()

	if active, ok := m.activeLocked(); ok {
		return nil, fmt.Errorf("%w: %s", ErrInteractiveActive, active.ID())
	}

	session, err := NewInteractiveSession(opts)
	if err != nil {
		return nil, err
	}

	session.id = fmt.Sprintf("%03X", m.counter.Add(1))
	m.sessions.Set(session.id, session)
	m.cleanupLocked()

	return session, nil
}

// Register adds an already-created session to the manager under a fresh ID.
// The UI uses it for user-initiated sessions so tools can drive them too.
func (m *InteractiveSessionManager) Register(session *InteractiveSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session == nil {
		return fmt.Errorf("interactive session is nil")
	}
	if active, ok := m.activeLocked(); ok {
		return fmt.Errorf("%w: %s", ErrInteractiveActive, active.ID())
	}

	session.id = fmt.Sprintf("%03X", m.counter.Add(1))
	m.sessions.Set(session.id, session)
	m.cleanupLocked()

	return nil
}

// Get returns a session by ID.
func (m *InteractiveSessionManager) Get(id string) (*InteractiveSession, bool) {
	return m.sessions.Get(id)
}

// MostRecent returns the most recently registered session, running or not.
func (m *InteractiveSessionManager) MostRecent() (*InteractiveSession, bool) {
	var newest *InteractiveSession
	for session := range m.sessions.Seq() {
		if newest == nil || session.ID() > newest.ID() {
			newest = session
		}
	}
	if newest == nil {
		return nil, false
	}
	return newest, true
}

// Active returns the session that is still running, if any.
func (m *InteractiveSessionManager) Active() (*InteractiveSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeLocked()
}

func (m *InteractiveSessionManager) activeLocked() (*InteractiveSession, bool) {
	for session := range m.sessions.Seq() {
		if !session.Exited() {
			return session, true
		}
	}
	return nil, false
}

// Remove drops a session from the manager without terminating it.
func (m *InteractiveSessionManager) Remove(id string) {
	m.sessions.Del(id)
}

// List returns the known session IDs.
func (m *InteractiveSessionManager) List() []string {
	ids := make([]string, 0, m.sessions.Len())
	for id := range m.sessions.Seq2() {
		ids = append(ids, id)
	}
	return ids
}

// KillAll terminates every session and closes its PTY, used on shutdown so
// no orphan processes outlive the app.
func (m *InteractiveSessionManager) KillAll() {
	sessions := slices.Collect(m.sessions.Seq())
	m.sessions.Reset(map[string]*InteractiveSession{})

	for _, session := range sessions {
		_ = session.Kill()
		_ = session.Close()
	}
}

// Cleanup drops completed sessions; it returns how many were dropped.
func (m *InteractiveSessionManager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cleanupLocked()
}

// cleanupLocked enforces the retention rules. Callers must hold m.mu.
func (m *InteractiveSessionManager) cleanupLocked() int {
	var removed int

	// Completed sessions beyond the cap are dropped oldest-first; the
	// counter is monotonic, so lexicographic ID order is age order.
	for m.sessions.Len() > MaxInteractiveSessions {
		var oldestID string
		var oldest *InteractiveSession
		for session := range m.sessions.Seq() {
			if !session.Exited() {
				continue
			}
			if oldest == nil || session.ID() < oldestID {
				oldest, oldestID = session, session.ID()
			}
		}
		if oldest == nil {
			break
		}
		m.sessions.Del(oldestID)
		removed++
	}

	// Completed sessions also age out.
	for session := range m.sessions.Seq() {
		if session.Exited() && session.completedAgeMinutes() > completedSessionRetentionMinutes {
			m.sessions.Del(session.ID())
			removed++
		}
	}

	return removed
}
