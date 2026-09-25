// Package terminal provides the bridge between an interactive terminal
// session (spawned by a tool) and the UI that displays it. It mirrors the
// permission and question services: a request is published over pubsub,
// and the attached TUI shows the session in its terminal dialog.
//
// Sessions live in [shell.GetInteractiveSessionManager]; this package only
// carries the "please display this" signal, since the session itself is an
// in-process value that cannot cross the client/server boundary.
package terminal

import (
	"context"
	"errors"
	"sync"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/google/uuid"
)

// ErrUnavailable is returned when no interactive UI is attached, so nobody
// could ever look at the terminal.
var ErrUnavailable = errors.New("no interactive terminal is attached")

// Request asks the attached TUI to show a running interactive session.
type Request struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	Command     string `json:"command"`
	WorkingDir  string `json:"working_dir"`
	Description string `json:"description,omitempty"`
	// AgentDriven marks sessions the agent owns and drives itself: the
	// panel is a read-only view and user input is refused. When false the
	// session belongs to the user and the terminal accepts their input.
	AgentDriven bool                      `json:"agent_driven,omitempty"`
	Session     *shell.InteractiveSession `json:"-"`
}

// Notification is published when a session is done so that other attached
// clients can dismiss a terminal they may be showing.
type Notification struct {
	ID string `json:"id"`
}

// Service manages display requests for interactive terminal sessions.
type Service interface {
	pubsub.Subscriber[Request]

	// Show publishes a request for the UI to display the session.
	Show(ctx context.Context, req Request) (Request, error)

	// SetAvailable marks whether an interactive UI is attached. Without
	// one, Show fails fast with [ErrUnavailable] instead of spawning a
	// terminal nobody can see.
	SetAvailable(available bool)

	// Available reports whether an interactive UI is attached.
	Available() bool
}

type terminalService struct {
	broker *pubsub.Broker[Request]

	mu        sync.Mutex
	available bool
}

// NewService creates a new terminal service.
func NewService() *terminalService {
	return &terminalService{
		broker: pubsub.NewBroker[Request](),
	}
}

// Subscribe returns a channel for display requests.
func (s *terminalService) Subscribe(ctx context.Context) <-chan pubsub.Event[Request] {
	return s.broker.Subscribe(ctx)
}

// Show publishes a display request and returns the request with its ID set.
func (s *terminalService) Show(_ context.Context, req Request) (Request, error) {
	s.mu.Lock()
	available := s.available
	s.mu.Unlock()

	if !available {
		return Request{}, ErrUnavailable
	}
	if req.Session == nil {
		return Request{}, errors.New("terminal request carries no session")
	}
	if req.ID == "" {
		req.ID = uuid.New().String()
	}

	s.broker.Publish(pubsub.CreatedEvent, req)
	return req, nil
}

// SetAvailable implements [Service].
func (s *terminalService) SetAvailable(available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.available = available
}

// Available implements [Service].
func (s *terminalService) Available() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.available
}
