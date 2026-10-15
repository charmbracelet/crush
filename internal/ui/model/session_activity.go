package model

import "github.com/charmbracelet/crush/internal/workspace"

// sessionActivity is what a session is doing, as shown in the session panel.
type sessionActivity uint8

const (
	activityIdle sessionActivity = iota
	activityWorking
	activityWaiting // Blocked on a permission or question prompt.
	activityDoneUnseen
	activityFailedUnseen
)

// sessionActivityTracker holds per-session activity. Idle is the absence of
// an entry, so the zero value is ready to use. Not safe for concurrent use:
// call it from Update only.
type sessionActivityTracker struct {
	states map[string]sessionActivity
	// version counts mark changes, so callers can tell that a probe or a
	// rendering predates the current marks.
	version uint64
}

// Get returns the activity of a session, idle if unknown.
func (t *sessionActivityTracker) Get(id string) sessionActivity {
	return t.states[id]
}

func (t *sessionActivityTracker) set(id string, a sessionActivity) {
	if t.states[id] == a {
		return
	}
	t.version++
	if a == activityIdle {
		delete(t.states, id)
		return
	}
	if t.states == nil {
		t.states = make(map[string]sessionActivity)
	}
	t.states[id] = a
}

// Working marks the session as running its agent.
func (t *sessionActivityTracker) Working(id string) { t.set(id, activityWorking) }

// Waiting marks the session as blocked on a prompt.
func (t *sessionActivityTracker) Waiting(id string) { t.set(id, activityWaiting) }

// Finished records the end of a run. It stays unseen unless the user is
// already looking at that session (viewing).
func (t *sessionActivityTracker) Finished(id string, failed bool, viewing string) {
	switch {
	case id == viewing:
		t.set(id, activityIdle)
	case failed:
		t.set(id, activityFailedUnseen)
	default:
		t.set(id, activityDoneUnseen)
	}
}

// Viewed clears an unseen result. A running or waiting session is untouched.
func (t *sessionActivityTracker) Viewed(id string) {
	if a := t.states[id]; a == activityDoneUnseen || a == activityFailedUnseen {
		t.set(id, activityIdle)
	}
}

// Active returns the IDs of the sessions that are not idle: running, waiting
// on a prompt, or holding a result the user has not seen. The Sessions panel
// pins them so their status shows even outside its first rows.
func (t *sessionActivityTracker) Active() []string {
	ids := make([]string, 0, len(t.states))
	for id := range t.states {
		ids = append(ids, id)
	}
	return ids
}

// Forget drops all state for a session.
func (t *sessionActivityTracker) Forget(id string) {
	if _, ok := t.states[id]; ok {
		delete(t.states, id)
		t.version++
	}
}

// Retain forgets every session not in ids, such as deleted ones.
func (t *sessionActivityTracker) Retain(ids map[string]struct{}) {
	for id := range t.states {
		if _, ok := ids[id]; !ok {
			t.Forget(id)
		}
	}
}

// Reconcile corrects the tracker from live probes. Only probed IDs change,
// and a failed probe (not Known) changes nothing. A pending prompt always
// means waiting; busy means working; not busy clears working or waiting but
// keeps an unseen result.
func (t *sessionActivityTracker) Reconcile(probes map[string]workspace.SessionStatus) {
	for id, p := range probes {
		if !p.Known {
			continue
		}
		switch cur := t.states[id]; {
		case p.PendingPrompt:
			t.set(id, activityWaiting)
		case p.Busy:
			t.set(id, activityWorking)
		case cur == activityWorking || cur == activityWaiting:
			t.set(id, activityIdle)
		}
	}
}
