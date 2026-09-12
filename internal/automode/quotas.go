package automode

import "sync"

// quotas tracks per-session denial counters for the native auto mode.
// State is in-memory: the process is long-lived, and quota state is
// session-scoped by design.
type quotas struct {
	mu    sync.Mutex
	state map[string]*quotaState
}

type quotaState struct {
	consecutiveDenials int
	totalDenials       int
}

func newQuotas() *quotas {
	return &quotas{state: make(map[string]*quotaState)}
}

func (q *quotas) get(sessionID string) quotaState {
	if sessionID == "" {
		return quotaState{}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if s, ok := q.state[sessionID]; ok {
		return *s
	}
	return quotaState{}
}

// recordAllow resets the consecutive counter for a session.
func (q *quotas) recordAllow(sessionID string) {
	if sessionID == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if s, ok := q.state[sessionID]; ok {
		s.consecutiveDenials = 0
	}
}

// recordDenial increments both counters and returns the updated state.
func (q *quotas) recordDenial(sessionID string) quotaState {
	if sessionID == "" {
		return quotaState{}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	s, ok := q.state[sessionID]
	if !ok {
		s = &quotaState{}
		q.state[sessionID] = s
	}
	s.consecutiveDenials++
	s.totalDenials++
	return *s
}

// paused reports whether a session has hit its quota limits, in which
// case classification escalates to the host prompt instead of denying.
func (q *quotas) paused(sessionID string, maxConsecutive, maxTotal int) bool {
	s := q.get(sessionID)
	return (maxConsecutive > 0 && s.consecutiveDenials >= maxConsecutive) ||
		(maxTotal > 0 && s.totalDenials >= maxTotal)
}
