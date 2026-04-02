package toolruntime

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
)

type Status string

const (
	StatusPending           Status = "pending"
	StatusRunning           Status = "running"
	StatusBackgroundRunning Status = "background_running"
	StatusCompleted         Status = "completed"
	StatusFailed            Status = "failed"
	StatusCanceled          Status = "canceled"
)

type State struct {
	SessionID      string         `json:"session_id"`
	ToolCallID     string         `json:"tool_call_id"`
	ToolName       string         `json:"tool_name"`
	Status         Status         `json:"status"`
	SnapshotText   string         `json:"snapshot_text,omitempty"`
	ClientMetadata map[string]any `json:"client_metadata,omitempty"`
	StartedAt      int64          `json:"started_at"`
	UpdatedAt      int64          `json:"updated_at"`
	DurationMs     int64          `json:"duration_ms,omitempty"`
}

type Service interface {
	pubsub.Subscriber[State]
	Publish(State)
	Get(sessionID, toolCallID string) (State, bool)
	ListBySession(sessionID string) []State
	Delete(sessionID, toolCallID string)
	DeleteSession(sessionID string)
}

type service struct {
	*pubsub.Broker[State]
	mu     sync.RWMutex
	states map[string]map[string]State
}

func NewService() Service {
	return &service{
		Broker: pubsub.NewBroker[State](),
		states: make(map[string]map[string]State),
	}
}

func (s *service) Publish(state State) {
	now := time.Now().UnixMilli()
	if state.StartedAt == 0 {
		state.StartedAt = now
	}
	state.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.states[state.SessionID]; !ok {
		s.states[state.SessionID] = make(map[string]State)
	}
	prev, ok := s.states[state.SessionID][state.ToolCallID]
	if ok {
		if prev.StartedAt != 0 {
			state.StartedAt = prev.StartedAt
		}
		if state.ClientMetadata == nil {
			state.ClientMetadata = prev.ClientMetadata
		}
	}
	if state.StartedAt > 0 {
		duration := state.UpdatedAt - state.StartedAt
		if duration < 0 {
			duration = 0
		}
		state.DurationMs = duration
		if state.ClientMetadata == nil {
			state.ClientMetadata = map[string]any{}
		}
		if _, ok := state.ClientMetadata["duration_ms"]; !ok {
			state.ClientMetadata["duration_ms"] = state.DurationMs
		}
	}
	s.states[state.SessionID][state.ToolCallID] = state
	s.Broker.Publish(pubsub.UpdatedEvent, state)
}

func (s *service) Get(sessionID, toolCallID string) (State, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items, ok := s.states[sessionID]
	if !ok {
		return State{}, false
	}
	state, ok := items[toolCallID]
	return state, ok
}

func (s *service) ListBySession(sessionID string) []State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items, ok := s.states[sessionID]
	if !ok {
		return nil
	}
	out := make([]State, 0, len(items))
	for _, state := range items {
		out = append(out, state)
	}
	return out
}

func (s *service) Delete(sessionID, toolCallID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.states[sessionID]
	if !ok {
		return
	}
	state, ok := items[toolCallID]
	if !ok {
		return
	}
	delete(items, toolCallID)
	if len(items) == 0 {
		delete(s.states, sessionID)
	}
	s.Broker.Publish(pubsub.DeletedEvent, state)
}

func (s *service) DeleteSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.states[sessionID]
	if !ok {
		return
	}
	delete(s.states, sessionID)
	for _, state := range items {
		s.Broker.Publish(pubsub.DeletedEvent, state)
	}
}

type serviceContextKey struct{}

type sessionIDContextKey struct{}

type toolCallIDContextKey struct{}

func WithService(ctx context.Context, service Service) context.Context {
	return context.WithValue(ctx, serviceContextKey{}, service)
}

func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDContextKey{}, sessionID)
}

func WithToolCallID(ctx context.Context, toolCallID string) context.Context {
	return context.WithValue(ctx, toolCallIDContextKey{}, toolCallID)
}

func SessionIDFromContext(ctx context.Context) string {
	sessionID, _ := ctx.Value(sessionIDContextKey{}).(string)
	return sessionID
}

func ToolCallIDFromContext(ctx context.Context) string {
	toolCallID, _ := ctx.Value(toolCallIDContextKey{}).(string)
	return toolCallID
}

type delegationMailboxContextKey struct{}

func WithDelegationMailbox(ctx context.Context, mailboxID string) context.Context {
	return context.WithValue(ctx, delegationMailboxContextKey{}, mailboxID)
}

func DelegationMailboxFromContext(ctx context.Context) string {
	mailboxID, _ := ctx.Value(delegationMailboxContextKey{}).(string)
	return mailboxID
}

// BackgroundAgentLookup is a function type for looking up background agent status.
type BackgroundAgentLookup func(agentID string) (status, content, childSessionID string, found bool)

type backgroundAgentLookupKey struct{}

func WithBackgroundAgentLookup(ctx context.Context, lookup BackgroundAgentLookup) context.Context {
	return context.WithValue(ctx, backgroundAgentLookupKey{}, lookup)
}

func BackgroundAgentLookupFromContext(ctx context.Context) BackgroundAgentLookup {
	lookup, _ := ctx.Value(backgroundAgentLookupKey{}).(BackgroundAgentLookup)
	return lookup
}

func ServiceFromContext(ctx context.Context) Service {
	service, _ := ctx.Value(serviceContextKey{}).(Service)
	return service
}

func Report(ctx context.Context, state State) {
	service := ServiceFromContext(ctx)
	if service == nil {
		return
	}
	service.Publish(state)
}
