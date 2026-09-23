// Package terminal provides a service for running a command in an embedded
// interactive terminal. It mirrors the permission and question services:
// publish a request over pubsub, block on a channel, and resolve when the
// attached TUI reports the session finished.
//
// Only one interactive session can be pending at a time (the requesting
// tool blocks until the user is done), so no correlation IDs are needed in
// the domain model.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Errors returned by [Service.Run].
var (
	// ErrCancelled is returned when the session was cancelled before the
	// user finished interacting with it.
	ErrCancelled = errors.New("interactive terminal cancelled")
	// ErrBusy is returned when another interactive session is already
	// pending.
	ErrBusy = errors.New("an interactive terminal session is already active")
	// ErrUnavailable is returned when no interactive UI is attached, so
	// nobody could ever interact with the terminal.
	ErrUnavailable = errors.New("no interactive terminal is attached")
)

// Request asks the attached TUI to open an interactive terminal.
type Request struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	Command     string `json:"command"`
	WorkingDir  string `json:"working_dir"`
	Description string `json:"description,omitempty"`
}

// Validate checks that a Request is usable.
func (r Request) Validate() error {
	if strings.TrimSpace(r.Command) == "" && r.SessionID == "" {
		return fmt.Errorf("interactive terminal requires a command or a session")
	}
	if r.WorkingDir == "" {
		return fmt.Errorf("interactive terminal requires a working directory")
	}
	return nil
}

// Result is what the user's session produced.
type Result struct {
	// Output is the session transcript: every line that scrolled off,
	// followed by the final screen.
	Output string `json:"output"`
	// ExitCode is the process exit status.
	ExitCode int `json:"exit_code"`
	// WorkingDir is the directory the session ended in, when the shell
	// reported it.
	WorkingDir string `json:"working_dir,omitempty"`
	// Terminated reports that the user ended the session early rather than
	// the command exiting on its own.
	Terminated bool `json:"terminated,omitempty"`
}

// Notification is published when a session is resolved so that other
// attached clients can tear down a terminal they are showing.
type Notification struct {
	ID string `json:"id"`
}

// Service manages the lifecycle of interactive terminal sessions.
type Service interface {
	pubsub.Subscriber[Request]

	// SubscribeNotifications returns a channel for session resolution
	// notifications.
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[Notification]

	// Run publishes a request and blocks until the session finishes or
	// the context is cancelled.
	Run(ctx context.Context, req Request) (Result, error)

	// Complete resolves the pending session with the given result.
	Complete(result Result) bool

	// Cancel cancels the pending session. Returns false if no session is
	// pending.
	Cancel() bool

	// SetAvailable marks whether an interactive UI is attached to this
	// service. Without one, Run fails fast with [ErrUnavailable] instead
	// of blocking a tool call forever.
	SetAvailable(available bool)
}

type terminalService struct {
	broker             *pubsub.Broker[Request]
	notificationBroker *pubsub.Broker[Notification]

	mu        sync.Mutex
	pending   chan Result
	cancelled chan struct{}
	pendingID string

	available bool
}

// NewService creates a new terminal service.
func NewService() *terminalService {
	return &terminalService{
		broker:             pubsub.NewBroker[Request](),
		notificationBroker: pubsub.NewBroker[Notification](),
	}
}

// Subscribe returns a channel for terminal session requests.
func (s *terminalService) Subscribe(ctx context.Context) <-chan pubsub.Event[Request] {
	return s.broker.Subscribe(ctx)
}

// SubscribeNotifications returns a channel for session resolution
// notifications.
func (s *terminalService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[Notification] {
	return s.notificationBroker.Subscribe(ctx)
}

// Run publishes a request and blocks until the session finishes.
func (s *terminalService) Run(ctx context.Context, req Request) (Result, error) {
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if err := req.Validate(); err != nil {
		return Result{}, err
	}

	s.mu.Lock()
	if !s.available {
		s.mu.Unlock()
		return Result{}, ErrUnavailable
	}
	if s.pending != nil {
		s.mu.Unlock()
		return Result{}, ErrBusy
	}
	s.pending = make(chan Result, 1)
	s.cancelled = make(chan struct{})
	s.pendingID = req.ID
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.pending = nil
		s.cancelled = nil
		s.pendingID = ""
		s.mu.Unlock()
	}()

	s.broker.Publish(pubsub.CreatedEvent, req)

	select {
	case <-ctx.Done():
		// Tell any UI that opened a terminal for this request to close
		// it, or it would linger with nothing left to resolve it.
		s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{ID: req.ID})
		return Result{}, ctx.Err()
	case <-s.cancelled:
		return Result{}, ErrCancelled
	case result := <-s.pending:
		return result, nil
	}
}

// Complete resolves the pending session. Returns false if no session is
// pending (already completed or cancelled).
func (s *terminalService) Complete(result Result) bool {
	s.mu.Lock()
	sessionID := s.pendingID
	ch := s.pending
	s.mu.Unlock()

	if ch == nil {
		return false
	}
	ch <- result

	// Notify other clients so they close a terminal they may be showing.
	if sessionID != "" {
		s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{ID: sessionID})
	}
	return true
}

// Cancel cancels the pending session. Returns false if no session is
// pending.
func (s *terminalService) Cancel() bool {
	s.mu.Lock()
	sessionID := s.pendingID
	cancelCh := s.cancelled
	s.mu.Unlock()

	if cancelCh == nil {
		return false
	}
	close(cancelCh)

	if sessionID != "" {
		s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{ID: sessionID})
	}
	return true
}

// Pending reports whether a session is waiting for the user.
func (s *terminalService) Pending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending != nil
}

// SetAvailable implements [Service].
func (s *terminalService) SetAvailable(available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.available = available
}
