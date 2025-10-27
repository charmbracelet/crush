package terminal

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coder/acp-go-sdk"
)

// Service owns 0…N running terminals for one session.
type Service struct {
	conn        *acp.AgentSideConnection
	termAllowed bool
	mu          sync.RWMutex
	term        map[acp.SessionId]map[ID]*Terminal
}

func NewService(conn *acp.AgentSideConnection, termAllowed bool) *Service {
	return &Service{
		conn:        conn,
		termAllowed: termAllowed,
		term:        make(map[acp.SessionId]map[ID]*Terminal),
	}
}

// Create launches a new terminal and keeps it in the registry.
func (s *Service) Create(ctx context.Context, sessionID acp.SessionId, cmd string, opts ...CreateOption) (*Terminal, error) {
	if !s.termAllowed {
		return nil, errors.New("client does not support terminal capability")
	}

	// apply defaults
	co := &createOpts{}
	for _, fn := range opts {
		fn(co)
	}

	// build env slice
	env := make([]acp.EnvVariable, 0, len(co.env))
	for k, v := range co.env {
		env = append(env, acp.EnvVariable{Name: k, Value: v})
	}

	t := New(cmd, co.args, env, co.cwd, co.byteLim)
	if err := t.Start(ctx, s.conn, sessionID); err != nil {
		return nil, err
	}

	// register
	s.mu.Lock()
	if s.term[sessionID] == nil {
		s.term[sessionID] = make(map[ID]*Terminal)
	}
	s.term[sessionID][t.ID] = t
	s.mu.Unlock()
	return t, nil
}

// Get returns an existing terminal or error.
func (s *Service) Get(sessionID acp.SessionId, id ID) (*Terminal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set, ok := s.term[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q has no terminals", sessionID)
	}
	t, ok := set[id]
	if !ok {
		return nil, fmt.Errorf("terminal %q not found", id)
	}
	return t, nil
}

// Release removes **one** terminal and calls its Release method.
func (s *Service) Release(ctx context.Context, sessionID acp.SessionId, id ID) error {
	s.mu.Lock()
	t, ok := s.term[sessionID][id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("terminal %q not found", id)
	}
	delete(s.term[sessionID], id)
	if len(s.term[sessionID]) == 0 {
		delete(s.term, sessionID)
	}
	s.mu.Unlock()

	return t.Release(ctx, s.conn)
}

// ReleaseAll kills every terminal that belongs to a session (handy on session close).
func (s *Service) ReleaseAll(ctx context.Context, sessionID acp.SessionId) {
	s.mu.Lock()
	set := s.term[sessionID]
	delete(s.term, sessionID)
	s.mu.Unlock()

	for _, t := range set {
		_ = t.Release(ctx, s.conn)
	}
}
