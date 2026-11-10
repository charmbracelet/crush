// Package terminal provides the bridge between an interactive terminal
// session (spawned by a tool) and the UI that displays it. It mirrors the
// permission and question services: a request is published over pubsub,
// and the attached TUI shows the session in its terminal dialog.
//
// Sessions live in [shell.GetInteractiveSessionManager] of the process that
// runs the TUI. A session is an in-process value that cannot cross the
// client/server boundary, so when the agent runs on a server, its terminal
// actions travel to the client instead: [Service.Exec] publishes a [Call],
// the client runs it against its own sessions, and answers with
// [Service.Respond].
package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/google/uuid"
)

// ErrUnavailable is returned when no interactive UI is attached, so nobody
// could ever look at the terminal.
var ErrUnavailable = errors.New("no interactive terminal is attached")

// ErrClientGone is returned by [Service.Exec] when the client running the
// call disconnects before answering.
var ErrClientGone = errors.New("the client hosting the terminal disconnected")

// OpCancel withdraws the in-flight call with the same ID. It expects no
// result.
const OpCancel = "cancel"

// Call is a terminal action forwarded to the client whose machine hosts
// the terminal.
type Call struct {
	ID string `json:"id"`
	// ClientID is the client that must run the call; no other client
	// receives it.
	ClientID   string          `json:"client_id"`
	SessionID  string          `json:"session_id"`
	ToolCallID string          `json:"tool_call_id"`
	Op         string          `json:"op"`
	Params     json.RawMessage `json:"params,omitempty"`
}

// Result is the outcome of a [Call].
type Result struct {
	Content  string `json:"content"`
	Metadata string `json:"metadata,omitempty"`
	IsError  bool   `json:"is_error,omitempty"`
}

// Router picks the client that runs terminal calls for a chat session.
// preferred is the client that served the session last, if any; sessions
// started there only exist on that machine. It returns [ErrUnavailable]
// when no client can take the call.
type Router func(sessionID, preferred string) (clientID string, err error)

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

	// SetRouter makes Exec forward calls to the clients router picks,
	// for a server whose TUIs run in other processes.
	SetRouter(router Router)

	// Exec forwards a call to a client and waits for its result. remote
	// is false when no router is set: the call must then run in this
	// process.
	Exec(ctx context.Context, call Call) (result Result, remote bool, err error)

	// SubscribeCalls returns a channel of calls forwarded to clients.
	SubscribeCalls(ctx context.Context) <-chan pubsub.Event[Call]

	// Respond resolves a pending call with the result from clientID. It
	// reports false when no such call is waiting on that client.
	Respond(callID, clientID string, result Result) bool

	// DropClient fails the calls waiting on a client that went away.
	DropClient(clientID string)
}

type pendingCall struct {
	clientID string
	done     chan Result
}

type terminalService struct {
	broker *pubsub.Broker[Request]
	calls  *pubsub.Broker[Call]

	mu        sync.Mutex
	available bool
	router    Router
	pending   map[string]pendingCall
	// pinned maps a chat session to the client that served it last.
	pinned map[string]string
}

// NewService creates a new terminal service.
func NewService() *terminalService {
	return &terminalService{
		broker:  pubsub.NewBroker[Request](),
		calls:   pubsub.NewBroker[Call](),
		pending: make(map[string]pendingCall),
		pinned:  make(map[string]string),
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

// SetRouter implements [Service].
func (s *terminalService) SetRouter(router Router) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.router = router
}

// SubscribeCalls implements [Service].
func (s *terminalService) SubscribeCalls(ctx context.Context) <-chan pubsub.Event[Call] {
	return s.calls.Subscribe(ctx)
}

// Exec implements [Service].
func (s *terminalService) Exec(ctx context.Context, call Call) (Result, bool, error) {
	s.mu.Lock()
	router, preferred := s.router, s.pinned[call.SessionID]
	s.mu.Unlock()
	if router == nil {
		return Result{}, false, nil
	}

	clientID, err := router(call.SessionID, preferred)
	if err != nil {
		return Result{}, true, err
	}

	call.ID = uuid.New().String()
	call.ClientID = clientID
	done := make(chan Result, 1)

	s.mu.Lock()
	s.pending[call.ID] = pendingCall{clientID: clientID, done: done}
	s.pinned[call.SessionID] = clientID
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, call.ID)
		s.mu.Unlock()
	}()

	s.calls.Publish(pubsub.CreatedEvent, call)

	select {
	case result, ok := <-done:
		if !ok {
			return Result{}, true, ErrClientGone
		}
		return result, true, nil
	case <-ctx.Done():
		// Stop the client's work too: a start may be waiting minutes for
		// the user.
		s.calls.Publish(pubsub.CreatedEvent, Call{
			ID:        call.ID,
			ClientID:  clientID,
			SessionID: call.SessionID,
			Op:        OpCancel,
		})
		return Result{}, true, ctx.Err()
	}
}

// Respond implements [Service].
func (s *terminalService) Respond(callID, clientID string, result Result) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending, ok := s.pending[callID]
	if !ok || pending.clientID != clientID {
		return false
	}
	delete(s.pending, callID)
	pending.done <- result
	return true
}

// DropClient implements [Service].
func (s *terminalService) DropClient(clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, pending := range s.pending {
		if pending.clientID == clientID {
			delete(s.pending, id)
			close(pending.done)
		}
	}
	for sessionID, pinned := range s.pinned {
		if pinned == clientID {
			delete(s.pinned, sessionID)
		}
	}
}
